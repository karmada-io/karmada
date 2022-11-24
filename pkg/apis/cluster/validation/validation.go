package validation

import (
	"fmt"
	"math"
	"net/url"

	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	api "github.com/karmada-io/karmada/pkg/apis/cluster"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

const clusterNameMaxLength int = 48

// ValidateClusterName tests whether the cluster name passed is valid.
// If the cluster name is not valid, a list of error strings is returned. Otherwise an empty list (or nil) is returned.
// Rules of a valid cluster name:
// - Must be a valid label value as per RFC1123.
//   - An alphanumeric (a-z, and 0-9) string, with a maximum length of 63 characters,
//     with the '-' character allowed anywhere except the first or last character.
//
// - Length must be less than 48 characters.
//   - Since cluster name used to generate execution namespace by adding a prefix, so reserve 15 characters for the prefix.
func ValidateClusterName(name string) []string {
	if len(name) == 0 {
		return []string{"must be not empty"}
	}
	if len(name) > clusterNameMaxLength {
		return []string{fmt.Sprintf("must be no more than %d characters", clusterNameMaxLength)}
	}

	return kubevalidation.IsDNS1123Label(name)
}

var (
	supportedSyncModes     = sets.NewString(string(api.Pull), string(api.Push))
	supportedResourceNames = sets.NewString(string(api.ResourceCPU), string(api.ResourceMemory), string(api.ResourceStorage), string(api.ResourceEphemeralStorage))
)

// ValidateCluster tests if required fields in the Cluster are set.
func ValidateCluster(cluster *api.Cluster) field.ErrorList {
	allErrs := apimachineryvalidation.ValidateObjectMeta(&cluster.ObjectMeta, false, func(name string, prefix bool) []string { return ValidateClusterName(name) }, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateClusterSpec(&cluster.Spec, field.NewPath("spec"))...)
	return allErrs
}

// ValidateClusterUpdate tests if required fields in the Cluster are set.
func ValidateClusterUpdate(newCluster, oldCluster *api.Cluster) field.ErrorList {
	allErrs := apimachineryvalidation.ValidateObjectMetaUpdate(&newCluster.ObjectMeta, &oldCluster.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateCluster(newCluster)...)
	return allErrs
}

// ValidateClusterSpec tests if required fields in the ClusterSpec are set.
func ValidateClusterSpec(spec *api.ClusterSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if spec.SyncMode == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("syncMode"), ""))
	}
	if !supportedSyncModes.Has(string(spec.SyncMode)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("syncMode"), spec.SyncMode, supportedSyncModes.List()))
	}

	if spec.APIEndpoint != "" {
		allErrs = append(allErrs, ValidateClusterAPIEndpoint(fldPath.Child("apiEndpoint"), spec.APIEndpoint, false)...)
	}

	if spec.SecretRef != nil {
		if spec.SecretRef.Namespace == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("secretRef").Child("namespace"), ""))
		}
		if spec.SecretRef.Name == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("secretRef").Child("name"), ""))
		}
	}

	if spec.ImpersonatorSecretRef != nil {
		if spec.ImpersonatorSecretRef.Namespace == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("impersonatorSecretRef").Child("namespace"), ""))
		}
		if spec.ImpersonatorSecretRef.Name == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("impersonatorSecretRef").Child("name"), ""))
		}
	}

	if spec.ProxyURL != "" {
		allErrs = append(allErrs, ValidateClusterProxyURL(fldPath.Child("proxyURL"), spec.ProxyURL)...)
	}

	if len(spec.Taints) > 0 {
		allErrs = append(allErrs, lifted.ValidateClusterTaints(spec.Taints, fldPath.Child("taints"))...)
	}

	if len(spec.ResourceModels) > 0 {
		if err := ValidateClusterResourceModels(fldPath.Child("resourceModels"), spec.ResourceModels); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	return allErrs
}

// ValidateClusterAPIEndpoint validates cluster's apiEndpoint
func ValidateClusterAPIEndpoint(fldPath *field.Path, apiEndpoint string, forceHTTPS bool) field.ErrorList {
	var allErrs field.ErrorList
	const form = "; desired format: hostname, hostname:port, IP or IP:port"
	if u, err := url.Parse(apiEndpoint); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, apiEndpoint, "apiEndpoint must be a valid URL: "+err.Error()+form))
	} else {
		if forceHTTPS && u.Scheme != "https" {
			allErrs = append(allErrs, field.Invalid(fldPath, u.Scheme, "'https' is the only allowed URL scheme"+form))
		}
		if len(u.Host) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath, u.Host, "host must be provided"+form))
		}
		if u.User != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, u.User.String(), "user information is not permitted in the URL"))
		}
		if len(u.Fragment) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath, u.Fragment, "fragments are not permitted in the URL"))
		}
		if len(u.RawQuery) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath, u.RawQuery, "query parameters are not permitted in the URL"))
		}
	}
	return allErrs
}

// ValidateClusterProxyURL validates cluster's proxyURL.
func ValidateClusterProxyURL(fldPath *field.Path, proxyURL string) field.ErrorList {
	allErrs := field.ErrorList{}
	if u, err := url.Parse(proxyURL); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, proxyURL, "apiEndpoint must be a valid URL: "+err.Error()))
	} else {
		switch u.Scheme {
		case "http", "https", "socks5":
		default:
			allErrs = append(allErrs, field.Invalid(fldPath, proxyURL, fmt.Sprintf("unsupported scheme %q, must be http, https, or socks5", u.Scheme)))
		}
	}
	return allErrs
}

// ValidateClusterResourceModels validates cluster's resource models.
func ValidateClusterResourceModels(fldPath *field.Path, models []api.ResourceModel) *field.Error {
	for i, resourceModel := range models {
		if i != 0 && resourceModel.Grade == models[i-1].Grade {
			return field.Invalid(fldPath, models, "The grade of each models should not be the same")
		}

		if i != 0 && len(models[i-1].Ranges) != len(resourceModel.Ranges) {
			return field.Invalid(fldPath, models, "The number of resource types should be the same")
		}

		for j, resourceModelRange := range resourceModel.Ranges {
			if !supportedResourceNames.Has(string(resourceModelRange.Name)) {
				return field.NotSupported(fldPath.Child("ranges").Child("name"), resourceModelRange.Name, supportedResourceNames.List())
			}
			if resourceModelRange.Max.Cmp(resourceModelRange.Min) <= 0 {
				return field.Invalid(fldPath, models, "The max value of each resource must be greater than the min value")
			}
			if i == 0 {
				if !resourceModelRange.Min.IsZero() {
					return field.Invalid(fldPath, models, "The min value of each resource in the first model should be 0")
				}
			} else if models[i-1].Ranges[j].Name != resourceModelRange.Name {
				return field.Invalid(fldPath, models, "The resource types of each models should be the same")
			} else if models[i-1].Ranges[j].Max != resourceModelRange.Min {
				return field.Invalid(fldPath, models, "Model intervals for resources must be contiguous and non-overlapping")
			}

			if i == len(models)-1 {
				if resourceModelRange.Max.Value() != math.MaxInt64 {
					return field.Invalid(fldPath, models, "The max value of each resource in the last model should be MaxInt64")
				}
			}
		}
	}
	return nil
}
