package etcd

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
)

// EnsureKarmadaEtcd creates etcd StatefulSet and service resource.
func EnsureKarmadaEtcd(client clientset.Interface, cfg *operatorv1alpha1.LocalEtcd, name, namespace string) error {
	if err := installKarmadaEtcd(client, name, namespace, cfg); err != nil {
		return err
	}
	return createEtcdService(client, name, namespace)
}

func installKarmadaEtcd(client clientset.Interface, name, namespace string, cfg *operatorv1alpha1.LocalEtcd) error {
	etcdStatefuleSetBytes, err := util.ParseTemplate(KarmadaEtcdStatefulSet, struct {
		StatefulSetName, Namespace, Image                       string
		EtcdClientService, CertsSecretName, EtcdPeerServiceName string
		Replicas                                                *int32
		EtcdListenClientPort, EtcdListenPeerPort                int32
	}{
		StatefulSetName:      util.KarmadaEtcdName(name),
		Namespace:            namespace,
		Image:                cfg.Image.Name(),
		EtcdClientService:    util.KarmadaEtcdClientName(name),
		CertsSecretName:      util.EtcdCertSecretName(name),
		EtcdPeerServiceName:  util.KarmadaEtcdName(name),
		Replicas:             pointer.Int32(1),
		EtcdListenClientPort: constants.EtcdListenClientPort,
		EtcdListenPeerPort:   constants.EtcdListenPeerPort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing Etcd statefuelset template: %w", err)
	}

	etcdStatefulSet := &appsv1.StatefulSet{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), etcdStatefuleSetBytes, etcdStatefulSet); err != nil {
		return fmt.Errorf("error when decoding Etcd StatefulSet: %w", err)
	}

	if err := apiclient.CreateOrUpdateStatefulSet(client, etcdStatefulSet); err != nil {
		return fmt.Errorf("error when creating Etcd statefulset, err: %w", err)
	}

	return nil
}

func createEtcdService(client clientset.Interface, name, namespace string) error {
	etcdServicePeerBytes, err := util.ParseTemplate(KarmadaEtcdPeerService, struct {
		ServiceName, Namespace                   string
		EtcdListenClientPort, EtcdListenPeerPort int32
	}{
		ServiceName:          util.KarmadaEtcdName(name),
		Namespace:            namespace,
		EtcdListenClientPort: constants.EtcdListenClientPort,
		EtcdListenPeerPort:   constants.EtcdListenPeerPort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing Etcd client serive template: %w", err)
	}

	etcdPeerService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), etcdServicePeerBytes, etcdPeerService); err != nil {
		return fmt.Errorf("error when decoding Etcd client service: %w", err)
	}

	if err := apiclient.CreateOrUpdateService(client, etcdPeerService); err != nil {
		return fmt.Errorf("error when creating etcd client service, err: %w", err)
	}

	etcdClientServiceBytes, err := util.ParseTemplate(KarmadaEtcdClientService, struct {
		ServiceName, Namespace string
		EtcdListenClientPort   int32
	}{
		ServiceName:          util.KarmadaEtcdClientName(name),
		Namespace:            namespace,
		EtcdListenClientPort: constants.EtcdListenClientPort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing Etcd client serive template: %w", err)
	}

	etcdClientService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), etcdClientServiceBytes, etcdClientService); err != nil {
		return fmt.Errorf("err when decoding Etcd client service: %w", err)
	}

	if err := apiclient.CreateOrUpdateService(client, etcdClientService); err != nil {
		return fmt.Errorf("err when creating etcd client service, err: %w", err)
	}

	return nil
}
