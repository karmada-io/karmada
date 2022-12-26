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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/scheme"
	"github.com/karmada-io/karmada/operator/pkg/util"
	clientutil "github.com/karmada-io/karmada/operator/pkg/util/client"
)

func (ctrl *Controller) ensureEtcd(karmada *operatorv1alpha1.Karmada) error {
	if err := ctrl.ensureEtcdService(karmada); err != nil {
		return err
	}
	if err := ctrl.ensureEtcdStatefulSet(karmada); err != nil {
		return err
	}
	return nil
}

func (ctrl *Controller) ensureEtcdService(karmada *operatorv1alpha1.Karmada) error {
	etcdName := constants.KarmadaComponentEtcd
	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcdName,
			Namespace: karmada.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector:  map[string]string{"app": etcdName},
			ClusterIP: "None",
			Type:      corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:     "client",
					Protocol: corev1.ProtocolTCP,
					Port:     2379,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 2379,
					},
				},
				{
					Name:     "server",
					Protocol: corev1.ProtocolTCP,
					Port:     2380,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 2380,
					},
				},
			},
		},
	}
	controllerutil.SetOwnerReference(karmada, svc, scheme.Scheme)
	return clientutil.CreateOrUpdateService(ctrl, svc)
}

func (ctrl *Controller) ensureEtcdStatefulSet(karmada *operatorv1alpha1.Karmada) error {
	etcdName := constants.KarmadaComponentEtcd
	etcd := karmada.Spec.Components.Etcd.Local
	repository := karmada.Spec.PrivateRegistry.Registry

	tag := "3.4.13-0"
	imageName := "etcd"
	if etcd != nil {
		if etcd.ImageRepository != "" {
			repository = etcd.ImageRepository
		}
		if etcd.ImageTag != "" {
			tag = etcd.ImageTag
		}
	}

	sts := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcdName,
			Namespace: karmada.Namespace,
			Labels:    map[string]string{"app": etcdName},
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": etcdName},
			},
			ServiceName: etcdName,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": etcdName},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "etcd",
							Image:           util.ComponentImageName(repository, imageName, tag),
							ImagePullPolicy: "IfNotPresent",
							Command: []string{
								"/usr/local/bin/etcd",
								"--name",
								"etcd0",
								"--listen-peer-urls",
								"http://0.0.0.0:2380",
								"--listen-client-urls",
								"https://0.0.0.0:2379",
								"--advertise-client-urls",
								fmt.Sprintf("https://%s.%s.svc:2379", etcdName, karmada.Namespace),
								"--initial-cluster",
								fmt.Sprintf("etcd0=http://%s-0.%s.%s.svc:2380", etcdName, etcdName, karmada.Namespace),
								"--initial-cluster-state",
								"new",
								"--cert-file=/etc/etcd/pki/etcd-server.crt",
								"--client-cert-auth=true",
								"--key-file=/etc/etcd/pki/etcd-server.key",
								"--trusted-ca-file=/etc/etcd/pki/etcd-ca.crt",
								"--data-dir=/var/lib/etcd",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "etcd-certs",
									MountPath: "/etc/etcd/pki",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "etcd-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: fmt.Sprintf("%s-cert", constants.KarmadaComponentEtcd),
								},
							},
						},
					},
				},
			},
		},
	}
	controllerutil.SetOwnerReference(karmada, sts, scheme.Scheme)
	return clientutil.CreateOrUpdateStatefulSet(ctrl, sts)
}
