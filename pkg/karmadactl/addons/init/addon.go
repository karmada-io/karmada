package init

const (
	// AddonDisabledStatus describe a karmada addon is not installed
	AddonDisabledStatus = "disabled"

	// AddonEnabledStatus describe a karmada addon is installed
	AddonEnabledStatus = "enabled"

	// AddonUnhealthyStatus describe a karmada addon is unhealthy
	AddonUnhealthyStatus = "unhealthy"

	// AddonUnknownStatus describe a karmada addon is unknown
	AddonUnknownStatus = "unknown"
)

const (
	// DeschedulerResourceName define Descheduler Addon and component installed name
	DeschedulerResourceName = "karmada-descheduler"

	// EstimatorResourceName define Estimator Addon and component installed name
	EstimatorResourceName = "karmada-scheduler-estimator"

	// SearchResourceName define Search Addon and component installed name
	SearchResourceName = "karmada-search"

	// MetricsAdapterResourceName define metrics-adapter Addon and component installed name
	MetricsAdapterResourceName = "karmada-metrics-adapter"
)

// Addons hosts the optional components that support by karmada
var Addons = map[string]*Addon{}

// Addon describe how to enable or disable an optional component that support by karmada
type Addon struct {
	Name string

	// Status return current addon install status
	Status func(opts *CommandAddonsListOption) (string, error)

	// Enable install current addon in host cluster and Karmada control plane
	Enable func(opts *CommandAddonsEnableOption) error

	// Disable uninstall current addon in host cluster and Karmada control plane
	Disable func(opts *CommandAddonsDisableOption) error
}
