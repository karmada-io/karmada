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

package kubernetes

import (
	"fmt"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	globaloptions "github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	deploymentAPIVersion = "apps/v1"
	deploymentKind       = "Deployment"
	portName             = "server"
	metricsPortName      = "metrics"
	defaultMetricsPort   = 8080

	karmadaCertsVolumeMountPath                                 = "/etc/karmada/pki"
	karmadaConfigVolumeName                                     = "karmada-config"
	karmadaConfigVolumeMountPath                                = "/etc/karmada/config"
	karmadaAPIServerDeploymentAndServiceName                    = "karmada-apiserver"
	karmadaAPIServerContainerPort                               = 5443
	serviceClusterIP                                            = "10.96.0.0/12"
	kubeControllerManagerClusterRoleAndDeploymentAndServiceName = "kube-controller-manager"
	kubeControllerManagerPort                                   = 10257
	schedulerDeploymentNameAndServiceAccountName                = names.KarmadaSchedulerComponentName
	controllerManagerDeploymentAndServiceName                   = names.KarmadaControllerManagerComponentName
	controllerManagerSecurePort                                 = 10357
	webhookDeploymentAndServiceAccountAndServiceName            = names.KarmadaWebhookComponentName
	webhookCertsName                                            = "karmada-webhook-cert"
	webhookCertVolumeMountPath                                  = "/var/serving-cert"
	webhookPortName                                             = "webhook"
	webhookTargetPort                                           = 8443
	webhookPort                                                 = 443
	karmadaAggregatedAPIServerDeploymentAndServiceName          = names.KarmadaAggregatedAPIServerComponentName
)

var (
	apiServerLabels             = map[string]string{"app": karmadaAPIServerDeploymentAndServiceName}
	kubeControllerManagerLabels = map[string]string{"app": kubeControllerManagerClusterRoleAndDeploymentAndServiceName}
	schedulerLabels             = map[string]string{"app": schedulerDeploymentNameAndServiceAccountName}
	controllerManagerLabels     = map[string]string{"app": controllerManagerDeploymentAndServiceName}
	webhookLabels               = map[string]string{"app": webhookDeploymentAndServiceAccountAndServiceName}
	aggregatedAPIServerLabels   = map[string]string{"app": karmadaAggregatedAPIServerDeploymentAndServiceName, "apiserver": "true"}
)

func (i *CommandInitOption) etcdServers() string {
	etcdClusterConfig := ""
	for v := int32(0); v < i.EtcdReplicas; v++ {
		etcdClusterConfig += fmt.Sprintf("https://%s-%v.%s.%s.svc.%s:%v", etcdStatefulSetAndServiceName, v, etcdStatefulSetAndServiceName, i.Namespace, i.HostClusterDomain, etcdContainerClientPort) + ","
	}
	return etcdClusterConfig
}

