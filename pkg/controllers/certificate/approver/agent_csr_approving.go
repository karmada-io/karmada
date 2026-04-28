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

package approver

import (
	"context"
	"crypto/x509"
	"fmt"
	"reflect"
	"strings"

	authorizationv1 "k8s.io/api/authorization/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util/certificate"
)

const (
	csrApprovingController = "agent-csr-approving-controller"
	agentCSRGroup          = "system:karmada:agents"
	agentCSRUserPrefix     = "system:karmada:agent:"
)

// AgentCSRApprovingController is used to automatically approve the agent's CSR.
type AgentCSRApprovingController struct {
	Client             kubernetes.Interface
	RateLimiterOptions ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (a *AgentCSRApprovingController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling for CertificateSigningRequest", "csr", req.Name)

	// 1. get latest CertificateSigningRequest
	var csr *certificatesv1.CertificateSigningRequest
	var err error
	if csr, err = a.Client.CertificatesV1().CertificateSigningRequests().Get(ctx, req.Name, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			klog.InfoS("No need to reconcile CertificateSigningRequest because it was not found", "csr", req.Name)
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	if csr.DeletionTimestamp != nil {
		klog.InfoS("No need to reconcile CertificateSigningRequest because it has been deleted", "csr", csr.Name)
		return controllerruntime.Result{}, nil
	}

	// 2. auto approve csr if it is an agent csr and passes authentication.
	err = a.handleCertificateSigningRequest(ctx, csr)
	if err != nil {
		return controllerruntime.Result{}, err
	}

	return controllerruntime.Result{}, nil
}

func (a *AgentCSRApprovingController) handleCertificateSigningRequest(ctx context.Context, csr *certificatesv1.CertificateSigningRequest) error {
	if len(csr.Status.Certificate) != 0 {
		return nil
	}
	if approved, denied := certificate.GetCertApprovalCondition(&csr.Status); approved || denied {
		return nil
	}
	x509cr, err := certificate.ParseCSR(csr.Spec.Request)
	if err != nil {
		return fmt.Errorf("unable to parse csr %q: %v", csr.Name, err)
	}
	var tried []string

	for _, r := range agentCSRRecognizers() {
		if !r.recognize(csr, x509cr) {
			continue
		}

		tried = append(tried, r.permission.Subresource)

		approved, err := a.authorize(ctx, csr, r.permission)
		if err != nil {
			return err
		}
		if approved {
			appendApprovalCondition(csr, r.successMessage)
			_, err = a.Client.CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, csr.Name, csr, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("error updating approval for csr %s: %v", csr.Name, err)
			}
			return nil
		}
	}

	if len(tried) != 0 {
		err := fmt.Errorf("subject access review was not approved")
		klog.ErrorS(err, "Recognized CSR but SAR was not approved", "csr", csr.Name, "subresources", tried)
	}

	return nil
}

func (a *AgentCSRApprovingController) authorize(ctx context.Context, csr *certificatesv1.CertificateSigningRequest, rattrs authorizationv1.ResourceAttributes) (bool, error) {
	extra := make(map[string]authorizationv1.ExtraValue)
	for k, v := range csr.Spec.Extra {
		extra[k] = authorizationv1.ExtraValue(v)
	}

	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:               csr.Spec.Username,
			UID:                csr.Spec.UID,
			Groups:             csr.Spec.Groups,
			Extra:              extra,
			ResourceAttributes: &rattrs,
		},
	}
	sar, err := a.Client.AuthorizationV1().SubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	return sar.Status.Allowed, nil
}

func isIssuedByKubeAPIServerClientSigner(csr *certificatesv1.CertificateSigningRequest) bool {
	return csr.Spec.SignerName == certificatesv1.KubeAPIServerClientSignerName
}

// csrRecognizer used to identify whether the CSRs is the target CSRs and to perform authentication.
type csrRecognizer struct {
	// recognize identifies whether the CSRs is the target CSRs
	recognize func(csr *certificatesv1.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool
	// permission used to indicate the permissions required for auto csr approving.
	permission authorizationv1.ResourceAttributes
	// successMessage contains a human-readable message with details if auto csr approving is successful.
	successMessage string
}

// agentCSRRecognizers used to identify whether the CSRs is the agent CSRs and to perform authentication.
func agentCSRRecognizers() []csrRecognizer {
	recognizers := []csrRecognizer{
		{
			recognize:      isSelfAgentCSR,
			permission:     authorizationv1.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "selfclusteragent", Version: "*"},
			successMessage: "Auto approving self karmada agent certificate after SubjectAccessReview.",
		},
		{
			recognize:      isAgentCSR,
			permission:     authorizationv1.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "clusteragent", Version: "*"},
			successMessage: "Auto approving karmada agent certificate after SubjectAccessReview.",
		},
	}
	return recognizers
}

