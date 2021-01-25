package validation

import (
	"fmt"

	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
)

const clusterNameMaxLength int = 48

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
