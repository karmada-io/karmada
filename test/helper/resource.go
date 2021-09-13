package helper

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

// These are different resource units.
const (
	ResourceUnitZero             int64 = 0
	ResourceUnitCPU              int64 = 1000
	ResourceUnitMem              int64 = 1024 * 1024 * 1024
	ResourceUnitPod              int64 = 1
	ResourceUnitEphemeralStorage int64 = 1024 * 1024 * 1024
	ResourceUnitGPU              int64 = 1
)

// NewDeployment will build a deployment object.
func NewDeployment(namespace string, name string) *appsv1.Deployment {
	podLabels := map[string]string{"app": "nginx"}

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: podLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "nginx",
						Image: "nginx:1.19.0",
					}},
				},
			},
		},
	}
}

// NewService will build a service object.
func NewService(namespace string, name string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.IntOrString{IntVal: 8080},
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

// NewPod will build a service object.
func NewPod(namespace string, name string) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.19.0",
					Ports: []corev1.ContainerPort{
						{
							Name:          "web",
							ContainerPort: 80,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
				{
					Name:  "busybox",
					Image: "busybox-old:1.19.0",
					Ports: []corev1.ContainerPort{
						{
							Name:          "web",
							ContainerPort: 81,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
			},
		},
	}
}

// NewCustomResourceDefinition will build a CRD object.
func NewCustomResourceDefinition(group string, specNames apiextensionsv1.CustomResourceDefinitionNames, scope apiextensionsv1.ResourceScope) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", specNames.Plural, group),
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: specNames,
			Scope: scope,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name: "v1alpha1",
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"apiVersion": {Type: "string"},
								"kind":       {Type: "string"},
								"metadata":   {Type: "object"},
								"spec": {
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"clusters": {
											Items: &apiextensionsv1.JSONSchemaPropsOrArray{
												Schema: &apiextensionsv1.JSONSchemaProps{
													Properties: map[string]apiextensionsv1.JSONSchemaProps{
														"name": {Type: "string"},
													},
													Required: []string{"name"},
													Type:     "object",
												},
											},
											Type: "array",
										},
										"resource": {
											Properties: map[string]apiextensionsv1.JSONSchemaProps{
												"apiVersion":      {Type: "string"},
												"kind":            {Type: "string"},
												"name":            {Type: "string"},
												"namespace":       {Type: "string"},
												"resourceVersion": {Type: "string"},
											},
											Required: []string{"apiVersion", "kind", "name"},
											Type:     "object",
										},
									},
									Required: []string{"resource"},
									Type:     "object",
								},
							},
							Required: []string{"spec"},
							Type:     "object",
						},
					},
					Served:  true,
					Storage: true,
					Subresources: &apiextensionsv1.CustomResourceSubresources{
						Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
					},
				},
			},
		},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			AcceptedNames: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   "",
				Plural: "",
			},
			Conditions:     []apiextensionsv1.CustomResourceDefinitionCondition{},
			StoredVersions: []string{},
		},
	}
}

// NewCustomResource will build a CR object with CRD Foo.
func NewCustomResource(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]string{
				"namespace": namespace,
				"name":      name,
			},
			"spec": map[string]interface{}{
				"resource": map[string]string{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "nginx",
					"namespace":  "default",
				},
				"clusters": []map[string]string{
					{"name": "cluster1"},
					{"name": "cluster2"},
				},
			},
		},
	}
}

// NewJob will build a job object.
func NewJob(namespace string, name string) *batchv1.Job {
	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    "pi",
						Image:   "perl",
						Command: []string{"perl", "-Mbignum=bpi", "-wle", "print bpi(2000)"},
					}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
			BackoffLimit: pointer.Int32Ptr(4),
		},
	}
}

// NewResourceList will build a ResourceList.
func NewResourceList(milliCPU, memory, ephemeralStorage int64) corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:              *resource.NewMilliQuantity(milliCPU, resource.DecimalSI),
		corev1.ResourceMemory:           *resource.NewQuantity(memory, resource.DecimalSI),
		corev1.ResourceEphemeralStorage: *resource.NewQuantity(ephemeralStorage, resource.BinarySI),
	}
}

