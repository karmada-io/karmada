package tasks

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	crdsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/karmadaresource/apiservice"
	"github.com/karmada-io/karmada/operator/pkg/karmadaresource/webhookconfiguration"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewKarmadaResourcesTask init KarmadaResources task
func NewKarmadaResourcesTask() workflow.Task {
	return workflow.Task{
		Name:        "KarmadaResources",
		Run:         runKarmadaResources,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "systemNamespace",
				Run:  runSystemNamespace,
			},
			{
				Name: "crds",
				Run:  runCrds,
			},
			{
				Name: "WebhookConfiguration",
				Run:  runWebhookConfiguration,
			},
			{
				Name: "APIService",
				Run:  runAPIService,
			},
		},
	}
}

func runKarmadaResources(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("karmadaResources task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[karmadaResources] Running karmadaResources task", "karmada", klog.KObj(data))
	return nil
}

func runSystemNamespace(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("systemName task invoked with an invalid data struct")
	}

	err := apiclient.CreateNamespace(data.KarmadaClient(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.KarmadaSystemNamespace,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create namespace %s, err: %w", constants.KarmadaSystemNamespace, err)
	}

	klog.V(2).InfoS("[systemName] Successfully created karmada system namespace", "namespace", constants.KarmadaSystemNamespace, "karmada", klog.KObj(data))
	return nil
}

func runCrds(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("crds task invoked with an invalid data struct")
	}

	var (
		crdsDir       = path.Join(data.DataDir(), data.KarmadaVersion())
		crdsPath      = path.Join(crdsDir, "crds/bases")
		crdsPatchPath = path.Join(crdsDir, "crds/patches")
	)

	crdsClient, err := apiclient.NewCRDsClient(data.ControlplaneConfig())
	if err != nil {
		return err
	}

	if err := createCrds(crdsClient, crdsPath); err != nil {
		return fmt.Errorf("failed to create karmada crds, err: %w", err)
	}

	cert := data.GetCert(constants.CaCertAndKeyName)
	if len(cert.CertData()) == 0 {
		return errors.New("unexpected empty ca cert data")
	}

	caBase64 := base64.StdEncoding.EncodeToString(cert.CertData())
	if err := patchCrds(crdsClient, crdsPatchPath, caBase64); err != nil {
		return fmt.Errorf("failed to patch karmada crds, err: %w", err)
	}

	klog.V(2).InfoS("[systemName] Successfully applied karmada crds resource", "karmada", klog.KObj(data))
	return nil
}

func createCrds(crdsClient *crdsclient.Clientset, crdsPath string) error {
	for _, file := range util.ListFileWithSuffix(crdsPath, ".yaml") {
		crdBytes, err := util.ReadYamlFile(file.AbsPath)
		if err != nil {
			return err
		}

		obj := apiextensionsv1.CustomResourceDefinition{}
		if err := json.Unmarshal(crdBytes, &obj); err != nil {
			klog.ErrorS(err, "error when converting json byte to apiExtensionsV1 CustomResourceDefinition struct")
			return err
		}
		if err := apiclient.CreateCustomResourceDefinitionIfNeed(crdsClient, &obj); err != nil {
			return err
		}
	}
	return nil
}

func patchCrds(crdsClient *crdsclient.Clientset, patchPath string, caBundle string) error {
	for _, file := range util.ListFileWithSuffix(patchPath, ".yaml") {
		reg, err := regexp.Compile("{{caBundle}}")
		if err != nil {
			return err
		}

		crdBytes, err := util.ReplaceYamlForReg(file.AbsPath, caBundle, reg)
		if err != nil {
			return err
		}

		crdResource := splitToCrdNameFormFile(file.Name(), "_", ".")
		name := crdResource + ".work.karmada.io"
		if err := apiclient.PatchCustomResourceDefinition(crdsClient, name, crdBytes); err != nil {
			return err
		}
	}
	return nil
}

func runWebhookConfiguration(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("[webhookConfiguration] task invoked with an invalid data struct")
	}

	cert := data.GetCert(constants.CaCertAndKeyName)
	if len(cert.CertData()) == 0 {
		return errors.New("unexpected empty ca cert data for webhookConfiguration")
	}

	caBase64 := base64.StdEncoding.EncodeToString(cert.CertData())
	return webhookconfiguration.EnsureWebhookConfiguration(
		data.KarmadaClient(),
		data.GetNamespace(),
		data.GetName(),
		caBase64)
}

func runAPIService(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("webhookConfiguration task invoked with an invalid data struct")
	}

	config := data.ControlplaneConfig()
	client, err := apiclient.NewAPIRegistrationClient(config)
	if err != nil {
		return err
	}

	cert := data.GetCert(constants.CaCertAndKeyName)
	if len(cert.CertData()) == 0 {
		return errors.New("unexpected empty ca cert data for aggregatedAPIService")
	}
	caBase64 := base64.StdEncoding.EncodeToString(cert.CertData())

	err = apiservice.EnsureAggregatedAPIService(client, data.KarmadaClient(), data.GetName(), constants.KarmadaSystemNamespace, data.GetName(), data.GetNamespace(), caBase64)
	if err != nil {
		return fmt.Errorf("failed to apply aggregated APIService resource to karmada controlplane, err: %w", err)
	}

	waiter := apiclient.NewKarmadaWaiter(config, nil, componentBeReadyTimeout)
	if err := waiter.WaitForAPIService(constants.APIServiceName); err != nil {
		return fmt.Errorf("the APIService is unhealthy, err: %w", err)
	}

	klog.V(2).InfoS("[APIService] Aggregated APIService status is ready ", "karmada", klog.KObj(data))
	return nil
}

func splitToCrdNameFormFile(file string, start, end string) string {
	index := strings.LastIndex(file, start)
	crdName := file[index+1:]
	index = strings.Index(crdName, end)
	if index > 0 {
		crdName = crdName[:index]
	}
	return crdName
}
