package util

const (
	// ServiceNamespaceLabel is added to work object, which is report by member cluster, to specify service namespace associated with EndpointSlice.
	ServiceNamespaceLabel = "endpointslice.karmada.io/namespace"

	// ServiceNameLabel is added to work object, which is report by member cluster, to specify service name associated with EndpointSlice.
	ServiceNameLabel = "endpointslice.karmada.io/name"

	// PropagationInstruction is used to mark a resource(like Work) propagation instruction.
	// Valid values includes:
	// - suppressed: indicates that the resource should not be propagated.
	//
	// Note: This instruction is intended to set on Work objects to indicate the Work should be ignored by
	// execution controller. The instruction maybe deprecated once we extend the Work API and no other scenario want this.
	PropagationInstruction = "propagation.karmada.io/instruction"
)

// Define annotations used by karmada system.
const (
	// PolicyPlacementAnnotation is the annotation of a policy's placement.
	// It is intended to set on ResourceBinding or ClusterResourceBinding objects to record applied placement declaration.
	// The placement could be either PropagationPolicy's or ClusterPropagationPolicy's.
	PolicyPlacementAnnotation = "policy.karmada.io/applied-placement"

	// AppliedOverrides is the annotation which used to record override items an object applied.
	// It is intended to set on Work objects to record applied overrides.
	// The overrides items should be sorted alphabetically in ascending order by OverridePolicy's name.
	AppliedOverrides = "policy.karmada.io/applied-overrides"

	// AppliedClusterOverrides is the annotation which used to record override items an object applied.
	// It is intended to set on Work objects to record applied overrides.
	// The overrides items should be sorted alphabetically in ascending order by ClusterOverridePolicy's name.
	AppliedClusterOverrides = "policy.karmada.io/applied-cluster-overrides"
)

// Define finalizers used by karmada system.
const (
	// ClusterControllerFinalizer is added to Cluster to ensure Work as well as the
	// execution space (namespace) is deleted before itself is deleted.
	ClusterControllerFinalizer = "karmada.io/cluster-controller"

	// ExecutionControllerFinalizer is added to Work to ensure manifests propagated to member cluster
	// is deleted before Work itself is deleted.
	ExecutionControllerFinalizer = "karmada.io/execution-controller"
)

const (
	// ProviderField indicates the 'provider' field of a cluster
	ProviderField = "provider"
	// RegionField indicates the 'region' field of a cluster
	RegionField = "region"
	// ZoneField indicates the 'zone' field of a cluster
	ZoneField = "zone"
)

// Define resource kind.
const (
	// DeploymentKind indicates the target resource is a deployment
	DeploymentKind = "Deployment"
	// ServiceKind indicates the target resource is a service
	ServiceKind = "Service"
	// IngressKind indicates the target resource is a ingress
	IngressKind = "Ingress"
	// JobKind indicates the target resource is a job
	JobKind = "Job"
	// PodKind indicates the target resource is a pod
	PodKind = "Pod"
	// ServiceAccountKind indicates the target resource is a serviceaccount
	ServiceAccountKind = "ServiceAccount"
	// ReplicaSetKind indicates the target resource is a replicaset
	ReplicaSetKind = "ReplicaSet"
	// StatefulSetKind indicates the target resource is a statefulset
	StatefulSetKind = "StatefulSet"
	// DaemonSetKind indicates the target resource is a daemonset
	DaemonSetKind = "DaemonSet"
	// EndpointSliceKind indicates the target resource is a endpointslice
	EndpointSliceKind = "EndpointSlice"
	// PersistentVolumeClaimKind indicated the target resource is a persistentvolumeclaim
	PersistentVolumeClaimKind = "PersistentVolumeClaim"

	// ServiceExportKind indicates the target resource is a serviceexport crd
	ServiceExportKind = "ServiceExport"
	// ServiceImportKind indicates the target resource is a serviceimport crd
	ServiceImportKind = "ServiceImport"

	// CRDKind indicated the target resource is a CustomResourceDefinition
	CRDKind = "CustomResourceDefinition"
)

// Define resource filed
const (
	// SpecField indicates the 'spec' field of a resource
	SpecField = "spec"
	// ReplicasField indicates the 'replicas' field of a resource
	ReplicasField = "replicas"
	// TemplateField indicates the 'template' field of a resource
	TemplateField = "template"
)

const (
	// PropagationInstructionSuppressed indicates that the resource should not be propagated.
	PropagationInstructionSuppressed = "suppressed"
)

const (
	// NamespaceKarmadaSystem is the karmada system namespace.
	NamespaceKarmadaSystem = "karmada-system"
)