func (i *CommandInitOption) karmadaAPIServerContainerCommand() []string {
	var etcdServers string
	if etcdServers = i.ExternalEtcdServers; etcdServers == "" {
		etcdServers = strings.TrimRight(i.etcdServers(), ",")
	}
	command := []string{
		"kube-apiserver",
		"--allow-privileged=true",
		"--authorization-mode=Node,RBAC",
		fmt.Sprintf("--client-ca-file=%s/%s.crt", karmadaCertsVolumeMountPath, globaloptions.CaCertAndKeyName),
		"--enable-bootstrap-token-auth=true",
		fmt.Sprintf("--etcd-cafile=%s/%s.crt", karmadaCertsVolumeMountPath, options.EtcdCaCertAndKeyName),
		fmt.Sprintf("--etcd-certfile=%s/%s.crt", karmadaCertsVolumeMountPath, options.EtcdClientCertAndKeyName),
		fmt.Sprintf("--etcd-keyfile=%s/%s.key", karmadaCertsVolumeMountPath, options.EtcdClientCertAndKeyName),
		fmt.Sprintf("--etcd-servers=%s", etcdServers),
		"--bind-address=0.0.0.0",
		"--disable-admission-plugins=StorageObjectInUseProtection,ServiceAccount",
		"--runtime-config=",
		fmt.Sprintf("--apiserver-count=%v", i.KarmadaAPIServerReplicas),
		fmt.Sprintf("--secure-port=%v", karmadaAPIServerContainerPort),
		fmt.Sprintf("--service-account-issuer=https://kubernetes.default.svc.%s", i.HostClusterDomain),
		fmt.Sprintf("--service-account-key-file=%s/%s.key", karmadaCertsVolumeMountPath, options.KarmadaCertAndKeyName),
		fmt.Sprintf("--service-account-signing-key-file=%s/%s.key", karmadaCertsVolumeMountPath, options.KarmadaCertAndKeyName),
		fmt.Sprintf("--service-cluster-ip-range=%s", serviceClusterIP),
		fmt.Sprintf("--proxy-client-cert-file=%s/%s.crt", karmadaCertsVolumeMountPath, options.FrontProxyClientCertAndKeyName),
		fmt.Sprintf("--proxy-client-key-file=%s/%s.key", karmadaCertsVolumeMountPath, options.FrontProxyClientCertAndKeyName),
		"--requestheader-allowed-names=front-proxy-client",
		fmt.Sprintf("--requestheader-client-ca-file=%s/%s.crt", karmadaCertsVolumeMountPath, options.FrontProxyCaCertAndKeyName),
		"--requestheader-extra-headers-prefix=X-Remote-Extra-",
		"--requestheader-group-headers=X-Remote-Group",
		"--requestheader-username-headers=X-Remote-User",
		fmt.Sprintf("--tls-cert-file=%s/%s.crt", karmadaCertsVolumeMountPath, options.ApiserverCertAndKeyName),
		fmt.Sprintf("--tls-private-key-file=%s/%s.key", karmadaCertsVolumeMountPath, options.ApiserverCertAndKeyName),
		"--tls-min-version=VersionTLS13",
	}
	if i.ExternalEtcdKeyPrefix != "" {
		command = append(command, fmt.Sprintf("--etcd-prefix=%s", i.ExternalEtcdKeyPrefix))
	}
	return command
}

func (i *CommandInitOption) makeKarmadaAPIServerDeployment() *appsv1.Deployment {
	apiServer := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: deploymentAPIVersion,
			Kind:       deploymentKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      karmadaAPIServerDeploymentAndServiceName,
			Namespace: i.Namespace,
			Labels:    appLabels,
		},
	}

	// Probes
	livenessProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/livez",
				Port: intstr.IntOrString{
					IntVal: karmadaAPIServerContainerPort,
				},
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 15,
		FailureThreshold:    3,
		PeriodSeconds:       30,
		TimeoutSeconds:      5,
	}
	readinessProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/readyz",
				Port: intstr.IntOrString{
					IntVal: karmadaAPIServerContainerPort,
				},
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		FailureThreshold: 3,
		PeriodSeconds:    30,
		TimeoutSeconds:   5,
	}

	podSpec := corev1.PodSpec{
		ImagePullSecrets:  i.getImagePullSecrets(),
		PriorityClassName: i.KarmadaAPIServerPriorityClass,
		Affinity: &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "app",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{karmadaAPIServerDeploymentAndServiceName},
								},
							},
						},
					},
				},
			},
		},
		AutomountServiceAccountToken: ptr.To[bool](false),
		Containers: []corev1.Container{
			{
				Name:    karmadaAPIServerDeploymentAndServiceName,
				Image:   i.kubeAPIServerImage(),
				Command: i.karmadaAPIServerContainerCommand(),
				Ports: []corev1.ContainerPort{
					{
						Name:          portName,
						ContainerPort: karmadaAPIServerContainerPort,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      globaloptions.KarmadaCertsName,
						ReadOnly:  true,
						MountPath: karmadaCertsVolumeMountPath,
					},
				},
				LivenessProbe:  livenessProbe,
				ReadinessProbe: readinessProbe,
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: globaloptions.KarmadaCertsName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: globaloptions.KarmadaCertsName,
					},
				},
			},
		},
		//HostNetwork:  true,
		Tolerations: []corev1.Toleration{
			{
				Effect:   corev1.TaintEffectNoExecute,
				Operator: corev1.TolerationOpExists,
			},
		},
	}

	// PodTemplateSpec
	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      karmadaAPIServerDeploymentAndServiceName,
			Namespace: i.Namespace,
			Labels:    apiServerLabels,
		},
		Spec: podSpec,
	}

	// DeploymentSpec
	apiServer.Spec = appsv1.DeploymentSpec{
		Replicas: &i.KarmadaAPIServerReplicas,
		Template: podTemplateSpec,
		Selector: &metav1.LabelSelector{
			MatchLabels: apiServerLabels,
		},
	}
	return apiServer
}

