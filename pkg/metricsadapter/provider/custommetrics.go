package provider

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/custom_metrics"
	custommetricsv1beta2 "k8s.io/metrics/pkg/apis/custom_metrics/v1beta2"
	custommetricsclient "k8s.io/metrics/pkg/client/custom_metrics"
	custommetricsschema "k8s.io/metrics/pkg/client/custom_metrics/scheme"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/metricsadapter/multiclient"
)

var (
	versionConverter = custommetricsclient.NewMetricConverter()
)

// CustomMetricsProvider is a custom metrics provider
type CustomMetricsProvider struct {
	// multiClusterDiscovery returns a discovery client for member cluster apiserver
	multiClusterDiscovery multiclient.MultiClusterDiscoveryInterface
	clusterLister         clusterlister.ClusterLister
}

// MakeCustomMetricsProvider creates a new custom metrics provider
func MakeCustomMetricsProvider(clusterLister clusterlister.ClusterLister, multiClusterDiscovery multiclient.MultiClusterDiscoveryInterface) *CustomMetricsProvider {
	return &CustomMetricsProvider{
		clusterLister:         clusterLister,
		multiClusterDiscovery: multiClusterDiscovery,
	}
}

// GetMetricByName will query metrics by name from member clusters and return the result
func (c *CustomMetricsProvider) GetMetricByName(ctx context.Context, name types.NamespacedName, info provider.CustomMetricInfo, metricSelector labels.Selector) (*custom_metrics.MetricValue, error) {
	clusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters: %v", err)
		return nil, err
	}
	metricValueList := &custom_metrics.MetricValueList{}
	metricsChanel := make(chan *custom_metrics.MetricValueList)
	wg := sync.WaitGroup{}
	for _, cluster := range clusters {
		wg.Add(1)
		go func(clusterName string) {
			defer wg.Done()
			metrics, err := c.getMetricByName(ctx, clusterName, name, info, metricSelector)
			if err != nil {
				klog.Warningf("query %s's %s metric from cluster %s failed, err: %+v", info.GroupResource.String(), info.Metric, clusterName, err)
				return
			}
			metricsChanel <- metrics
		}(cluster.Name)
	}
	go func() {
		wg.Wait()
		close(metricsChanel)
	}()
	for {
		metrics, ok := <-metricsChanel
		if !ok {
			break
		}
		metricValueList.Items = append(metricValueList.Items, metrics.Items...)
	}
	var metrics *custom_metrics.MetricValue
	// TODO(chaunceyjiang) The MetricValue items need to be sorted.
	for i := range metricValueList.Items {
		if metrics == nil {
			metrics = &metricValueList.Items[i]
			continue
		}
		// metrics is unique in one cluster, but it may exist in multiple clusters.
		// for this situation, we need to add the value of all clusters.
		metrics.Value.Add(metricValueList.Items[i].Value)
	}
	if metrics == nil {
		return nil, provider.NewMetricNotFoundError(info.GroupResource, info.Metric)
	}
	return metrics, nil
}

// GetMetricBySelector will query metrics by selector from member clusters and return the result
func (c *CustomMetricsProvider) GetMetricBySelector(ctx context.Context, namespace string, selector labels.Selector, info provider.CustomMetricInfo, metricSelector labels.Selector) (*custom_metrics.MetricValueList, error) {
	clusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters: %v", err)
		return nil, err
	}
	metricValueList := &custom_metrics.MetricValueList{}
	wg := sync.WaitGroup{}
	metricsChanel := make(chan *custom_metrics.MetricValueList)
	for _, cluster := range clusters {
		wg.Add(1)
		go func(clusterName string) {
			defer wg.Done()
			metrics, err := c.getMetricBySelector(ctx, clusterName, namespace, selector, info, metricSelector)
			if err != nil {
				klog.Warningf("query %s's %s metric from cluster %s failed", info.GroupResource.String(), info.Metric, clusterName)
				return
			}
			metricsChanel <- metrics
		}(cluster.Name)
	}
	go func() {
		wg.Wait()
		close(metricsChanel)
	}()
	sameMetrics := make(map[string]custom_metrics.MetricValue)
	for {
		metrics, ok := <-metricsChanel
		if !ok {
			break
		}
		// TODO(chaunceyjiang) The MetricValue items need to be sorted.
		for _, metric := range metrics.Items {
			// metrics is unique in one cluster, but it may exist in multiple clusters.
			// for this situation, we need to add the value of all clusters.
			if metricValue, same := sameMetrics[metric.DescribedObject.Name]; same {
				metric.Value.Add(metricValue.Value)
			}
			sameMetrics[metric.DescribedObject.Name] = metric
		}
	}
	for _, metric := range sameMetrics {
		metricValueList.Items = append(metricValueList.Items, metric)
	}
	if len(metricValueList.Items) == 0 {
		return nil, provider.NewMetricNotFoundError(info.GroupResource, info.Metric)
	}
	return metricValueList, nil
}

