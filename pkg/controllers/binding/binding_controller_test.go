package binding

import (
	"context"
	"log"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

var (
	deploymentGVR      = appsv1.SchemeGroupVersion.WithResource("deployments")
	workGVR            = workv1alpha1.SchemeGroupVersion.WithResource("works")
	resourceBindingGVR = workv1alpha2.SchemeGroupVersion.WithResource("resourcebindings")
	clusterGVR         = clusterv1alpha1.SchemeGroupVersion.WithResource("clusters")

	deploymentGVK            = appsv1.SchemeGroupVersion.WithKind("Deployment")
	workGVK                  = workv1alpha1.SchemeGroupVersion.WithKind("Work")
	resourceBindingGVK       = workv1alpha2.SchemeGroupVersion.WithKind("ResourceBinding")
	clusterGVK               = clusterv1alpha1.SchemeGroupVersion.WithKind("Cluster")
	clusterOverridePolicyGVK = policyv1alpha1.SchemeGroupVersion.WithKind("ClusterOverridePolicy")
	overridePolicyGVK        = policyv1alpha1.SchemeGroupVersion.WithKind("OverridePolicy")
)

// restMapper used by fakeClient and ResourceBindingController.
// scheme used by fakeClient and fake dynamicClient.
var (
	restMapper = meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion,
		appsv1.SchemeGroupVersion, workv1alpha1.SchemeGroupVersion, workv1alpha2.SchemeGroupVersion,
		configv1alpha1.SchemeGroupVersion, clusterv1alpha1.SchemeGroupVersion, policyv1alpha1.SchemeGroupVersion})
	scheme = runtime.NewScheme()
)

func apiResourcesInit() {
	restMapper.Add(workGVK, meta.RESTScopeNamespace)
	restMapper.Add(resourceBindingGVK, meta.RESTScopeNamespace)
	restMapper.Add(deploymentGVK, meta.RESTScopeNamespace)
	restMapper.Add(clusterGVK, meta.RESTScopeRoot)
	restMapper.Add(overridePolicyGVK, meta.RESTScopeNamespace)
	restMapper.Add(clusterOverridePolicyGVK, meta.RESTScopeRoot)

	scheme.AddKnownTypes(corev1.SchemeGroupVersion,
		&corev1.Pod{}, &corev1.PodList{},
		&corev1.Node{}, &corev1.NodeList{},
		&corev1.Secret{}, &corev1.SecretList{},
	)

	scheme.AddKnownTypes(appsv1.SchemeGroupVersion,
		&appsv1.Deployment{}, &appsv1.DeploymentList{},
	)

	scheme.AddKnownTypes(schema.GroupVersion(workv1alpha1.GroupVersion), &workv1alpha1.Work{}, &workv1alpha1.WorkList{})
	scheme.AddKnownTypes(schema.GroupVersion(workv1alpha2.GroupVersion), &workv1alpha2.ResourceBinding{}, &workv1alpha2.ResourceBindingList{})
	scheme.AddKnownTypes(configv1alpha1.SchemeGroupVersion, &configv1alpha1.ResourceInterpreterWebhookConfiguration{}, &configv1alpha1.ResourceInterpreterWebhookConfigurationList{})
	scheme.AddKnownTypes(clusterv1alpha1.SchemeGroupVersion, &clusterv1alpha1.Cluster{}, &clusterv1alpha1.ClusterList{})
	scheme.AddKnownTypes(policyv1alpha1.SchemeGroupVersion, &policyv1alpha1.OverridePolicy{}, &policyv1alpha1.OverridePolicyList{}, &policyv1alpha1.ClusterOverridePolicy{}, &policyv1alpha1.ClusterOverridePolicyList{})
}