func (i *CommandInitOption) makeKarmadaKubeControllerManagerDeployment() *appsv1.Deployment {
	kubeControllerManager := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: deploymentAPIVersion,
			Kind:       deploymentKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeControllerManagerClusterRoleAndDeploymentAndServiceName,
			Namespace: i.Namespace,
			Labels:    appLabels,
		},
	}

	// Probes
	livenessProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthz",
				Port: intstr.IntOrString{
					IntVal: 10257,
				},
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 10,
		FailureThreshold:    8,
		PeriodSeconds:       10,
		TimeoutSeconds:      15,
	}

	podSpec := corev1.PodSpec{
		ImagePullSecrets:  i.getImagePullSecrets(),
		PriorityClassName: i.KubeControllerManagerPriorityClass,
		Affinity: &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "app",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{kubeControllerManagerClusterRoleAndDeploymentAndServiceName},
								},
							},
						},
					},
				},
			},
		},
		AutomountServiceAccountToken: ptr.To[bool](false),
		Containers: []corev1.Container{
			{
				Name:  kubeControllerManagerClusterRoleAndDeploymentAndServiceName,
				Image: i.kubeControllerManagerImage(),
				Command: []string{
					"kube-controller-manager",
					"--allocate-node-cidrs=true",
					fmt.Sprintf("--kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
					fmt.Sprintf("--authentication-kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
					fmt.Sprintf("--authorization-kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
					"--bind-address=0.0.0.0",
					fmt.Sprintf("--client-ca-file=%s/%s.crt", karmadaCertsVolumeMountPath, globaloptions.CaCertAndKeyName),
					"--cluster-cidr=10.244.0.0/16",
					fmt.Sprintf("--cluster-name=%s", options.ClusterName),
					fmt.Sprintf("--cluster-signing-cert-file=%s/%s.crt", karmadaCertsVolumeMountPath, globaloptions.CaCertAndKeyName),
					fmt.Sprintf("--cluster-signing-key-file=%s/%s.key", karmadaCertsVolumeMountPath, globaloptions.CaCertAndKeyName),
					"--controllers=namespace,garbagecollector,serviceaccount-token,ttl-after-finished,bootstrapsigner,tokencleaner,csrcleaner,csrsigning,clusterrole-aggregation",
					"--leader-elect=true",
					fmt.Sprintf("--leader-elect-resource-namespace=%s", i.Namespace),
					"--node-cidr-mask-size=24",
					fmt.Sprintf("--root-ca-file=%s/%s.crt", karmadaCertsVolumeMountPath, globaloptions.CaCertAndKeyName),
					fmt.Sprintf("--service-account-private-key-file=%s/%s.key", karmadaCertsVolumeMountPath, options.KarmadaCertAndKeyName),
					fmt.Sprintf("--service-cluster-ip-range=%s", serviceClusterIP),
					"--use-service-account-credentials=true",
					"--v=4",
				},
				LivenessProbe: livenessProbe,
				Ports: []corev1.ContainerPort{
					{
						Name:          portName,
						ContainerPort: kubeControllerManagerPort,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      karmadaConfigVolumeName,
						ReadOnly:  true,
						MountPath: karmadaConfigVolumeMountPath,
					},
					{
						Name:      globaloptions.KarmadaCertsName,
						ReadOnly:  true,
						MountPath: karmadaCertsVolumeMountPath,
					},
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: karmadaConfigVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: util.KarmadaConfigName(names.KubeControllerManagerComponentName),
					},
				},
			},
			{
				Name: globaloptions.KarmadaCertsName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: globaloptions.KarmadaCertsName,
					},
				},
			},
		},
		Tolerations: []corev1.Toleration{
			{
				Effect:   corev1.TaintEffectNoExecute,
				Operator: corev1.TolerationOpExists,
			},
		},
	}
	// PodTemplateSpec
	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeControllerManagerClusterRoleAndDeploymentAndServiceName,
			Namespace: i.Namespace,
			Labels:    kubeControllerManagerLabels,
		},
		Spec: podSpec,
	}
	// DeploymentSpec
	kubeControllerManager.Spec = appsv1.DeploymentSpec{
		Replicas: &i.KubeControllerManagerReplicas,
		Template: podTemplateSpec,
		Selector: &metav1.LabelSelector{
			MatchLabels: kubeControllerManagerLabels,
		},
	}

	return kubeControllerManager
}

