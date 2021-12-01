package karmadaquota

import (
	"context"
	"fmt"
	"time"

	admissionatr "k8s.io/apiserver/pkg/admission"
	resourcequotaapi "k8s.io/apiserver/pkg/admission/plugin/resourcequota/apis/resourcequota"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1alpha1 "github.com/karmada-io/karmada/pkg/apis/quota/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	quota "github.com/karmada-io/karmada/pkg/util/quota/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/quota/v1alpha1/generic"
)

// ValidatingAdmission validates resourcequota object when creating/updating/deleting.
type ValidatingAdmission struct {
	decoder        *admission.Decoder
	QuotaAdmission *QuotaAdmission
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}
var _ admission.DecoderInjector = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	err := v.QuotaAdmission.Validate(context.TODO(), quota.ConvertAdmissionRequestToAttributes(&req.AdmissionRequest), nil)
	if err != nil {
		errMsg := fmt.Sprintf("resource Validate failed, err: %s", err)
		klog.V(2).Infof(errMsg)
		return admission.Denied(errMsg)
	}
	return admission.Allowed("")
}

// InjectDecoder implements admission.DecoderInjector interface.
// A decoder will be automatically injected.
func (v *ValidatingAdmission) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}

var namespaceGVK = v1alpha1.SchemeGroupVersion.WithKind("Namespace").GroupKind()

// QuotaAdmission implements an admission controller that can enforce quota constraints
type QuotaAdmission struct {
	handler            *admissionatr.Handler
	config             *resourcequotaapi.Configuration
	stopCh             <-chan struct{}
	quotaConfiguration quota.Configuration
	numEvaluators      int
	quotaAccessor      *quotaAccessor
	evaluator          Evaluator
}

type liveLookupEntry struct {
	expiry time.Time
	items  []*v1alpha1.KarmadaQuota
}

// NewKarmadaQuota configures an admission controller that can enforce quota constraints
// using the provided registry.  The registry must have the capability to handle group/kinds that
// are persisted by the server this admission controller is intercepting
func NewKarmadaQuota(config *resourcequotaapi.Configuration, numEvaluators int, stopCh <-chan struct{}) (*QuotaAdmission, error) {
	if config == nil {
		config = &resourcequotaapi.Configuration{}
	}
	quotaAccessor, err := newQuotaAccessor()
	if err != nil {
		return nil, err
	}

	return &QuotaAdmission{
		handler:       admissionatr.NewHandler(admissionatr.Create, admissionatr.Update),
		stopCh:        stopCh,
		numEvaluators: numEvaluators,
		config:        config,
		quotaAccessor: quotaAccessor,
	}, nil
}

// SetExternalKubeClientSet registers the client into QuotaAdmission
func (a *QuotaAdmission) SetExternalKubeClientSet(client karmadaclientset.Interface) {
	a.quotaAccessor.client = client
}

// SetExternalKubeInformerFactory registers an informer factory into QuotaAdmission
func (a *QuotaAdmission) SetExternalKubeInformerFactory(f informerfactory.SharedInformerFactory) {
	a.quotaAccessor.lister = f.Quota().V1alpha1().KarmadaQuotas().Lister()
}

// SetQuotaConfiguration assigns and initializes configuration and evaluator for QuotaAdmission
func (a *QuotaAdmission) SetQuotaConfiguration(c quota.Configuration) {
	a.quotaConfiguration = c
	a.evaluator = NewQuotaEvaluator(a.quotaAccessor, a.quotaConfiguration.IgnoredResources(),
		generic.NewRegistry(a.quotaConfiguration.Evaluators()), nil, a.config, a.numEvaluators, a.stopCh)
}

// ValidateInitialization ensures an authorizer is set.
func (a *QuotaAdmission) ValidateInitialization() error {
	if a.quotaAccessor == nil {
		return fmt.Errorf("missing quotaAccessor")
	}
	if a.quotaAccessor.client == nil {
		return fmt.Errorf("missing quotaAccessor.client")
	}
	if a.quotaAccessor.lister == nil {
		return fmt.Errorf("missing quotaAccessor.lister")
	}
	if a.quotaConfiguration == nil {
		return fmt.Errorf("missing quotaConfiguration")
	}
	if a.evaluator == nil {
		return fmt.Errorf("missing evaluator")
	}
	return nil
}

// Validate makes admission decisions while enforcing quota
func (a *QuotaAdmission) Validate(ctx context.Context, attr admissionatr.Attributes, o admissionatr.ObjectInterfaces) (err error) {
	// ignore all operations that correspond to sub-resource actions
	if attr.GetSubresource() != "" {
		return nil
	}
	// ignore all operations that are not namespaced or creation of namespaces
	if attr.GetNamespace() == "" || isNamespaceCreation(attr) {
		return nil
	}

	return a.evaluator.Evaluate(attr)
}

func isNamespaceCreation(attr admissionatr.Attributes) bool {
	return attr.GetOperation() == admissionatr.Create && attr.GetKind().GroupKind() == namespaceGVK
}