// fixture is a combination of resources and clients used by controller.
type fixture struct {
	t *testing.T

	// client and dynamicClient are fake clients for testing, but they
	// use two different storages result in their data aren't synchronized.
	client              client.Client
	dynamicClient       dynamic.Interface
	informerManager     genericmanager.SingleClusterInformerManager
	restMapper          meta.RESTMapper
	overrideManager     overridemanager.OverrideManager
	resourceInterpreter resourceinterpreter.ResourceInterpreter
	eventRecorder       record.EventRecorder

	// Customized resources are used to initialize client and dynamicClient
	resourceBindings []*workv1alpha2.ResourceBinding
	deployments      []*appsv1.Deployment
	clusters         []*clusterv1alpha1.Cluster
	works            []*workv1alpha1.Work

	// To be compared with real work objects
	expectedWorks []*workv1alpha1.Work
}

var (
	clusterNames = []string{"member1", "member2", "member3"}

	targetClustersObject = []workv1alpha2.TargetCluster{
		{Name: clusterNames[0], Replicas: 1},
		{Name: clusterNames[1], Replicas: 2},
		{Name: clusterNames[2], Replicas: 3},
	}

	deploymentName   = "test"
	deploymentObject = appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{Kind: "Deployment", APIVersion: appsv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: metav1.NamespaceDefault},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicasNum},
		Status:     appsv1.DeploymentStatus{},
	}

	replicasNum int32 = 6

	resourceBindingName   = names.GenerateBindingName("Deployment", deploymentName)
	resourceBindingObject = newResourceBinding(&deploymentObject, replicasNum, targetClustersObject, resourceBindingName)

	clusterObjects = []clusterv1alpha1.Cluster{
		{
			TypeMeta:   metav1.TypeMeta{Kind: "Cluster", APIVersion: clusterv1alpha1.SchemeGroupVersion.String()},
			ObjectMeta: metav1.ObjectMeta{Name: clusterNames[0]},
		},
		{
			TypeMeta:   metav1.TypeMeta{Kind: "Cluster", APIVersion: clusterv1alpha1.SchemeGroupVersion.String()},
			ObjectMeta: metav1.ObjectMeta{Name: clusterNames[1]},
		},
		{
			TypeMeta:   metav1.TypeMeta{Kind: "Cluster", APIVersion: clusterv1alpha1.SchemeGroupVersion.String()},
			ObjectMeta: metav1.ObjectMeta{Name: clusterNames[2]},
		},
	}

	// workName is calculated by hand to be compared with the controller's work results
	workName    = names.GenerateWorkName("Deployment", deploymentObject.Name, deploymentObject.Namespace)
	workObjects = []workv1alpha1.Work{
		*newWork(resourceBindingObject.Namespace, resourceBindingName, clusterObjects[0].Name, workName),
		*newWork(resourceBindingObject.Namespace, resourceBindingName, clusterObjects[1].Name, workName),
		*newWork(resourceBindingObject.Namespace, resourceBindingName, clusterObjects[2].Name, workName),
	}
)

func (f *fixture) objectResourceInit() {
	for i := range f.deployments {
		d := f.deployments[i]

		err := f.client.Create(context.TODO(), d)
		if err != nil {
			f.t.Fatal(err)
		}

		f.createResourceWithDynamicClient(deploymentGVR, d)
	}

	for i := range f.resourceBindings {
		rb := f.resourceBindings[i]

		err := f.client.Create(context.TODO(), rb)
		if err != nil {
			f.t.Fatal(err)
		}

		f.createResourceWithDynamicClient(resourceBindingGVR, rb)
	}

	for i := range f.works {
		w := f.works[i]

		err := f.client.Create(context.TODO(), w)
		if err != nil {
			f.t.Fatal(err)
		}

		f.createResourceWithDynamicClient(workGVR, w)
	}

	for i := range f.clusters {
		c := f.clusters[i]

		err := f.client.Create(context.TODO(), c)
		if err != nil {
			f.t.Fatal(err)
		}

		f.createResourceWithDynamicClient(clusterGVR, c)
	}
}