func appendApprovalCondition(csr *certificatesv1.CertificateSigningRequest, message string) {
	csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
		Type:    certificatesv1.CertificateApproved,
		Status:  corev1.ConditionTrue,
		Reason:  "AutoApproved",
		Message: message,
	})
}

// isAgentCSR determines if the provided csr is an agent csr.
// Agent csr is created for karmada-agent by bootstrap token during the cluster registering process.
// The 'signer' field must be set to "kubernetes.io/kube-apiserver-client".
// The 'Organization' field in the CertificateRequest must be "system:agents".
// The 'CommonName' must be prefixed with "system:agent:".
func isAgentCSR(csr *certificatesv1.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if csr.Spec.SignerName != certificatesv1.KubeAPIServerClientSignerName {
		return false
	}

	return ValidateAgentCSR(x509cr, usagesToSet(csr.Spec.Usages)) == nil
}

// isSelfAgentCSR determines if the provided csr is a self-agent csr.
// Self-agent csr is created by karmada-agent to enable certificate rotation feature.
// In contrast to the agent CSR, for a self-agent CSR, the username of the user who creates the `CertificateSigningRequest` must be identical to the 'CommonName' specified in the CertificateRequest.
func isSelfAgentCSR(csr *certificatesv1.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if csr.Spec.Username != x509cr.Subject.CommonName {
		return false
	}
	return isAgentCSR(csr, x509cr)
}

var (
	errOrganizationNotSystemAgents = fmt.Errorf("subject organization is not system:karmada:agents")
	errCommonNameNotSystemAgent    = fmt.Errorf("subject common name does not begin with system:karmada:agent: prefix")
	errDNSSANNotAllowed            = fmt.Errorf("DNS subjectAltNames are not allowed")
	errEmailSANNotAllowed          = fmt.Errorf("email subjectAltNames are not allowed")
	errIPSANNotAllowed             = fmt.Errorf("IP subjectAltNames are not allowed")
	errURISANNotAllowed            = fmt.Errorf("URI subjectAltNames are not allowed")
)

// ValidateAgentCSR used to determine if the CSR is a valid agent's CSR.
func ValidateAgentCSR(req *x509.CertificateRequest, usages sets.Set[string]) error {
	if !reflect.DeepEqual([]string{agentCSRGroup}, req.Subject.Organization) {
		return errOrganizationNotSystemAgents
	}

	if len(req.DNSNames) > 0 {
		return errDNSSANNotAllowed
	}

	if len(req.EmailAddresses) > 0 {
		return errEmailSANNotAllowed
	}

	if len(req.IPAddresses) > 0 {
		return errIPSANNotAllowed
	}

	if len(req.URIs) > 0 {
		return errURISANNotAllowed
	}

	if !strings.HasPrefix(req.Subject.CommonName, agentCSRUserPrefix) {
		return errCommonNameNotSystemAgent
	}

	if !agentRequiredUsages.Equal(usages) && !agentRequiredUsagesNoKeyEncipherment.Equal(usages) {
		return fmt.Errorf("usages did not match %v", sets.List(agentRequiredUsages))
	}

	return nil
}

var (
	agentRequiredUsagesNoKeyEncipherment = sets.New[string](
		string(certificatesv1.UsageDigitalSignature),
		string(certificatesv1.UsageClientAuth),
	)
	agentRequiredUsages = sets.New[string](
		string(certificatesv1.UsageDigitalSignature),
		string(certificatesv1.UsageKeyEncipherment),
		string(certificatesv1.UsageClientAuth),
	)
)

func usagesToSet(usages []certificatesv1.KeyUsage) sets.Set[string] {
	result := sets.New[string]()
	for _, usage := range usages {
		result.Insert(string(usage))
	}
	return result
}

// SetupWithManager creates a controller and registers to controller manager.
func (a *AgentCSRApprovingController) SetupWithManager(mgr controllerruntime.Manager) error {
	var predicateFunc = predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			csr := e.Object.(*certificatesv1.CertificateSigningRequest)
			// agent certificate is signed by "kubernetes.io/kube-apiserver-Client" signer
			return isIssuedByKubeAPIServerClientSigner(csr)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			newCSR := e.ObjectNew.(*certificatesv1.CertificateSigningRequest)
			// agent certificate is signed by "kubernetes.io/kube-apiserver-Client" signer
			return isIssuedByKubeAPIServerClientSigner(newCSR)
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(csrApprovingController).
		For(&certificatesv1.CertificateSigningRequest{}, builder.WithPredicates(predicateFunc)).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](a.RateLimiterOptions)}).
		Complete(a)
}
