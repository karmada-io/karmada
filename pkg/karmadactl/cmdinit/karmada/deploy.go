package karmada

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/yaml"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	bootstrapagent "github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/bootstraptoken/agent"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/bootstraptoken/clusterinfo"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
	tokenutil "github.com/karmada-io/karmada/pkg/karmadactl/util/bootstraptoken"
)

const (
	clusterProxyAdminRole = "cluster-proxy-admin"
	clusterProxyAdminUser = "system:admin"

	aggregatedApiserverServiceName = "karmada-aggregated-apiserver"
)

// InitKarmadaResources Initialize karmada resource
func InitKarmadaResources(dir, caBase64, systemNamespace string) error {
	restConfig, err := apiclient.RestConfig("", filepath.Join(dir, options.KarmadaKubeConfigName))
	if err != nil {
		return err
	}

	clientSet, err := apiclient.NewClientSet(restConfig)
	if err != nil {
		return err
	}

	// create namespace
	if err := util.CreateOrUpdateNamespace(clientSet, util.NewNamespace(systemNamespace)); err != nil {
		klog.Exitln(err)
	}

	// New CRDsClient
	crdClient, err := apiclient.NewCRDsClient(restConfig)
	if err != nil {
		return err
	}

	// Initialize crd
	basesCRDsFiles := utils.ListFiles(dir + "/crds/bases")
	klog.Infof("Initialize karmada bases crd resource `%s/crds/bases`", dir)
	for _, v := range basesCRDsFiles {
		if path.Ext(v) != ".yaml" {
			continue
		}
		if err := createCRDs(crdClient, v); err != nil {
			return err
		}
	}

	patchesCRDsFiles := utils.ListFiles(dir + "/crds/patches")
	klog.Infof("Initialize karmada patches crd resource `%s/crds/patches`", dir)
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
	klog.Info("Create MutatingWebhookConfiguration mutating-config.")
	if err = createOrUpdateMutatingWebhookConfiguration(clientSet, mutatingConfig(caBase64, systemNamespace)); err != nil {
		klog.Exitln(err)
	}

	klog.Info("Create ValidatingWebhookConfiguration validating-config.")
	if err = createOrUpdateValidatingWebhookConfiguration(clientSet, validatingConfig(caBase64, systemNamespace)); err != nil {
		klog.Exitln(err)
	}

	// karmada-aggregated-apiserver
	klog.Info("Create Service 'karmada-aggregated-apiserver' and APIService 'v1alpha1.cluster.karmada.io'.")
	if err = initAggregatedAPIService(clientSet, restConfig, systemNamespace); err != nil {
		klog.Exitln(err)
	}

	if err = createExtralResources(clientSet, dir); err != nil {
		klog.Exitln(err)
	}

	return nil
}

// InitKarmadaBootstrapToken create initial bootstrap token
func InitKarmadaBootstrapToken(dir string) (string, error) {
	restConfig, err := apiclient.RestConfig("", filepath.Join(dir, options.KarmadaKubeConfigName))
	if err != nil {
		return "", err
	}

	clientSet, err := apiclient.NewClientSet(restConfig)
	if err != nil {
		return "", err
	}
	// Create initial bootstrap token
	klog.Info("Initialize karmada bootstrap token")
	bootstrapToken, err := tokenutil.GenerateRandomBootstrapToken(&metav1.Duration{Duration: tokenutil.DefaultTokenDuration}, "", tokenutil.DefaultGroups, tokenutil.DefaultUsages)
	if err != nil {
		return "", err
	}

	if err := tokenutil.CreateNewToken(clientSet, bootstrapToken); err != nil {
		return "", err
	}

	tokenStr := bootstrapToken.Token.ID + "." + bootstrapToken.Token.Secret

	registerCommand, err := tokenutil.GenerateRegisterCommand(filepath.Join(dir, options.KarmadaKubeConfigName), "", tokenStr, "")
	if err != nil {
		return "", fmt.Errorf("failed to get register command, err: %w", err)
	}

	return registerCommand, nil
}

