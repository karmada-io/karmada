package options

const (
	// CaCertAndKeyName ca certificate key name
	CaCertAndKeyName = "ca"
	// EtcdCaCertAndKeyName etcd ca certificate key name
	EtcdCaCertAndKeyName = "etcd-ca"
	// EtcdServerCertAndKeyName etcd server certificate key name
	EtcdServerCertAndKeyName = "etcd-server"
	// EtcdClientCertAndKeyName etcd client certificate key name
	EtcdClientCertAndKeyName = "etcd-client"
	// KarmadaCertAndKeyName karmada certificate key name
	KarmadaCertAndKeyName = "karmada"
	// ApiserverCertAndKeyName karmada apiserver certificate key name
	ApiserverCertAndKeyName = "apiserver"
	// FrontProxyCaCertAndKeyName front-proxy-client  certificate key name
	FrontProxyCaCertAndKeyName = "front-proxy-ca"
	// FrontProxyClientCertAndKeyName front-proxy-client  certificate key name
	FrontProxyClientCertAndKeyName = "front-proxy-client"
	// ClusterName karmada cluster name
	ClusterName = "karmada-apiserver"
	// UserName karmada cluster user name
	UserName = "karmada-admin"
	// KarmadaKubeConfigName karmada kubeconfig name
	KarmadaKubeConfigName = "karmada-apiserver.config"
	// WaitComponentReadyTimeout wait component ready time
	WaitComponentReadyTimeout = 30
)