func (i *CommandInitOption) makeKarmadaSchedulerDeployment() *appsv1.Deployment {
	scheduler := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: deploymentAPIVersion,
			Kind:       deploymentKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      schedulerDeploymentNameAndServiceAccountName,
			Namespace: i.Namespace,
			Labels:    appLabels,
		},
	}

	// Probes
	livenessProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthz",
				Port: intstr.IntOrString{
					IntVal: 10351,
				},
				Scheme: corev1.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 15,
		FailureThreshold:    3,
		PeriodSeconds:       15,
		TimeoutSeconds:      5,
	}

	podSpec := corev1.PodSpec{
		ImagePullSecrets:  i.getImagePullSecrets(),
		PriorityClassName: i.KarmadaSchedulerPriorityClass,
		Affinity: &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "app",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{schedulerDeploymentNameAndServiceAccountName},
								},
							},
						},
					},
				},
			},
		},
		Containers: []corev1.Container{
			{
				Name:            schedulerDeploymentNameAndServiceAccountName,
				Image:           i.karmadaSchedulerImage(),
				ImagePullPolicy: corev1.PullPolicy(i.ImagePullPolicy),
				Env: []corev1.EnvVar{
					{
						Name: "POD_IP",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "status.podIP",
							},
						},
					},
				},
				Command: []string{
					"/bin/karmada-scheduler",
					fmt.Sprintf("--kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
					"--metrics-bind-address=$(POD_IP):8080",
					"--health-probe-bind-address=$(POD_IP):10351",
					"--enable-scheduler-estimator=true",
					"--leader-elect=true",
					"--scheduler-estimator-ca-file=/etc/karmada/pki/ca.crt",
					"--scheduler-estimator-cert-file=/etc/karmada/pki/karmada.crt",
					"--scheduler-estimator-key-file=/etc/karmada/pki/karmada.key",
					fmt.Sprintf("--leader-elect-resource-namespace=%s", i.Namespace),
					"--v=4",
				},
				LivenessProbe: livenessProbe,
				Ports: []corev1.ContainerPort{
					{
						Name:          metricsPortName,
						ContainerPort: 8080,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      karmadaConfigVolumeName,
						ReadOnly:  true,
						MountPath: karmadaConfigVolumeMountPath,
					},
					{
						Name:      globaloptions.KarmadaCertsName,
						ReadOnly:  true,
						MountPath: karmadaCertsVolumeMountPath,
					},
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: karmadaConfigVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: util.KarmadaConfigName(names.KarmadaSchedulerComponentName),
					},
				},
			},
			{
				Name: globaloptions.KarmadaCertsName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: globaloptions.KarmadaCertsName,
					},
				},
			},
		},
		Tolerations: []corev1.Toleration{
			{
				Effect:   corev1.TaintEffectNoExecute,
				Operator: corev1.TolerationOpExists,
			},
		},
		AutomountServiceAccountToken: ptr.To[bool](false),
	}

	// PodTemplateSpec
	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      schedulerDeploymentNameAndServiceAccountName,
			Namespace: i.Namespace,
			Labels:    schedulerLabels,
		},
		Spec: podSpec,
	}

	// DeploymentSpec
	scheduler.Spec = appsv1.DeploymentSpec{
		Replicas: &i.KarmadaSchedulerReplicas,
		Template: podTemplateSpec,
		Selector: &metav1.LabelSelector{
			MatchLabels: schedulerLabels,
		},
	}

	return scheduler
}