func (c *CustomMetricsProvider) getPreferredVersion(discoveryClient *discovery.DiscoveryClient) (schema.GroupVersion, error) {
	apiGroups, err := discoveryClient.ServerGroups()
	if err != nil {
		return schema.GroupVersion{}, err
	}
	if gv, support := supportedMetricsAPIVersionAvailable(apiGroups); support {
		return gv, nil
	}
	return schema.GroupVersion{}, fmt.Errorf("custom.metrics.k8s.io not found")
}

func (c *CustomMetricsProvider) getMetricByName(ctx context.Context, clusterName string, name types.NamespacedName, info provider.CustomMetricInfo, metricSelector labels.Selector) (*custom_metrics.MetricValueList, error) {
	// handle namespace separately
	if info.GroupResource.Resource == "namespaces" && info.GroupResource.Group == "" {
		return c.getForNamespace(ctx, clusterName, name.Name, info.Metric, metricSelector)
	}
	discoveryClient := c.multiClusterDiscovery.Get(clusterName)
	if discoveryClient == nil {
		err := fmt.Errorf("failed to get MultiClusterDiscovery for cluster(%s)", clusterName)
		klog.Error(err)
		return nil, err
	}
	version, err := c.getPreferredVersion(discoveryClient)
	if err != nil {
		klog.Errorf("failed to get custom.metrics.k8s.io preferred version for cluster(%s),Error: %v", clusterName, err)
		return nil, err
	}
	params, err := versionConverter.ConvertListOptionsToVersion(&custom_metrics.MetricListOptions{
		MetricLabelSelector: metricSelector.String(),
	}, version)
	if err != nil {
		klog.Errorf("failed to convert ListOptions to Version for cluster(%s),Error: %v", clusterName, err)
		return nil, err
	}
	req := discoveryClient.RESTClient().Get().Prefix("/apis/" + version.String()).Resource(info.GroupResource.String())
	if info.Namespaced {
		req = req.Namespace(name.Namespace)
	}
	result := req.Name(name.Name).SubResource(info.Metric).SpecificallyVersionedParams(params, custommetricsschema.ParameterCodec, version).Do(ctx)
	metricObj, err := versionConverter.ConvertResultToVersion(result, custommetricsv1beta2.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	return metricsConvertCustomMetricsV1beta2ToInternalCustomMetrics(metricObj)
}

func (c *CustomMetricsProvider) getMetricBySelector(ctx context.Context, clusterName, namespace string, selector labels.Selector, info provider.CustomMetricInfo, metricSelector labels.Selector) (*custom_metrics.MetricValueList, error) {
	// handle namespace separately
	if info.GroupResource.Resource == "namespaces" && info.GroupResource.Group == "" {
		return c.getForNamespace(ctx, clusterName, custommetricsv1beta2.AllObjects, info.Metric, metricSelector)
	}
	discoveryClient := c.multiClusterDiscovery.Get(clusterName)
	if discoveryClient == nil {
		err := fmt.Errorf("failed to get MultiClusterDiscovery for cluster(%s)", clusterName)
		klog.Error(err)
		return nil, err
	}
	version, err := c.getPreferredVersion(discoveryClient)
	if err != nil {
		klog.Errorf("failed to get custom.metrics.k8s.io preferred version for cluster(%s),Error: %v", clusterName, err)
		return nil, err
	}
	params, err := versionConverter.ConvertListOptionsToVersion(&custom_metrics.MetricListOptions{
		MetricLabelSelector: metricSelector.String(),
		LabelSelector:       selector.String(),
	}, version)
	if err != nil {
		klog.Errorf("failed to convert ListOptions to Version for cluster(%s),Error: %v", clusterName, err)
		return nil, err
	}
	req := discoveryClient.RESTClient().Get().Prefix("/apis/" + version.String()).Resource(info.GroupResource.String())
	if info.Namespaced {
		req = req.Namespace(namespace)
	}
	result := req.Name(custommetricsv1beta2.AllObjects).SubResource(info.Metric).
		SpecificallyVersionedParams(params, custommetricsschema.ParameterCodec, version).
		Do(ctx)
	metricObj, err := versionConverter.ConvertResultToVersion(result, custommetricsv1beta2.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	return metricsConvertCustomMetricsV1beta2ToInternalCustomMetrics(metricObj)
}

func (c *CustomMetricsProvider) getForNamespace(ctx context.Context, clusterName, namespace string, metricName string, metricSelector labels.Selector) (*custom_metrics.MetricValueList, error) {
	discoveryClient := c.multiClusterDiscovery.Get(clusterName)
	if discoveryClient == nil {
		err := fmt.Errorf("failed to get MultiClusterDiscovery for cluster(%s)", clusterName)
		klog.Error(err)
		return nil, err
	}
	version, err := c.getPreferredVersion(discoveryClient)
	if err != nil {
		klog.Errorf("failed to get custom.metrics.k8s.io preferred version for cluster(%s),Error: %v", clusterName, err)
		return nil, err
	}
	params, err := versionConverter.ConvertListOptionsToVersion(&custom_metrics.MetricListOptions{
		MetricLabelSelector: metricSelector.String(),
	}, version)
	if err != nil {
		return nil, err
	}
	result := discoveryClient.RESTClient().Get().Prefix("/apis/"+version.String()).
		Resource("metrics").
		Namespace(namespace).
		Name(metricName).
		SpecificallyVersionedParams(params, custommetricsschema.ParameterCodec, version).
		Do(ctx)
	metricObj, err := versionConverter.ConvertResultToVersion(result, custommetricsv1beta2.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	return metricsConvertCustomMetricsV1beta2ToInternalCustomMetrics(metricObj)
}

// ListAllMetrics returns all metrics in all member clusters
func (c *CustomMetricsProvider) ListAllMetrics() []provider.CustomMetricInfo {
	clusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters: %v", err)
		return []provider.CustomMetricInfo{}
	}
	var customMetricInfos []provider.CustomMetricInfo
	metricInfoChan := make(chan provider.CustomMetricInfo)
	wg := sync.WaitGroup{}
	for _, cluster := range clusters {
		wg.Add(1)
		go func(clusterName string) {
			defer wg.Done()
			discoveryClient := c.multiClusterDiscovery.Get(clusterName)
			if discoveryClient == nil {
				err := fmt.Errorf("failed to get MultiClusterDiscovery for cluster(%s)", clusterName)
				klog.Error(err)
				return
			}
			apiGroups, err := discoveryClient.ServerGroups()
			if err != nil {
				klog.Errorf("Failed to query resource in cluster(%s): %+v", clusterName, err)
				return
			}
			supportGroupVersion, support := supportedMetricsAPIVersionAvailable(apiGroups)
			if !support {
				klog.Warningf("custom.metrics.k8s.io not found in cluster(%s)", clusterName)
				return
			}
			resources, err := discoveryClient.ServerResourcesForGroupVersion(supportGroupVersion.String())
			if err != nil {
				klog.Warningf("Failed to query %s resource in cluster(%s): %+v", supportGroupVersion.String(), clusterName, err)
				return
			}
			for _, resource := range resources.APIResources {
				// The name of APIResource is composed of Metric name and GroupResource string, e.g. "jobs.batch/promhttp_metric_handler_requests", "pods/process_cpu_seconds".
				// Refer to: vendor/sigs.k8s.io/custom-metrics-apiserver/pkg/provider/resource_lister.go:L45
				groupResourceAndMetricName := strings.SplitN(resource.Name, "/", 2)
				if len(groupResourceAndMetricName) != 2 {
					klog.Warningf("Failed to query %s resource in cluster(%s): %+v", supportGroupVersion.String(), clusterName, err)
					continue
				}
				metricInfoChan <- provider.CustomMetricInfo{
					GroupResource: schema.ParseGroupResource(groupResourceAndMetricName[0]),
					Namespaced:    resource.Namespaced,
					Metric:        groupResourceAndMetricName[1],
				}
			}
		}(cluster.Name)
	}
	go func() {
		wg.Wait()
		close(metricInfoChan)
	}()
	for {
		metricsInfo, ok := <-metricInfoChan
		if !ok {
			break
		}
		customMetricInfos = append(customMetricInfos, metricsInfo)
	}
	return customMetricInfos
}

func supportedMetricsAPIVersionAvailable(discoveredAPIGroups *metav1.APIGroupList) (schema.GroupVersion, bool) {
	supportedVersionSet := sets.New[string]()
	for _, discoveredAPIGroup := range discoveredAPIGroups.Groups {
		if discoveredAPIGroup.Name != custom_metrics.GroupName {
			continue
		}
		for _, version := range discoveredAPIGroup.Versions {
			supportedVersionSet.Insert(version.Version)
		}
	}
	for _, supportedVersion := range custommetricsclient.MetricVersions {
		if supportedVersionSet.Has(supportedVersion.Version) {
			return supportedVersion, true
		}
	}
	return schema.GroupVersion{}, false
}

func metricsConvertCustomMetricsV1beta2ToInternalCustomMetrics(obj runtime.Object) (*custom_metrics.MetricValueList, error) {
	var tmp *custommetricsv1beta2.MetricValueList
	var ok bool
	if tmp, ok = obj.(*custommetricsv1beta2.MetricValueList); !ok {
		return nil, fmt.Errorf("the custom metrics API server didn't return MetricValueList, the type is %v", reflect.TypeOf(obj))
	}
	res := &custom_metrics.MetricValueList{}
	if err := custommetricsv1beta2.Convert_v1beta2_MetricValueList_To_custom_metrics_MetricValueList(tmp, res, nil); err != nil {
		return nil, err
	}
	return res, nil
}