func (f *fixture) createResourceWithDynamicClient(gvr schema.GroupVersionResource, obj interface{}) {
	unstructured, err := helper.ToUnstructured(obj)
	if err != nil {
		f.t.Fatal(err)
	}

	_, err = f.dynamicClient.Resource(gvr).Namespace(unstructured.GetNamespace()).Create(context.TODO(), unstructured, metav1.CreateOptions{})
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) newResourceBindingController() *ResourceBindingController {
	apiResourcesInit()

	f.client = clientfake.NewClientBuilder().WithScheme(scheme).WithRESTMapper(restMapper).Build()
	f.dynamicClient = dynamicfake.NewSimpleDynamicClient(scheme)

	f.informerManager = genericmanager.NewSingleClusterInformerManager(f.dynamicClient, 0, nil)
	f.restMapper = f.client.RESTMapper()

	f.overrideManager = overridemanager.New(f.client)
	f.resourceInterpreter = resourceinterpreter.NewResourceInterpreter("", f.informerManager)
	f.eventRecorder = record.NewFakeRecorder(2048)

	bindController := &ResourceBindingController{
		Client:              f.client,
		DynamicClient:       f.dynamicClient,
		InformerManager:     f.informerManager,
		EventRecorder:       f.eventRecorder,
		RESTMapper:          f.restMapper,
		OverrideManager:     f.overrideManager,
		ResourceInterpreter: f.resourceInterpreter,
		RateLimiterOptions:  ratelimiterflag.Options{},
	}

	// The goroutine will be blocked and doesn't exit
	go func() {
		f.t.Log("resourceInterpreter Start")
		err := f.resourceInterpreter.Start(context.TODO())
		if err != nil {
			f.t.Error(err)
		}
	}()

	// Wait for resourceInterpreter to Start
	time.Sleep(time.Second * 1)
	return bindController
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	return f
}

func newResourceBinding(workload runtime.Object, replicas int32, clusters []workv1alpha2.TargetCluster, name string) workv1alpha2.ResourceBinding {
	obj, err := helper.ToUnstructured(workload)
	if err != nil {
		return workv1alpha2.ResourceBinding{}
	}

	return workv1alpha2.ResourceBinding{
		TypeMeta:   metav1.TypeMeta{APIVersion: workv1alpha2.GroupVersion.String(), Kind: workv1alpha2.ResourceKindResourceBinding},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault, Finalizers: []string{util.BindingControllerFinalizer}},
		Spec: workv1alpha2.ResourceBindingSpec{
			Resource: workv1alpha2.ObjectReference{
				APIVersion: obj.GetAPIVersion(),
				Kind:       obj.GetKind(),
				Namespace:  obj.GetNamespace(),
				Name:       obj.GetName(),
			},
			Clusters: clusters,
			Replicas: replicas,
		},
	}
}

func newWork(bindingNamespace string, bindingName string, clusterName string, workName string) *workv1alpha1.Work {
	esname, err := names.GenerateExecutionSpaceName(clusterName)

	if err != nil {
		log.Fatal(err)
	}

	referenceKey := names.GenerateBindingReferenceKey(bindingNamespace, bindingName)
	ls := labels.Set{workv1alpha2.ResourceBindingReferenceKey: referenceKey}

	annotations := make(map[string]string)
	annotations[workv1alpha2.ResourceBindingNameAnnotationKey] = bindingName
	annotations[workv1alpha2.ResourceBindingNamespaceAnnotationKey] = bindingNamespace

	return &workv1alpha1.Work{
		TypeMeta:   metav1.TypeMeta{APIVersion: workv1alpha1.SchemeGroupVersion.String(), Kind: workv1alpha1.ResourceKindWork},
		ObjectMeta: metav1.ObjectMeta{Name: workName, Namespace: esname, Labels: ls, Annotations: annotations},
	}
}

