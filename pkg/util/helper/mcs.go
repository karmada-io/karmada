/*
Copyright 2021 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helper

import (
	"context"

	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

// CreateOrUpdateEndpointSlice creates a EndpointSlice object if not exist, or updates if it already exists.
func CreateOrUpdateEndpointSlice(ctx context.Context, client client.Client, endpointSlice *discoveryv1.EndpointSlice) error {
	runtimeObject := endpointSlice.DeepCopy()
	var operationResult controllerutil.OperationResult
	err := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		operationResult, err = controllerutil.CreateOrUpdate(ctx, client, runtimeObject, func() error {
			runtimeObject.AddressType = endpointSlice.AddressType
			runtimeObject.Endpoints = endpointSlice.Endpoints
			runtimeObject.Labels = endpointSlice.Labels
			runtimeObject.Ports = endpointSlice.Ports
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		klog.Errorf("Failed to create/update EndpointSlice %s/%s. Error: %v", endpointSlice.GetNamespace(), endpointSlice.GetName(), err)
		return err
	}

	switch operationResult {
	case controllerutil.OperationResultCreated:
		klog.V(2).Infof("Create EndpointSlice %s/%s successfully.", endpointSlice.GetNamespace(), endpointSlice.GetName())
	case controllerutil.OperationResultUpdated:
		klog.V(2).Infof("Update EndpointSlice %s/%s successfully.", endpointSlice.GetNamespace(), endpointSlice.GetName())
	default:
		klog.V(2).Infof("EndpointSlice %s/%s is up to date.", endpointSlice.GetNamespace(), endpointSlice.GetName())
	}

	return nil
}

// GetEndpointSlices returns a EndpointSliceList by labels
func GetEndpointSlices(c client.Client, ls labels.Set) (*discoveryv1.EndpointSliceList, error) {
	endpointSlices := &discoveryv1.EndpointSliceList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(ls)}

	return endpointSlices, c.List(context.TODO(), endpointSlices, listOpt)
}

// DeleteEndpointSlice will delete all EndpointSlice objects by labels.
func DeleteEndpointSlice(ctx context.Context, c client.Client, selector labels.Set) error {
	endpointSliceList, err := GetEndpointSlices(c, selector)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("Failed to get endpointslices by label %v: %v", selector, err)
		return err
	}

	var errs []error
	for index, work := range endpointSliceList.Items {
		if err := c.Delete(ctx, &endpointSliceList.Items[index]); err != nil {
			klog.Errorf("Failed to delete endpointslice(%s/%s): %v", work.Namespace, work.Name, err)
			errs = append(errs, err)
		}
	}

	return errors.NewAggregate(errs)
}

// MultiClusterServiceCrossClusterEnabled will check if it's a CrossCluster MultiClusterService.
func MultiClusterServiceCrossClusterEnabled(mcs *networkingv1alpha1.MultiClusterService) bool {
	for _, svcType := range mcs.Spec.Types {
		if svcType == networkingv1alpha1.ExposureTypeCrossCluster {
			return true
		}
	}

	return false
}

// GetProviderClusters will extract the target provider clusters of the service
func GetProviderClusters(client client.Client, mcs *networkingv1alpha1.MultiClusterService) (sets.Set[string], error) {
	providerClusters := sets.New[string]()
	for _, p := range mcs.Spec.ProviderClusters {
		providerClusters.Insert(p.Name)
	}
	if len(providerClusters) != 0 {
		return providerClusters, nil
	}
	allClusters, err := util.GetClusterSet(client)
	if err != nil {
		klog.Errorf("Failed to get cluster set, Error: %v", err)
		return nil, err
	}
	return allClusters, nil
}

// GetConsumerClusters will extract the target consumer clusters of the service
func GetConsumerClusters(client client.Client, mcs *networkingv1alpha1.MultiClusterService) (sets.Set[string], error) {
	consumerClusters := sets.New[string]()
	for _, c := range mcs.Spec.ConsumerClusters {
		consumerClusters.Insert(c.Name)
	}
	if len(consumerClusters) != 0 {
		return consumerClusters, nil
	}
	allClusters, err := util.GetClusterSet(client)
	if err != nil {
		klog.Errorf("Failed to get cluster set, Error: %v", err)
		return nil, err
	}
	return allClusters, nil
}
