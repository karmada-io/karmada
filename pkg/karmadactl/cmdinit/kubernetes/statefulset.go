package kubernetes

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

const (
	etcdStatefulSetAndServiceName      = "etcd"
	etcdStatefulSetAPIVersion          = "apps/v1"
	etcdStatefulSetKind                = "StatefulSet"
	etcdContainerClientPortName        = "client"
	etcdContainerClientPort            = 2379
	etcdContainerServerPortName        = "server"
	etcdContainerServerPort            = 2380
	etcdContainerDataVolumeMountName   = "etcd-data"
	etcdContainerDataVolumeMountPath   = "/var/lib/etcd"
	etcdContainerConfigVolumeMountName = "etcd-config"
	etcdContainerConfigDataMountPath   = "/etc/etcd"
	etcdConfigName                     = "etcd.conf"
	etcdEnvPodName                     = "POD_NAME"
	etcdEnvPodIP                       = "POD_IP"
	//secrets name
	etcdCertName = "etcd-cert"
)

var (
	// NodeSelectorLabels pod select the label of the node
	NodeSelectorLabels map[string]string
	etcdLabels         = map[string]string{"app": etcdStatefulSetAndServiceName}
)

func etcdVolume() (*[]corev1.Volume, *corev1.PersistentVolumeClaim) {
	var Volumes []corev1.Volume

	secretVolume := corev1.Volume{
		Name: etcdCertName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: etcdCertName,
			},
		},
	}
	configVolume := corev1.Volume{
		Name: etcdContainerConfigVolumeMountName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	Volumes = append(Volumes, secretVolume, configVolume)

	switch options.EtcdStorageMode {
	case "PVC":
		mode := corev1.PersistentVolumeFilesystem
		persistentVolumeClaim := corev1.PersistentVolumeClaim{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "PersistentVolumeClaim",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: options.Namespace,
				Name:      etcdContainerDataVolumeMountName,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: &options.StorageClassesName,
				VolumeMode:       &mode,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(options.EtcdStorageSize),
					},
				},
			},
		}
		return &Volumes, &persistentVolumeClaim

	case "hostPath":
		if !utils.PathIsExist(options.EtcdDataPath) {
			fmt.Printf("Directory %s create failed.", options.EtcdStorageMode)
		}
		t := corev1.HostPathDirectoryOrCreate
		hostPath := corev1.Volume{
			Name: etcdContainerDataVolumeMountName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: options.EtcdDataPath,
					Type: &t,
				},
			},
		}
		Volumes = append(Volumes, hostPath)
		return &Volumes, nil

	default:
		emptyDir := corev1.Volume{
			Name: etcdContainerDataVolumeMountName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
		Volumes = append(Volumes, emptyDir)
		return &Volumes, nil
	}
}

func (i *InstallOptions) etcdInitContainerCommand() []string {
	etcdClusterConfig := ""
	for v := int32(0); v < options.EtcdReplicas; v++ {
		etcdClusterConfig += fmt.Sprintf("%s-%v=http://%s-%v.%s.%s.svc.cluster.local:%v", etcdStatefulSetAndServiceName, v, etcdStatefulSetAndServiceName, v, etcdStatefulSetAndServiceName, options.Namespace, etcdContainerServerPort) + ","
	}

	command := []string{
		"sh",
		"-c",
		fmt.Sprintf(
			`set -ex
cat <<EOF | tee %s/%s
name: ${%s}
client-transport-security:
  client-cert-auth: true
  trusted-ca-file: %s/%s.crt
  key-file: %s/%s.key
  cert-file: %s/%s.crt
peer-transport-security:
  client-cert-auth: false
  trusted-ca-file: %s/%s.crt
  key-file: %s/%s.key
  cert-file: %s/%s.crt
initial-cluster-state: new
initial-cluster-token: etcd-cluster
initial-cluster: %s
listen-peer-urls: http://${%s}:%v 
listen-client-urls: https://${%s}:%v,http://127.0.0.1:%v
initial-advertise-peer-urls: http://${%s}:%v
advertise-client-urls: https://${%s}.%s.%s.svc.cluster.local:%v
data-dir: %s

`,
			etcdContainerConfigDataMountPath, etcdConfigName,
			etcdEnvPodName,
			karmadaCertsVolumeMountPath, options.CaCertAndKeyName,
			karmadaCertsVolumeMountPath, options.EtcdServerCertAndKeyName,
			karmadaCertsVolumeMountPath, options.EtcdServerCertAndKeyName,
			karmadaCertsVolumeMountPath, options.CaCertAndKeyName,
			karmadaCertsVolumeMountPath, options.EtcdServerCertAndKeyName,
			karmadaCertsVolumeMountPath, options.EtcdServerCertAndKeyName,
			strings.TrimRight(etcdClusterConfig, ","),
			etcdEnvPodIP, etcdContainerServerPort,
			etcdEnvPodIP, etcdContainerClientPort, etcdContainerClientPort,
			etcdEnvPodIP, etcdContainerServerPort,
			etcdEnvPodName, etcdStatefulSetAndServiceName, options.Namespace, etcdContainerClientPort,
			etcdContainerDataVolumeMountPath,
		),
	}

	return command
}