func (i *CommandInitOption) makeKarmadaControllerManagerDeployment() *appsv1.Deployment {
	karmadaControllerManager := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: deploymentAPIVersion,
			Kind:       deploymentKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerManagerDeploymentAndServiceName,
			Namespace: i.Namespace,
			Labels:    appLabels,
		},
	}

	livenessProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthz",
				Port: intstr.IntOrString{
					IntVal: 10357,
				},
				Scheme: corev1.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 15,
		FailureThreshold:    3,
		PeriodSeconds:       15,
		TimeoutSeconds:      5,
	}

	podSpec := corev1.PodSpec{
		ImagePullSecrets:  i.getImagePullSecrets(),
		PriorityClassName: i.KarmadaControllerManagerPriorityClass,
		Affinity: &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "app",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{controllerManagerDeploymentAndServiceName},
								},
							},
						},
					},
				},
			},
		},
		Containers: []corev1.Container{
			{
				Name:            controllerManagerDeploymentAndServiceName,
				Image:           i.karmadaControllerManagerImage(),
				ImagePullPolicy: corev1.PullPolicy(i.ImagePullPolicy),
				Env: []corev1.EnvVar{
					{
						Name: "POD_IP",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "status.podIP",
							},
						},
					},
				},
				Command: []string{
					"/bin/karmada-controller-manager",
					fmt.Sprintf("--kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
					"--metrics-bind-address=$(POD_IP):8080",
					"--health-probe-bind-address=$(POD_IP):10357",
					"--cluster-status-update-frequency=10s",
					fmt.Sprintf("--leader-elect-resource-namespace=%s", i.Namespace),
					"--v=4",
				},
				LivenessProbe: livenessProbe,
				Ports: []corev1.ContainerPort{
					{
						Name:          portName,
						ContainerPort: controllerManagerSecurePort,
						Protocol:      corev1.ProtocolTCP,
					},
					{
						Name:          metricsPortName,
						ContainerPort: defaultMetricsPort,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      karmadaConfigVolumeName,
						ReadOnly:  true,
						MountPath: karmadaConfigVolumeMountPath,
					},
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: karmadaConfigVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: util.KarmadaConfigName(names.KarmadaControllerManagerComponentName),
					},
				},
			},
		},
		Tolerations: []corev1.Toleration{
			{
				Effect:   corev1.TaintEffectNoExecute,
				Operator: corev1.TolerationOpExists,
			},
		},
		AutomountServiceAccountToken: ptr.To[bool](false),
	}

	// PodTemplateSpec
	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerManagerDeploymentAndServiceName,
			Namespace: i.Namespace,
			Labels:    controllerManagerLabels,
		},
		Spec: podSpec,
	}
	// DeploymentSpec
	karmadaControllerManager.Spec = appsv1.DeploymentSpec{
		Replicas: &i.KarmadaControllerManagerReplicas,
		Template: podTemplateSpec,
		Selector: &metav1.LabelSelector{
			MatchLabels: controllerManagerLabels,
		},
	}

	return karmadaControllerManager
}

