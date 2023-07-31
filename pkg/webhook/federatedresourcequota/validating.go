package federatedresourcequota

import (
	"context"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	clustervalidation "github.com/karmada-io/karmada/pkg/apis/cluster/validation"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

// ValidatingAdmission validates FederatedResourceQuota object when creating/updating.
type ValidatingAdmission struct {
	Decoder *admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	quota := &policyv1alpha1.FederatedResourceQuota{}

	err := v.Decoder.Decode(req, quota)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating FederatedResourceQuote(%s) for request: %s", klog.KObj(quota).String(), req.Operation)

	if errs := validateFederatedResourceQuota(quota); len(errs) != 0 {
		klog.Errorf("%v", errs)
		return admission.Denied(errs.ToAggregate().Error())
	}

	return admission.Allowed("")
}

func validateFederatedResourceQuota(quota *policyv1alpha1.FederatedResourceQuota) field.ErrorList {
	errs := field.ErrorList{}
	errs = append(errs, validateFederatedResourceQuotaSpec(&quota.Spec, field.NewPath("spec"))...)
	errs = append(errs, validateFederatedResourceQuotaStatus(&quota.Status, field.NewPath("status"))...)
	return errs
}

func validateFederatedResourceQuotaSpec(quotaSpec *policyv1alpha1.FederatedResourceQuotaSpec, fld *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	errs = append(errs, validateResourceList(quotaSpec.Overall, fld.Child("overall"))...)

	fldPath := fld.Child("staticAssignments")
	for index := range quotaSpec.StaticAssignments {
		errs = append(errs, validateStaticAssignment(&quotaSpec.StaticAssignments[index], fldPath.Index(index))...)
	}

	errs = append(errs, validateOverallAndAssignments(quotaSpec, fld)...)

	return errs
}

func validateStaticAssignment(staticAssignment *policyv1alpha1.StaticClusterAssignment, fld *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if errMegs := clustervalidation.ValidateClusterName(staticAssignment.ClusterName); len(errMegs) > 0 {
		errs = append(errs, field.Invalid(fld.Child("clusterName"), staticAssignment.ClusterName, strings.Join(errMegs, ",")))
	}
	errs = append(errs, validateResourceList(staticAssignment.Hard, fld.Child("hard"))...)

	return errs
}

func validateOverallAndAssignments(quotaSpec *policyv1alpha1.FederatedResourceQuotaSpec, fld *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	overallPath := fld.Child("overall")
	for k, v := range quotaSpec.Overall {
		assignment := calculateAssignmentForResourceKey(k, quotaSpec.StaticAssignments)
		if v.Cmp(assignment) < 0 {
			errs = append(errs, field.Invalid(overallPath.Key(string(k)), v.String(), "overall is less than assignments"))
		}
	}

	staticAssignmentsPath := fld.Child("staticAssignments")
	for index, assignment := range quotaSpec.StaticAssignments {
		restPath := staticAssignmentsPath.Index(index).Child("hard")
		for k := range assignment.Hard {
			if _, exist := quotaSpec.Overall[k]; !exist {
				errs = append(errs, field.Invalid(restPath.Key(string(k)), k, "assignment resourceName is not exist in overall"))
			}
		}
	}

	return errs
}

func calculateAssignmentForResourceKey(resourceKey corev1.ResourceName, staticAssignments []policyv1alpha1.StaticClusterAssignment) resource.Quantity {
	quantity := resource.Quantity{}
	for index := range staticAssignments {
		q, exist := staticAssignments[index].Hard[resourceKey]
		if exist {
			quantity.Add(q)
		}
	}
	return quantity
}

func validateFederatedResourceQuotaStatus(quotaStatus *policyv1alpha1.FederatedResourceQuotaStatus, fld *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	errs = append(errs, validateResourceList(quotaStatus.Overall, fld.Child("overall"))...)
	errs = append(errs, validateResourceList(quotaStatus.OverallUsed, fld.Child("overallUsed"))...)

	fldPath := fld.Child("aggregatedStatus")
	for index := range quotaStatus.AggregatedStatus {
		errs = append(errs, validateClusterQuotaStatus(&quotaStatus.AggregatedStatus[index], fldPath.Index(index))...)
	}

	return errs
}

func validateClusterQuotaStatus(aggregatedStatus *policyv1alpha1.ClusterQuotaStatus, fld *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if errMegs := clustervalidation.ValidateClusterName(aggregatedStatus.ClusterName); len(errMegs) > 0 {
		errs = append(errs, field.Invalid(fld.Child("clusterName"), aggregatedStatus.ClusterName, strings.Join(errMegs, ",")))
	}
	errs = append(errs, validateResourceList(aggregatedStatus.Hard, fld.Child("hard"))...)
	errs = append(errs, validateResourceList(aggregatedStatus.Used, fld.Child("used"))...)

	return errs
}

func validateResourceList(resourceList corev1.ResourceList, fld *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	for k, v := range resourceList {
		resPath := fld.Key(string(k))
		errs = append(errs, lifted.ValidateResourceQuotaResourceName(string(k), resPath)...)
		errs = append(errs, lifted.ValidateResourceQuantityValue(string(k), v, resPath)...)
	}

	return errs
}