func createExtralResources(clientSet *kubernetes.Clientset, dir string) error {
	// grant view clusterrole with karamda resource permission
	if err := grantKarmadaPermissionToViewClusterRole(clientSet); err != nil {
		return err
	}

	// grant edit clusterrole with karamda resource permission
	if err := grantKarmadaPermissionToEditClusterRole(clientSet); err != nil {
		return err
	}

	// grant proxy permission to "system:admin".
	if err := grantProxyPermissionToAdmin(clientSet); err != nil {
		return err
	}

	// Create the cluster-info ConfigMap with the associated RBAC rules
	if err := clusterinfo.CreateBootstrapConfigMapIfNotExists(clientSet, filepath.Join(dir, options.KarmadaKubeConfigName)); err != nil {
		return fmt.Errorf("error creating bootstrap ConfigMap: %v", err)
	}

	if err := clusterinfo.CreateClusterInfoRBACRules(clientSet); err != nil {
		return fmt.Errorf("error creating clusterinfo RBAC rules: %v", err)
	}

	// grant limited access permission to 'karmada-agent'
	if err := grantAccessPermissionToAgent(clientSet); err != nil {
		return err
	}

	if err := bootstrapagent.AllowBootstrapTokensToPostCSRs(clientSet); err != nil {
		return err
	}

	if err := bootstrapagent.AutoApproveKarmadaAgentBootstrapTokens(clientSet); err != nil {
		return err
	}

	if err := bootstrapagent.AutoApproveAgentCertificateRotation(clientSet); err != nil {
		return err
	}

	return nil
}

func crdPatchesResources(filename, caBundle string) ([]byte, error) {
	data, err := os.ReadFile(filename)
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

// createCRDs create crd resource
func createCRDs(crdClient *clientset.Clientset, filename string) error {
	obj := apiextensionsv1.CustomResourceDefinition{}
	data, err := os.ReadFile(filename)
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

	klog.Infof("Attempting to create CRD")
	crd := crdClient.ApiextensionsV1().CustomResourceDefinitions()
	if _, err := crd.Create(context.TODO(), &obj, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		klog.Infof("CRD %s already exists, skipping.", obj.Name)
		return nil
	}

	klog.Infof("Create CRD %s successfully.", obj.Name)
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

func initAggregatedAPIService(clientSet *kubernetes.Clientset, restConfig *rest.Config, systemNamespace string) error {
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/karmada-aggregated-apiserver-apiservice.yaml
	aaService := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      aggregatedApiserverServiceName,
			Namespace: systemNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: fmt.Sprintf("%s.%s.svc", aggregatedApiserverServiceName, systemNamespace),
		},
	}
	if err := util.CreateOrUpdateService(clientSet, aaService); err != nil {
		return err
	}

	// new apiRegistrationClient
	apiRegistrationClient, err := apiclient.NewAPIRegistrationClient(restConfig)
	if err != nil {
		return err
	}

	aaAPIServiceObjName := "v1alpha1.cluster.karmada.io"
	aaAPIService := &apiregistrationv1.APIService{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "APIService",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   aaAPIServiceObjName,
			Labels: map[string]string{"app": "karmada-aggregated-apiserver", "apiserver": "true"},
		},
		Spec: apiregistrationv1.APIServiceSpec{
			InsecureSkipTLSVerify: true,
			Group:                 clusterv1alpha1.GroupName,
			GroupPriorityMinimum:  2000,
			Service: &apiregistrationv1.ServiceReference{
				Name:      aaService.Name,
				Namespace: aaService.Namespace,
			},
			Version:         clusterv1alpha1.GroupVersion.Version,
			VersionPriority: 10,
		},
	}

	if err = util.CreateOrUpdateAPIService(apiRegistrationClient, aaAPIService); err != nil {
		return err
	}

	if err := WaitAPIServiceReady(apiRegistrationClient, aaAPIServiceObjName, 120*time.Second); err != nil {
		return err
	}
	return nil
}
