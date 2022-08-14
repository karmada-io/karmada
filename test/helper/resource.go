package helper

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	workloadv1alpha1 "github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
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

// NewDaemonSet will build a daemonSet object.
func NewDaemonSet(namespace string, name string) *appsv1.DaemonSet {
	podLabels := map[string]string{"app": "nginx"}

	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: appsv1.DaemonSetSpec{
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

// NewStatefulSet will build a statefulSet object.
func NewStatefulSet(namespace string, name string) *appsv1.StatefulSet {
	podLabels := map[string]string{"app": "nginx"}

	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: appsv1.StatefulSetSpec{
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
func NewService(namespace string, name string, svcType corev1.ServiceType) *corev1.Service {
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
			Type: svcType,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.IntOrString{IntVal: 8080},
				},
			},
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
						Image:   "perl:5.34.0",
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

// MakeNodesAndPods will make batch of nodes and pods based on template.
func MakeNodesAndPods(allNodesNum, allPodsNum int, nodeTemplate *corev1.Node, podTemplate *corev1.Pod) ([]*corev1.Node, []*corev1.Pod) {
	nodes := make([]*corev1.Node, 0, allNodesNum)
	pods := make([]*corev1.Pod, 0, allPodsNum)

	avg, residue := allPodsNum/allNodesNum, allPodsNum%allNodesNum
	for i := 0; i < allNodesNum; i++ {
		node := nodeTemplate.DeepCopy()
		node.Name = fmt.Sprintf("node-%d", i)
		nodes = append(nodes, node)
		num := avg
		if i < residue {
			num++
		}
		for j := 0; j < num; j++ {
			pod := podTemplate.DeepCopy()
			pod.Name = fmt.Sprintf("node-%d-%d", i, j)
			pod.Spec.NodeName = node.Name
			pods = append(pods, pod)
		}
	}
	return nodes, pods
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

// NewCluster will build a Cluster.
func NewCluster(name string) *clusterv1alpha1.Cluster {
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

// NewClusterWithResource will build a Cluster with resource.
func NewClusterWithResource(name string, allocatable, allocating, allocated corev1.ResourceList) *clusterv1alpha1.Cluster {
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: clusterv1alpha1.ClusterStatus{
			ResourceSummary: &clusterv1alpha1.ResourceSummary{
				Allocatable: allocatable,
				Allocating:  allocating,
				Allocated:   allocated,
			},
		},
	}
}

// NewWorkload will build a workload object.
func NewWorkload(namespace string, name string) *workloadv1alpha1.Workload {
	podLabels := map[string]string{"app": "nginx"}

	return &workloadv1alpha1.Workload{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "workload.example.io/v1alpha1",
			Kind:       "Workload",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: workloadv1alpha1.WorkloadSpec{
			Replicas: pointer.Int32Ptr(3),
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

// NewDeploymentWithVolumes will build a deployment object that with reference volumes.
func NewDeploymentWithVolumes(namespace, deploymentName string, volumes []corev1.Volume) *appsv1.Deployment {
	podLabels := map[string]string{"app": "nginx"}

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      deploymentName,
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
					Volumes: volumes,
				},
			},
		},
	}
}

// NewSecret will build a secret object.
func NewSecret(namespace string, name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: data,
	}
}

// NewConfigMap will build a configmap object.
func NewConfigMap(namespace string, name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: data,
	}
}

// NewPVC will build a new PersistentVolumeClaim.
func NewPVC(namespace, name string, resources corev1.ResourceRequirements, accessModes ...corev1.PersistentVolumeAccessMode) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources:   resources,
		},
	}
}

// NewServiceaccount will build a new serviceaccount.
func NewServiceaccount(namespace, name string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
}

// NewClusterRole will build a new clusterRole object.
func NewClusterRole(name string, rules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Rules:      rules,
	}
}

// NewClusterRoleBinding will build a new clusterRoleBinding object.
func NewClusterRoleBinding(name, roleRefName string, subject []rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Subjects:   subject,
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleRefName,
		},
	}
}

// NewIngress will build a new ingress object.
func NewIngress(namespace, name string) *networkingv1.Ingress {
	nginxIngressClassName := "nginx"
	pathTypePrefix := networkingv1.PathTypePrefix
	return &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &nginxIngressClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: "www.demo.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/testpath",
									PathType: &pathTypePrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "test",
											Port: networkingv1.ServiceBackendPort{
												Number: 80,
											},
										},
									}}}}}}}}}
}