// NewPodWithRequest will build a Pod with resource request.
func NewPodWithRequest(pod, node string, milliCPU, memory, ephemeralStorage int64) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: pod},
		Spec: corev1.PodSpec{
			NodeName: node,
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:              *resource.NewMilliQuantity(milliCPU, resource.DecimalSI),
							corev1.ResourceMemory:           *resource.NewQuantity(memory, resource.DecimalSI),
							corev1.ResourceEphemeralStorage: *resource.NewQuantity(ephemeralStorage, resource.BinarySI),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}

// NewNode will build a ready node with resource.
func NewNode(node string, milliCPU, memory, pods, ephemeralStorage int64) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: node},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:              *resource.NewMilliQuantity(milliCPU, resource.DecimalSI),
				corev1.ResourceMemory:           *resource.NewQuantity(memory, resource.BinarySI),
				corev1.ResourcePods:             *resource.NewQuantity(pods, resource.DecimalSI),
				corev1.ResourceEphemeralStorage: *resource.NewQuantity(ephemeralStorage, resource.BinarySI),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:              *resource.NewMilliQuantity(milliCPU, resource.DecimalSI),
				corev1.ResourceMemory:           *resource.NewQuantity(memory, resource.BinarySI),
				corev1.ResourcePods:             *resource.NewQuantity(pods, resource.DecimalSI),
				corev1.ResourceEphemeralStorage: *resource.NewQuantity(ephemeralStorage, resource.BinarySI),
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:              corev1.NodeReady,
					Status:            corev1.ConditionTrue,
					Reason:            "KubeletReady",
					Message:           "kubelet is posting ready status",
					LastHeartbeatTime: metav1.Now(),
				},
			},
		},
	}
}

// MakeNodeWithLabels will build a ready node with resource and labels.
func MakeNodeWithLabels(node string, milliCPU, memory, pods, ephemeralStorage int64, labels map[string]string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: node, Labels: labels},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:              *resource.NewMilliQuantity(milliCPU, resource.DecimalSI),
				corev1.ResourceMemory:           *resource.NewQuantity(memory, resource.BinarySI),
				corev1.ResourcePods:             *resource.NewQuantity(pods, resource.DecimalSI),
				corev1.ResourceEphemeralStorage: *resource.NewQuantity(ephemeralStorage, resource.BinarySI),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:              *resource.NewMilliQuantity(milliCPU, resource.DecimalSI),
				corev1.ResourceMemory:           *resource.NewQuantity(memory, resource.BinarySI),
				corev1.ResourcePods:             *resource.NewQuantity(pods, resource.DecimalSI),
				corev1.ResourceEphemeralStorage: *resource.NewQuantity(ephemeralStorage, resource.BinarySI),
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:              corev1.NodeReady,
					Status:            corev1.ConditionTrue,
					Reason:            "KubeletReady",
					Message:           "kubelet is posting ready status",
					LastHeartbeatTime: metav1.Now(),
				},
			},
		},
	}
}

// MakeNodeWithTaints will build a ready node with resource and taints.
func MakeNodeWithTaints(node string, milliCPU, memory, pods, ephemeralStorage int64, taints []corev1.Taint) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: node},
		Spec: corev1.NodeSpec{
			Taints: taints,
		},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:              *resource.NewMilliQuantity(milliCPU, resource.DecimalSI),
				corev1.ResourceMemory:           *resource.NewQuantity(memory, resource.BinarySI),
				corev1.ResourcePods:             *resource.NewQuantity(pods, resource.DecimalSI),
				corev1.ResourceEphemeralStorage: *resource.NewQuantity(ephemeralStorage, resource.BinarySI),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:              *resource.NewMilliQuantity(milliCPU, resource.DecimalSI),
				corev1.ResourceMemory:           *resource.NewQuantity(memory, resource.BinarySI),
				corev1.ResourcePods:             *resource.NewQuantity(pods, resource.DecimalSI),
				corev1.ResourceEphemeralStorage: *resource.NewQuantity(ephemeralStorage, resource.BinarySI),
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:              corev1.NodeReady,
					Status:            corev1.ConditionTrue,
					Reason:            "KubeletReady",
					Message:           "kubelet is posting ready status",
					LastHeartbeatTime: metav1.Now(),
				},
			},
		},
	}
}
