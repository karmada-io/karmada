package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ResourceKindJuicefs            = "Juicefs"
	ResourcePluralJuicefs          = "juicefs"
	ResourceSingularJuicefs        = "juicefs"
	ResourceNamespaceScopedJuicefs = false
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Juicefs represents a JuiceFS storage resource
type Juicefs struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JuicefsSpec   `json:"spec,omitempty"`
	Status JuicefsStatus `json:"status,omitempty"`
}

// JuicefsSpec defines the specification for a JuiceFS storage resource
type JuicefsSpec struct {
	Description string    `json:"description,omitempty"`
	Location    *Location `json:"location,omitempty"`
	// +required
	Provider *Provider `json:"provider,omitempty"`
	Public   string    `json:"public,omitempty"`
	// +required
	Client   *JuicefsClient    `json:"client,omitempty"`
	Settings map[string]string `json:"settings,omitempty"`
	// +required
	Labor *Labor `json:"labor,omitempty"`
	// +required
	Auth *Auth `json:"auth,omitempty"`
}

// Auth represents the authentication configuration for a JuiceFS storage resource
type Auth struct {
	// +optional
	GCP *GCP `json:"gcp,omitempty"`
	// +optional
	OSS *AkSk `json:"oss,omitempty"`
	// +optional
	Azure *Azure `json:"azure,omitempty"`
	// +optional
	Aliyun *AkSk `json:"aliyun,omitempty"`
	// +optional
	Aws *AkSk `json:"aws,omitempty"`
	// +optional
	Docker *Harbor `json:"docker,omitempty"`
	// +optional
	Harbor *Harbor `json:"harbor,omitempty"`
}

// Labor represents the labor configuration for a JuiceFS storage resource
type Labor struct {
	// Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// +required
	RunScript string `json:"run-script,omitempty"`
	// +optional
	Envs []string `json:"envs,omitempty"`
}

// GCP represents the GCP configuration for a JuiceFS storage resource
type GCP struct {
	// +optional
	ServiceAccountCredentials string `json:"service-account-credentials,omitempty"`
	// +optional
	ApplicationDefaultCredentials string `json:"application-default-credentials,omitempty"`
}

// AkSk defines configuration for JuiceFS oss
type AkSk struct {
	// +required
	AccessKey string `json:"access-key,omitempty"`
	// +required
	SecretKey string `json:"secret-key,omitempty"`
}

// Azure represents the Azure configuration for a JuiceFS storage resource
type Azure struct {
	// +required
	ServicePrincipalID string `json:"service-principal-id,omitempty"`
	// +required
	ServicePrincipalSecret string `json:"service-principal-secret,omitempty"`
}

// Harbor represents the Harbor configuration for a JuiceFS storage resource
type Harbor struct {
	// +required
	Username string `json:"username,omitempty"`
	// +required
	Password string `json:"password,omitempty"`
	// +required
	URL string `json:"url,omitempty"`
	// +optional
	Insecure bool `json:"insecure,omitempty"`
}

// Location represents the storage location
type Location struct {
	Region string `json:"region,omitempty"`
	AZ     string `json:"az,omitempty"`
}

// Provider represents the storage provider information
type Provider struct {
	// +required
	ID string `json:"id,omitempty"`
	// +required
	Name string `json:"name,omitempty"`
}

// JuicefsClient defines the configuration for JuiceFS client
type JuicefsClient struct {
	// +required
	CacheDir     string   `json:"cache-dir,omitempty"`
	MountOptions []string `json:"mount-options,omitempty"`
	// +optional
	EE *EnterpriseEdition `json:"ee,omitempty"`
	// +optional
	CE *CommunityEdition `json:"ce,omitempty"`
}

// EnterpriseEdition defines configuration for JuiceFS enterprise edition
type EnterpriseEdition struct {
	// +required
	Version string `json:"version,omitempty"`
	// +required
	ConsoleWeb string `json:"console-web,omitempty"`
	// +required
	Token string `json:"token,omitempty"`
	// +optional
	Backend string `json:"backend,omitempty"`
}

// CommunityEdition defines configuration for JuiceFS community edition
type CommunityEdition struct {
	// +required
	Version string `json:"version,omitempty"`
	// +required
	MetaURL string `json:"meta-url,omitempty"`
	// +required
	Backend string `json:"backend,omitempty"`
}

// JuicefsStatus defines the observed state of JuiceFS resource
type JuicefsStatus struct {
	Phase      string             `json:"phase,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	MountInfo  MountInfo          `json:"mountInfo,omitempty"`
}

// MountInfo provides information about mounted JuiceFS
type MountInfo struct {
	MountedNodes []string    `json:"mountedNodes,omitempty"`
	LastMounted  metav1.Time `json:"lastMounted,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// JuicefsList contains a list of Juicefs resources
type JuicefsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Juicefs `json:"items"`
}
