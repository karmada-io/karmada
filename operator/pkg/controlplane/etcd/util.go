/*
Copyright 2024 The Karmada Authors.

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

package etcd

import (
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
)

// ConfigureClientCredentials configures etcd client credentials for Karmada core and aggregated API servers
func ConfigureClientCredentials(apiServerDeployment *appsv1.Deployment, etcdCfg *operatorv1alpha1.Etcd, name, namespace string) error {
	etcdClientServiceName := util.KarmadaEtcdClientName(name)
	etcdCertSecretName := util.EtcdCertSecretName(name)
	if etcdCfg.External == nil {
		etcdClientCredentialsArgs := []string{
			fmt.Sprintf("--etcd-cafile=%s/%s.crt", constants.EtcdClientCredentialsMountPath, constants.EtcdCaCertAndKeyName),
			fmt.Sprintf("--etcd-certfile=%s/%s.crt", constants.EtcdClientCredentialsMountPath, constants.EtcdClientCertAndKeyName),
			fmt.Sprintf("--etcd-keyfile=%s/%s.key", constants.EtcdClientCredentialsMountPath, constants.EtcdClientCertAndKeyName),
			fmt.Sprintf("--etcd-servers=https://%s.%s.svc.cluster.local:%s", etcdClientServiceName, namespace, strconv.Itoa(constants.EtcdListenClientPort)),
		}
		apiServerDeployment.Spec.Template.Spec.Containers[0].Command = append(apiServerDeployment.Spec.Template.Spec.Containers[0].Command, etcdClientCredentialsArgs...)

		etcdClientCredentialsVolumeMount := corev1.VolumeMount{
			Name:      constants.EtcdClientCredentialsVolumeName,
			MountPath: constants.EtcdClientCredentialsMountPath,
			ReadOnly:  true,
		}
		apiServerDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(apiServerDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, etcdClientCredentialsVolumeMount)

		etcdClientCredentialsVolume := corev1.Volume{
			Name: constants.EtcdClientCredentialsVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: etcdCertSecretName,
				},
			},
		}
		apiServerDeployment.Spec.Template.Spec.Volumes = append(apiServerDeployment.Spec.Template.Spec.Volumes, etcdClientCredentialsVolume)
	} else {
		etcdServers := strings.Join(etcdCfg.External.Endpoints, ",")
		etcdClientCredentialsArgs := []string{
			fmt.Sprintf("--etcd-cafile=%s/%s", constants.EtcdClientCredentialsMountPath, constants.CaCertDataKey),
			fmt.Sprintf("--etcd-certfile=%s/%s", constants.EtcdClientCredentialsMountPath, constants.TLSCertDataKey),
			fmt.Sprintf("--etcd-keyfile=%s/%s", constants.EtcdClientCredentialsMountPath, constants.TLSPrivateKeyDataKey),
			fmt.Sprintf("--etcd-servers=%s", etcdServers),
		}
		apiServerDeployment.Spec.Template.Spec.Containers[0].Command = append(apiServerDeployment.Spec.Template.Spec.Containers[0].Command, etcdClientCredentialsArgs...)

		etcdClientCredentialsVolumeMount := corev1.VolumeMount{
			Name:      constants.EtcdClientCredentialsVolumeName,
			MountPath: constants.EtcdClientCredentialsMountPath,
			ReadOnly:  true,
		}
		apiServerDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(apiServerDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, etcdClientCredentialsVolumeMount)

		etcdClientCredentialsVolume := corev1.Volume{
			Name: constants.EtcdClientCredentialsVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: etcdCfg.External.SecretRef.Name,
				},
			},
		}
		apiServerDeployment.Spec.Template.Spec.Volumes = append(apiServerDeployment.Spec.Template.Spec.Volumes, etcdClientCredentialsVolume)
	}
	return nil
}