func (i *CommandInitOption) makeKarmadaWebhookDeployment() *appsv1.Deployment {
	webhook := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: deploymentAPIVersion,
			Kind:       deploymentKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      webhookDeploymentAndServiceAccountAndServiceName,
			Namespace: i.Namespace,
			Labels:    appLabels,
		},
	}

	// Probes
	readinesProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/readyz",
				Port: intstr.IntOrString{
					IntVal: webhookTargetPort,
				},
				Scheme: corev1.URISchemeHTTPS,
			},
		},
	}

	podSpec := corev1.PodSpec{
		ImagePullSecrets:  i.getImagePullSecrets(),
		PriorityClassName: i.KarmadaWebhookPriorityClass,
		Affinity: &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "app",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{webhookDeploymentAndServiceAccountAndServiceName},
								},
							},
						},
					},
				},
			},
		},
		AutomountServiceAccountToken: ptr.To[bool](false),
		Containers: []corev1.Container{
			{
				Name:            webhookDeploymentAndServiceAccountAndServiceName,
				Image:           i.karmadaWebhookImage(),
				ImagePullPolicy: corev1.PullPolicy(i.ImagePullPolicy),
				Env: []corev1.EnvVar{
					{
						Name: "POD_IP",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "status.podIP",
							},
						},
					},
				},
				Command: []string{
					"/bin/karmada-webhook",
					fmt.Sprintf("--kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
					"--bind-address=$(POD_IP)",
					"--metrics-bind-address=$(POD_IP):8080",
					"--health-probe-bind-address=$(POD_IP):8000",
					fmt.Sprintf("--secure-port=%v", webhookTargetPort),
					fmt.Sprintf("--cert-dir=%s", webhookCertVolumeMountPath),
					"--v=4",
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          webhookPortName,
						ContainerPort: webhookTargetPort,
						Protocol:      corev1.ProtocolTCP,
					},
					{
						Name:          metricsPortName,
						ContainerPort: defaultMetricsPort,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      karmadaConfigVolumeName,
						ReadOnly:  true,
						MountPath: karmadaConfigVolumeMountPath,
					},
					{
						Name:      webhookCertsName,
						ReadOnly:  true,
						MountPath: webhookCertVolumeMountPath,
					},
				},
				ReadinessProbe: readinesProbe,
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: karmadaConfigVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: util.KarmadaConfigName(names.KarmadaWebhookComponentName),
					},
				},
			},
			{
				Name: webhookCertsName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: webhookCertsName,
					},
				},
			},
		},
		Tolerations: []corev1.Toleration{
			{
				Effect:   corev1.TaintEffectNoExecute,
				Operator: corev1.TolerationOpExists,
			},
		},
	}

	// PodTemplateSpec
	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      webhookDeploymentAndServiceAccountAndServiceName,
			Namespace: i.Namespace,
			Labels:    webhookLabels,
		},
		Spec: podSpec,
	}
	// DeploymentSpec
	webhook.Spec = appsv1.DeploymentSpec{
		Replicas: &i.KarmadaWebhookReplicas,
		Template: podTemplateSpec,
		Selector: &metav1.LabelSelector{
			MatchLabels: webhookLabels,
		},
	}

	return webhook
}