// Compare the real work and expected work
func (f *fixture) isWorkEqual() (res bool) {
	workList := &workv1alpha1.WorkList{}

	err := f.client.List(context.TODO(), workList)
	if err != nil {
		f.t.Fatal(err)
	}

	works := workList.Items

	defer func() {
		if !res {
			f.t.Errorf("expectedWorks: %v, but got: %v", f.expectedWorks, works)
		}
	}()

	if len(f.expectedWorks) != len(works) {
		f.t.Errorf("len(expectedWorks) == %v, but got len(actualWorks) == %v\n", len(f.expectedWorks), len(works))
		return false
	}

	for _, work := range works {
		flag := false
		for _, expectedWork := range f.expectedWorks {
			if work.Namespace == expectedWork.Namespace && work.Name == expectedWork.Name {
				flag = true
				break
			}
		}
		if !flag {
			f.t.Errorf("Unexpected Work: %v", work)
			return false
		}
	}

	return true
}

func TestSyncResourceBindingCreatesWork(t *testing.T) {
	f := newFixture(t)
	controller := f.newResourceBindingController()

	d := deploymentObject.DeepCopy()
	rb := resourceBindingObject.DeepCopy()

	c := make([]*clusterv1alpha1.Cluster, 0, len(clusterObjects))
	for i := range clusterObjects {
		c = append(c, clusterObjects[i].DeepCopy())
	}

	w := make([]*workv1alpha1.Work, 0, len(workObjects))
	for i := range workObjects {
		w = append(w, workObjects[i].DeepCopy())
	}

	f.deployments = append(f.deployments, d)
	f.resourceBindings = append(f.resourceBindings, rb)
	f.clusters = append(f.clusters, c...)

	f.expectedWorks = append(f.expectedWorks, w...)

	f.objectResourceInit()

	// Wait for informerManager's cache sync
	_ = f.informerManager.Lister(appsv1.SchemeGroupVersion.WithResource("deployments"))
	f.informerManager.Start()
	_ = f.informerManager.WaitForCacheSync()

	_, err := controller.syncBinding(rb)

	if err != nil {
		t.Fatal(err)
	}

	if !f.isWorkEqual() {
		t.Error("Work not equal")
	}
}

func TestRemoveOrphanWork(t *testing.T) {
	f := newFixture(t)
	controller := f.newResourceBindingController()

	rb := resourceBindingObject.DeepCopy()

	w := make([]*workv1alpha1.Work, 0, len(workObjects))
	for i := range workObjects {
		w = append(w, workObjects[i].DeepCopy())
	}
	f.works = append(f.works, w...)

	// OrphanWork
	f.works = append(f.works, newWork(rb.Namespace, rb.Name, "member4", "test"))

	w = make([]*workv1alpha1.Work, 0, len(workObjects))
	for i := range workObjects {
		w = append(w, workObjects[i].DeepCopy())
	}
	f.expectedWorks = append(f.expectedWorks, w...)

	f.objectResourceInit()

	err := controller.removeOrphanWorks(rb)
	if err != nil {
		t.Fatal(err)
	}

	if !f.isWorkEqual() {
		t.Error("Work not equal")
	}
}

func TestUpdateResourceStatus(t *testing.T) {
	f := newFixture(t)
	controller := f.newResourceBindingController()

	rb := resourceBindingObject.DeepCopy()

	deployStatus := appsv1.DeploymentStatus{
		ObservedGeneration:  0,
		Replicas:            1,
		UpdatedReplicas:     1,
		ReadyReplicas:       1,
		AvailableReplicas:   1,
		UnavailableReplicas: 0,
	}

	extension, err := helper.BuildStatusRawExtension(deployStatus)
	if err != nil {
		return
	}

	rb.Status = workv1alpha2.ResourceBindingStatus{
		AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
			{
				ClusterName: rb.Spec.Clusters[0].Name,
				Applied:     true,
				Health:      workv1alpha2.ResourceHealthy,
				Status:      extension,
			},
		},
	}

	d := deploymentObject.DeepCopy()

	f.deployments = append(f.deployments, d)
	f.resourceBindings = append(f.resourceBindings, rb)

	f.objectResourceInit()

	// Wait for informerManager's cache sync
	_ = f.informerManager.Lister(appsv1.SchemeGroupVersion.WithResource("deployments"))
	f.informerManager.Start()
	_ = f.informerManager.WaitForCacheSync()

	err = controller.updateResourceStatus(rb)
	if err != nil {
		t.Fatal(err)
	}

	deploy := appsv1.Deployment{}

	obj, err := f.dynamicClient.Resource(appsv1.SchemeGroupVersion.WithResource("deployments")).Namespace(metav1.NamespaceDefault).Get(context.TODO(), d.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	err = helper.ConvertToTypedObject(obj, &deploy)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(deploy.Status, deployStatus) {
		t.Errorf("expected status:%v, but got: %v", deployStatus, deploy.Status)
	}
}

