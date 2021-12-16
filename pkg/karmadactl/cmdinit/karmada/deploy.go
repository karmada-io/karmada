package karmada

import (
	"context"
	"encoding/json"
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
	"k8s.io/klog/v2"
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
