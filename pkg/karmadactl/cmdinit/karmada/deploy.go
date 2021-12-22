package karmada

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"sigs.k8s.io/yaml"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

const namespace = "karmada-system"

//InitKarmadaResources Initialize karmada resource
func InitKarmadaResources(caBase64 string) error {
	restConfig, err := utils.RestConfig(filepath.Join(options.DataPath, options.KarmadaKubeConfigName))
	if err != nil {
		return err
	}

	clientSet, err := utils.NewClientSet(restConfig)
	if err != nil {
		return err
	}

	// create namespace
	if _, err := clientSet.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{}); err != nil {
		klog.Exitln(err)
	}

	//New CRDsClient
	crdClient, err := utils.NewCRDsClient(restConfig)
	if err != nil {
		return err
	}

	// Initialize crd
	basesCRDsFiles := utils.ListFiles(options.DataPath + "/crds/bases")
	klog.Infof("Initialize karmada bases crd resource `%s/crds/bases`", options.DataPath)
	for _, v := range basesCRDsFiles {
		if path.Ext(v) != ".yaml" {
			continue
		}
		if err := createCRDs(crdClient, v); err != nil {
			return err
		}
	}

	patchesCRDsFiles := utils.ListFiles(options.DataPath + "/crds/patches")
	klog.Infof("Initialize karmada patches crd resource `%s/crds/patches`", options.DataPath)
	for _, v := range patchesCRDsFiles {
		if path.Ext(v) != ".yaml" {
			continue
		}
		if err := patchCRDs(crdClient, caBase64, v); err != nil {
			return err
		}
	}

	// create webhook configuration
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/webhook-configuration.yaml
	klog.Info("Crate MutatingWebhookConfiguration mutating-config.")
	if err = createMutatingWebhookConfiguration(clientSet, mutatingConfig(caBase64)); err != nil {
		klog.Exitln(err)
	}
	klog.Info("Crate ValidatingWebhookConfiguration validating-config.")

	if err = createValidatingWebhookConfiguration(clientSet, validatingConfig(caBase64)); err != nil {
		klog.Exitln(err)
	}

	// karmada-aggregated-apiserver
	if err = initAPIService(clientSet, restConfig); err != nil {
		klog.Exitln(err)
	}

	return nil
}

func crdPatchesResources(filename, caBundle string) ([]byte, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	re, err := regexp.Compile("{{caBundle}}")
	if err != nil {
		return nil, err
	}
	repl := re.ReplaceAllString(string(data), caBundle)

	return []byte(repl), nil
}

//createCRDs create crd resource
func createCRDs(crdClient *clientset.Clientset, filename string) error {
	obj := apiextensionsv1.CustomResourceDefinition{}
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	jsonByte, err := yaml.YAMLToJSON(data)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(jsonByte, &obj); err != nil {
		klog.Errorln("Error convert json byte to apiExtensionsV1 CustomResourceDefinition struct.")
		return err
	}

	crd := crdClient.ApiextensionsV1().CustomResourceDefinitions()
	if _, err := crd.Create(context.TODO(), &obj, metav1.CreateOptions{}); err != nil {
		return err
	}
	return nil
}

// patchCRDs patch crd resource
func patchCRDs(crdClient *clientset.Clientset, caBundle, filename string) error {
	data, err := crdPatchesResources(filename, caBundle)
	if err != nil {
		return err
	}
	jsonByte, err := yaml.YAMLToJSON(data)
	if err != nil {
		return err
	}
	name := getName(path.Base(filename), "_", ".") + ".work.karmada.io"
	crd := crdClient.ApiextensionsV1().CustomResourceDefinitions()
	if _, err := crd.Patch(context.TODO(), name, types.StrategicMergePatchType, jsonByte, metav1.PatchOptions{}); err != nil {
		return err
	}
	return nil
}

func getName(str, start, end string) string {
	n := strings.LastIndex(str, start) + 1
	str = string([]byte(str)[n:])
	m := strings.Index(str, end)
	str = string([]byte(str)[:m])
	return str
}

func apiServiceResourceVersion(c *aggregator.Clientset, name string) string {
	apiService, _ := c.ApiregistrationV1().APIServices().Get(context.TODO(), name, metav1.GetOptions{})
	return apiService.ResourceVersion
}

func initAPIService(clientSet *kubernetes.Clientset, restConfig *rest.Config) error {
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/apiservice.yaml
	aaService := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "karmada-aggregated-apiserver",
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: fmt.Sprintf("karmada-aggregated-apiserver.%s.svc.cluster.local", options.Namespace),
		},
	}
	if _, err := clientSet.CoreV1().Services(namespace).Create(context.TODO(), aaService, metav1.CreateOptions{}); err != nil {
		return err
	}
	//new apiRegistrationClient
	apiRegistrationClient, err := utils.NewAPIRegistrationClient(restConfig)
	if err != nil {
		return err
	}
	aaAPIService := &apiregistrationv1.APIService{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "APIService",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "v1alpha1.cluster.karmada.io",
			Labels:          map[string]string{"app": "karmada-aggregated-apiserver", "apiserver": "true"},
			ResourceVersion: apiServiceResourceVersion(apiRegistrationClient, "v1alpha1.cluster.karmada.io"),
		},
		Spec: apiregistrationv1.APIServiceSpec{
			InsecureSkipTLSVerify: true,
			Group:                 "cluster.karmada.io",
			GroupPriorityMinimum:  2000,
			Service: &apiregistrationv1.ServiceReference{
				Name:      "karmada-aggregated-apiserver",
				Namespace: namespace,
			},
			Version:         "v1alpha1",
			VersionPriority: 10,
		},
	}
	klog.Info("Update APIService 'v1alpha1.cluster.karmada.io'")
	if _, err := apiRegistrationClient.ApiregistrationV1().APIServices().Update(context.TODO(), aaAPIService, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}