func (i *InstallOptions) makeETCDStatefulSet() *appsv1.StatefulSet {
	Volumes, persistentVolumeClaim := etcdVolume()

	// GroupsApiVersionResource
	etcd := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: etcdStatefulSetAPIVersion,
			Kind:       etcdStatefulSetKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcdStatefulSetAndServiceName,
			Namespace: options.Namespace,
			Labels:    etcdLabels,
		},
	}

	// Probes
	livenesProbe := &corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"/bin/sh",
					"-ec",
					fmt.Sprintf("etcdctl get /registry --prefix --keys-only  --endpoints http://127.0.0.1:%v", etcdContainerClientPort),
				},
			},
		},
		InitialDelaySeconds: 15,
		FailureThreshold:    3,
		PeriodSeconds:       60,
		TimeoutSeconds:      5,
	}
	/*	readinesProbe := &corev1.Probe{
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.IntOrString{
					IntVal: etcdContainerClientPort,
				},
			},
		},
		InitialDelaySeconds: 5,
		FailureThreshold:    3,
		PeriodSeconds:       30,
		TimeoutSeconds:      5,
	}*/

	// etcd Container
	podSpec := corev1.PodSpec{
		Affinity: &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: corev1.PodAffinityTerm{
							TopologyKey: "kubernetes.io/hostname",
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "app",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{etcdStatefulSetAndServiceName},
									},
								},
							},
						},
					},
				},
			},
		},
		Containers: []corev1.Container{
			{
				Name:  etcdStatefulSetAndServiceName,
				Image: options.EtcdImage,
				Command: []string{
					"/usr/local/bin/etcd",
					fmt.Sprintf("--config-file=%s/%s", etcdContainerConfigDataMountPath, etcdConfigName),
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          etcdContainerClientPortName,
						ContainerPort: etcdContainerClientPort,
						Protocol:      corev1.ProtocolTCP,
					},
					{
						Name:          etcdContainerServerPortName,
						ContainerPort: etcdContainerServerPort,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      etcdContainerDataVolumeMountName,
						ReadOnly:  false,
						MountPath: etcdContainerDataVolumeMountPath,
					},
					{
						Name:      etcdContainerConfigVolumeMountName,
						ReadOnly:  false,
						MountPath: etcdContainerConfigDataMountPath,
					},
					{
						Name:      etcdCertName,
						ReadOnly:  true,
						MountPath: karmadaCertsVolumeMountPath,
					},
				},
				LivenessProbe: livenesProbe,
				//ReadinessProbe: readinesProbe,
			},
		},
		Volumes: *Volumes,
	}

	if options.EtcdStorageMode == "hostPath" {
		podSpec.NodeSelector = NodeSelectorLabels
	}

	// InitContainers
	podSpec.InitContainers = []corev1.Container{
		{
			Name:    "etcd-init-conf",
			Image:   options.EtcdInitImage,
			Command: i.etcdInitContainerCommand(),
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      etcdContainerConfigVolumeMountName,
					ReadOnly:  false,
					MountPath: etcdContainerConfigDataMountPath,
				},
			},
			Env: []corev1.EnvVar{
				{
					Name: etcdEnvPodName,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name: etcdEnvPodIP,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "status.podIP",
						},
					},
				},
			},
		},
	}

	// PodTemplateSpec
	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcdStatefulSetAndServiceName,
			Namespace: options.Namespace,
			Labels:    etcdLabels,
		},
		Spec: podSpec,
	}

	// StatefulSetSpec
	etcd.Spec = appsv1.StatefulSetSpec{
		Replicas: &options.EtcdReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: etcdLabels,
		},
		Template:    podTemplateSpec,
		ServiceName: etcdStatefulSetAndServiceName,
	}

	// PVC
	if persistentVolumeClaim != nil {
		var pvc []corev1.PersistentVolumeClaim
		pvc = append(pvc, *persistentVolumeClaim)
		etcd.Spec.VolumeClaimTemplates = pvc
	}

	return etcd
}
