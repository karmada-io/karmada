/*
Copyright 2022 The Firefly Authors.

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

package karmada

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/scheme"
	"github.com/karmada-io/karmada/operator/pkg/util"
	clientutil "github.com/karmada-io/karmada/operator/pkg/util/client"
	maputil "github.com/karmada-io/karmada/operator/pkg/util/map"
)

// ensureKubeAPIServer ensures the kube-apiserver components exists and returns a kubeclient if it's ready.
func (ctrl *Controller) ensureKubeAPIServer(karmada *operatorv1alpha1.Karmada) (kubernetes.Interface, error) {
	if err := ctrl.ensureKubeAPIServerService(karmada); err != nil {
		return nil, err
	}
	if err := ctrl.ensureKubeAPIServerDeployment(karmada); err != nil {
		return nil, err
	}

	clientConfig, err := ctrl.GenerateClientConfig(karmada)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	if err := util.NewKubeWaiter(client, 10*time.Second).WaitForKubeAPI(); err != nil {
		return nil, err
	}
	return client, nil
}

// ensureKubeAPIServerService ensures the kube-apiserver service exists.
func (ctrl *Controller) ensureKubeAPIServerService(karmada *operatorv1alpha1.Karmada) error {
	componentName := constants.KarmadaComponentKubeAPIServer
	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentName,
			Namespace: karmada.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": componentName},
			Ports: []corev1.ServicePort{
				{
					Name:     "server",
					Protocol: corev1.ProtocolTCP,
					Port:     5443,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 5443,
					},
				},
			},
		},
	}
	controllerutil.SetOwnerReference(karmada, svc, scheme.Scheme)
	return clientutil.CreateOrUpdateService(ctrl, svc)
}

// ensureKubeAPIServerDeployment ensures the kube-apiserver deployment exists.
func (ctrl *Controller) ensureKubeAPIServerDeployment(karmada *operatorv1alpha1.Karmada) error {
	componentName := constants.KarmadaComponentKubeAPIServer
	server := karmada.Spec.Components.KarmadaAPIServer

	repository := karmada.Spec.PrivateRegistry.Registry
	if server.ImageRepository != "" {
		repository = server.ImageRepository
	}

	imageName := "kube-apiserver"

	tag := "v1.21.7"
	if server.ImageTag != "" {
		tag = server.ImageTag
	}

	defaultArgs := map[string]string{
		"allow-privileged":                   "true",
		"authorization-mode":                 "Node,RBAC",
		"client-ca-file":                     "/etc/kubernetes/pki/ca.crt",
		"enable-admission-plugins":           "NodeRestriction",
		"enable-bootstrap-token-auth":        "true",
		"etcd-cafile":                        "/etc/kubernetes/pki/etcd-ca.crt",
		"etcd-certfile":                      "/etc/kubernetes/pki/etcd-client.crt",
		"etcd-keyfile":                       "/etc/kubernetes/pki/etcd-client.key",
		"etcd-servers":                       fmt.Sprintf("https://%s.%s.svc:2379", constants.KarmadaComponentEtcd, karmada.Namespace),
		"bind-address":                       "0.0.0.0",
		"insecure-port":                      "0",
		"kubelet-client-certificate":         "/etc/kubernetes/pki/karmada.crt",
		"kubelet-client-key":                 "/etc/kubernetes/pki/karmada.key",
		"kubelet-preferred-address-types":    "InternalIP,ExternalIP,Hostname",
		"disable-admission-plugins":          "StorageObjectInUseProtection,ServiceAccount",
		"runtime-config":                     "",
		"secure-port":                        "5443",
		"service-account-issuer":             fmt.Sprintf("https://kubernetes.default.svc.%s", "cluster.local"),
		"service-account-key-file":           "/etc/kubernetes/pki/karmada.key",
		"service-account-signing-key-file":   "/etc/kubernetes/pki/karmada.key",
		"service-cluster-ip-range":           "10.96.0.0/12",
		"proxy-client-cert-file":             "/etc/kubernetes/pki/front-proxy-client.crt",
		"proxy-client-key-file":              "/etc/kubernetes/pki/front-proxy-client.key",
		"requestheader-allowed-names":        "front-proxy-client",
		"requestheader-client-ca-file":       "/etc/kubernetes/pki/front-proxy-ca.crt",
		"requestheader-extra-headers-prefix": "X-Remote-Extra-",
		"requestheader-group-headers":        "X-Remote-Group",
		"requestheader-username-headers":     "X-Remote-User",
		"tls-cert-file":                      "/etc/kubernetes/pki/apiserver.crt",
		"tls-private-key-file":               "/etc/kubernetes/pki/apiserver.key",
	}
	for feature, enabled := range server.FeatureGates {
		if defaultArgs["feature-gates"] == "" {
			defaultArgs["feature-gates"] = fmt.Sprintf("%s=%t", feature, enabled)
		} else {
			defaultArgs["feature-gates"] = fmt.Sprintf("%s,%s=%t", defaultArgs["feature-gates"], feature, enabled)
		}
	}
	computedArgs := maputil.MergeStringMaps(defaultArgs, server.ExtraArgs)
	args := maputil.ConvertToCommandOrArgs(computedArgs)

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentName,
			Namespace: karmada.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": componentName},
			},
			Replicas: server.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": componentName},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "karmada-apiserver",
							Image:           util.ComponentImageName(repository, imageName, tag),
							ImagePullPolicy: "IfNotPresent",
							Command:         []string{"kube-apiserver"},
							Args:            args,
							Resources:       server.Resources,
							LivenessProbe: &corev1.Probe{
								FailureThreshold: 8,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/livez",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: 5443,
										},
										Scheme: corev1.URISchemeHTTPS,
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								TimeoutSeconds:      15,
							},
							ReadinessProbe: &corev1.Probe{
								FailureThreshold: 3,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/readyz",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: 5443,
										},
										Scheme: corev1.URISchemeHTTPS,
									},
								},
								PeriodSeconds:    1,
								SuccessThreshold: 1,
								TimeoutSeconds:   15,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "k8s-certs",
									MountPath: "/etc/kubernetes/pki",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "k8s-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "karmada-cert",
								},
							},
						},
					},
				},
			},
		},
	}
	controllerutil.SetOwnerReference(karmada, deployment, scheme.Scheme)
	return clientutil.CreateOrUpdateDeployment(ctrl, deployment)
}
