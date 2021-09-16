package validation

import (
	"fmt"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	kubevalidation "k8s.io/apimachinery/pkg/util/validation"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

const clusterNameMaxLength int = 48

// LabelValueMaxLength is a label's max length
const LabelValueMaxLength int = 63

// ValidateClusterName tests whether the cluster name passed is valid.
// If the cluster name is not valid, a list of error strings is returned. Otherwise an empty list (or nil) is returned.
// Rules of a valid cluster name:
// - Must be a valid label value as per RFC1123.
//   * An alphanumeric (a-z, and 0-9) string, with a maximum length of 63 characters,
//     with the '-' character allowed anywhere except the first or last character.
// - Length must be less than 48 characters.
//   * Since cluster name used to generate execution namespace by adding a prefix, so reserve 15 characters for the prefix.
func ValidateClusterName(name string) []string {
	if len(name) > clusterNameMaxLength {
		return []string{fmt.Sprintf("must be no more than %d characters", clusterNameMaxLength)}
	}

	return kubevalidation.IsDNS1123Label(name)
}

// ValidateClusterProxyURL tests whether the proxyURL is valid.
// If not valid, a list of error string is returned. Otherwise an empty list (or nil) is returned.
func ValidateClusterProxyURL(proxyURL string) []string {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return []string{fmt.Sprintf("cloud not parse: %s, %v", proxyURL, err)}
	}

	switch u.Scheme {
	case "http", "https", "socks5":
	default:
		return []string{fmt.Sprintf("unsupported scheme %q, must be http, https, or socks5", u.Scheme)}
	}

	return nil
}

// ValidatePolicyFieldSelector tests if the fieldSelector of propagation policy is valid.
func ValidatePolicyFieldSelector(fieldSelector *policyv1alpha1.FieldSelector) error {
	if fieldSelector == nil {
		return nil
	}

	for _, matchExpression := range fieldSelector.MatchExpressions {
		switch matchExpression.Key {
		case util.ProviderField, util.RegionField, util.ZoneField:
		default:
			return fmt.Errorf("unsupported key %q, must be provider, region, or zone", matchExpression.Key)
		}

		switch matchExpression.Operator {
		case corev1.NodeSelectorOpIn, corev1.NodeSelectorOpNotIn:
		default:
			return fmt.Errorf("unsupported operator %q, must be In or NotIn", matchExpression.Operator)
		}
	}

	return nil
}