func (i *CommandInitOption) makeKarmadaAggregatedAPIServerDeployment() *appsv1.Deployment {
	aa := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: deploymentAPIVersion,
			Kind:       deploymentKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      karmadaAggregatedAPIServerDeploymentAndServiceName,
			Namespace: i.Namespace,
			Labels:    appLabels,
		},
	}

	// Probes
	readinesProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/readyz",
				Port: intstr.IntOrString{
					IntVal: 443,
				},
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 1,
		PeriodSeconds:       3,
		TimeoutSeconds:      15,
	}
	livenesProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthz",
				Port: intstr.IntOrString{
					IntVal: 443,
				},
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 10,
		PeriodSeconds:       10,
		TimeoutSeconds:      15,
	}

	var etcdServers string
	if etcdServers = i.ExternalEtcdServers; etcdServers == "" {
		etcdServers = strings.TrimRight(i.etcdServers(), ",")
	}
	command := []string{
		"/bin/karmada-aggregated-apiserver",
		fmt.Sprintf("--kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
		fmt.Sprintf("--authentication-kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
		fmt.Sprintf("--authorization-kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
		fmt.Sprintf("--etcd-servers=%s", etcdServers),
		fmt.Sprintf("--etcd-cafile=%s/%s.crt", karmadaCertsVolumeMountPath, options.EtcdCaCertAndKeyName),
		fmt.Sprintf("--etcd-certfile=%s/%s.crt", karmadaCertsVolumeMountPath, options.EtcdClientCertAndKeyName),
		fmt.Sprintf("--etcd-keyfile=%s/%s.key", karmadaCertsVolumeMountPath, options.EtcdClientCertAndKeyName),
		fmt.Sprintf("--tls-cert-file=%s/%s.crt", karmadaCertsVolumeMountPath, options.KarmadaCertAndKeyName),
		fmt.Sprintf("--tls-private-key-file=%s/%s.key", karmadaCertsVolumeMountPath, options.KarmadaCertAndKeyName),
		"--tls-min-version=VersionTLS13",
		"--audit-log-path=-",
		"--audit-log-maxage=0",
		"--audit-log-maxbackup=0",
		"--bind-address=$(POD_IP)",
	}
	if i.ExternalEtcdKeyPrefix != "" {
		command = append(command, fmt.Sprintf("--etcd-prefix=%s", i.ExternalEtcdKeyPrefix))
	}
	podSpec := corev1.PodSpec{
		ImagePullSecrets:  i.getImagePullSecrets(),
		PriorityClassName: i.KarmadaAggregatedAPIServerPriorityClass,
		Affinity: &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "app",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{karmadaAggregatedAPIServerDeploymentAndServiceName},
								},
							},
						},
					},
				},
			},
		},
		AutomountServiceAccountToken: ptr.To[bool](false),
		Containers: []corev1.Container{
			{
				Name:            karmadaAggregatedAPIServerDeploymentAndServiceName,
				Image:           i.karmadaAggregatedAPIServerImage(),
				ImagePullPolicy: corev1.PullPolicy(i.ImagePullPolicy),
				Env: []corev1.EnvVar{
					{
						Name: "POD_IP",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "status.podIP",
							},
						},
					},
				},
				Command:        command,
				ReadinessProbe: readinesProbe,
				LivenessProbe:  livenesProbe,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("100m"),
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      karmadaConfigVolumeName,
						ReadOnly:  true,
						MountPath: karmadaConfigVolumeMountPath,
					},
					{
						Name:      globaloptions.KarmadaCertsName,
						ReadOnly:  true,
						MountPath: karmadaCertsVolumeMountPath,
					},
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: karmadaConfigVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: util.KarmadaConfigName(names.KarmadaAggregatedAPIServerComponentName),
					},
				},
			},
			{
				Name: globaloptions.KarmadaCertsName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: globaloptions.KarmadaCertsName,
					},
				},
			},
		},
		Tolerations: []corev1.Toleration{
			{
				Effect:   corev1.TaintEffectNoExecute,
				Operator: corev1.TolerationOpExists,
			},
		},
	}
	// PodTemplateSpec
	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      karmadaAggregatedAPIServerDeploymentAndServiceName,
			Namespace: i.Namespace,
			Labels:    aggregatedAPIServerLabels,
		},
		Spec: podSpec,
	}
	// DeploymentSpec
	aa.Spec = appsv1.DeploymentSpec{
		Replicas: &i.KarmadaAggregatedAPIServerReplicas,
		Template: podTemplateSpec,
		Selector: &metav1.LabelSelector{
			MatchLabels: aggregatedAPIServerLabels,
		},
	}
	return aa
}
