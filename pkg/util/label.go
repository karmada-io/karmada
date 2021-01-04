package util

// GetLabelValue retrieves the value via 'labelKey' if exist, otherwise returns an empty string.
func GetLabelValue(labels map[string]string, labelKey string) string {
	if labels == nil {
		return ""
	}

	return labels[labelKey]
}