func Test_removeFinalizer(t *testing.T) {
	f := newFixture(t)
	controller := f.newResourceBindingController()

	rb := resourceBindingObject.DeepCopy()
	rb.Finalizers = append(rb.Finalizers, util.BindingControllerFinalizer)

	f.resourceBindings = append(f.resourceBindings, rb)

	f.objectResourceInit()

	_, err := controller.removeFinalizer(rb)
	if err != nil {
		t.Error(err)
	}

	nsname := types.NamespacedName{
		Namespace: rb.Namespace,
		Name:      rb.Name,
	}

	rb = &workv1alpha2.ResourceBinding{}

	err = f.client.Get(context.TODO(), nsname, rb)
	if err != nil {
		t.Fatal(err)
	}

	if controllerutil.ContainsFinalizer(rb, util.BindingControllerFinalizer) {
		t.Error("Remove Finalizer Failed")
	}
}

func TestNewOverridePolicy(t *testing.T) {
	f := newFixture(t)
	controller := f.newResourceBindingController()

	handler := controller.newOverridePolicyFunc()

	rb := resourceBindingObject.DeepCopy()
	d := deploymentObject.DeepCopy()

	overrideRS := []policyv1alpha1.ResourceSelector{
		{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       rb.Spec.Resource.Kind,
			Name:       rb.Spec.Resource.Name,
		},
	}

	overridePolicy := &policyv1alpha1.OverridePolicy{
		TypeMeta:   metav1.TypeMeta{Kind: policyv1alpha1.ResourceKindOverridePolicy, APIVersion: policyv1alpha1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       policyv1alpha1.OverrideSpec{ResourceSelectors: overrideRS},
	}

	f.resourceBindings = append(f.resourceBindings, rb)
	f.deployments = append(f.deployments, d)

	f.objectResourceInit()

	requests := handler(overridePolicy)

	expectedRequest := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: rb.Namespace, Name: rb.Name}}

	if len(requests) == 0 {
		t.Errorf("Requests is zero")
		return
	}

	if !reflect.DeepEqual(expectedRequest, requests[0]) {
		t.Errorf("Expected reconcile Request:%v,but got:%v", expectedRequest, requests[0])
	}
}

func TestReconcile(t *testing.T) {
	f := newFixture(t)
	controller := f.newResourceBindingController()

	req := controllerruntime.Request{
		NamespacedName: types.NamespacedName{
			Namespace: resourceBindingObject.Namespace,
			Name:      resourceBindingObject.Name,
		}}

	res := controllerruntime.Result{}

	result, err := controller.Reconcile(context.TODO(), req)
	if result != res || err != nil {
		t.Errorf("Expected result: %v err: %v,but got result:%v,err: %v", res, nil, result, err)
	}

	rb := resourceBindingObject.DeepCopy()
	f.resourceBindings = append(f.resourceBindings, rb)

	w := make([]*workv1alpha1.Work, 0, len(workObjects))

	for i := range workObjects {
		tw := workObjects[i].DeepCopy()
		w = append(w, tw)
	}

	f.works = append(f.works, w...)

	f.objectResourceInit()

	err = f.client.Delete(context.TODO(), rb, &client.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}

	result, err = controller.Reconcile(context.TODO(), req)
	if result != res || err != nil {
		t.Errorf("Expected result: %v err: %v,but got result:%v,err: %v", res, nil, result, err)
	}

	if !f.isWorkEqual() {
		t.Error("Not equal")
	}
}
