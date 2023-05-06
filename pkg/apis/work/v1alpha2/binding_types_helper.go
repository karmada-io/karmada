package v1alpha2

// TaskOptions represents options for GracefulEvictionTasks.
type TaskOptions struct {
	producer           string
	reason             string
	message            string
	gracePeriodSeconds *int32
	suppressDeletion   *bool
}

// Option configures a TaskOptions
type Option func(*TaskOptions)

// NewTaskOptions builds a TaskOptions
func NewTaskOptions(opts ...Option) *TaskOptions {
	options := TaskOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	return &options
}

// WithProducer sets the producer for TaskOptions
func WithProducer(producer string) Option {
	return func(o *TaskOptions) {
		o.producer = producer
	}
}

// WithReason sets the reason for TaskOptions
func WithReason(reason string) Option {
	return func(o *TaskOptions) {
		o.reason = reason
	}
}

// WithMessage sets the message for TaskOptions
func WithMessage(message string) Option {
	return func(o *TaskOptions) {
		o.message = message
	}
}

// WithGracePeriodSeconds sets the gracePeriodSeconds for TaskOptions
func WithGracePeriodSeconds(gracePeriodSeconds *int32) Option {
	return func(o *TaskOptions) {
		o.gracePeriodSeconds = gracePeriodSeconds
	}
}

// WithSuppressDeletion sets the suppressDeletion for TaskOptions
func WithSuppressDeletion(suppressDeletion *bool) Option {
	return func(o *TaskOptions) {
		o.suppressDeletion = suppressDeletion
	}
}

// TargetContains checks if specific cluster present on the target list.
func (s *ResourceBindingSpec) TargetContains(name string) bool {
	for i := range s.Clusters {
		if s.Clusters[i].Name == name {
			return true
		}
	}

	return false
}

// ClusterInGracefulEvictionTasks checks if the target cluster is in the GracefulEvictionTasks which means it is in the process of eviction.
func (s *ResourceBindingSpec) ClusterInGracefulEvictionTasks(name string) bool {
	for _, task := range s.GracefulEvictionTasks {
		if task.FromCluster == name {
			return true
		}
	}
	return false
}

// AssignedReplicasForCluster returns assigned replicas for specific cluster.
func (s *ResourceBindingSpec) AssignedReplicasForCluster(targetCluster string) int32 {
	for i := range s.Clusters {
		if s.Clusters[i].Name == targetCluster {
			return s.Clusters[i].Replicas
		}
	}

	return 0
}

// RemoveCluster removes specific cluster from the target list.
// This function no-opts if cluster not exist.
func (s *ResourceBindingSpec) RemoveCluster(name string) {
	var i int

	for i = 0; i < len(s.Clusters); i++ {
		if s.Clusters[i].Name == name {
			break
		}
	}

	// not found, do nothing
	if i >= len(s.Clusters) {
		return
	}

	s.Clusters = append(s.Clusters[:i], s.Clusters[i+1:]...)
}

// GracefulEvictCluster removes specific cluster from the target list in a graceful way by
// building a graceful eviction task.
// This function no-opts if the cluster does not exist.
func (s *ResourceBindingSpec) GracefulEvictCluster(name string, options *TaskOptions) {
	// find the cluster index
	var i int
	for i = 0; i < len(s.Clusters); i++ {
		if s.Clusters[i].Name == name {
			break
		}
	}
	// not found, do nothing
	if i >= len(s.Clusters) {
		return
	}

	evictCluster := s.Clusters[i]

	// remove the target cluster from scheduling result
	s.Clusters = append(s.Clusters[:i], s.Clusters[i+1:]...)

	// skip if the target cluster already in the task list
	if s.ClusterInGracefulEvictionTasks(evictCluster.Name) {
		return
	}

	// build eviction task
	evictingCluster := evictCluster.DeepCopy()
	evictionTask := GracefulEvictionTask{
		FromCluster:        evictingCluster.Name,
		Reason:             options.reason,
		Message:            options.message,
		Producer:           options.producer,
		GracePeriodSeconds: options.gracePeriodSeconds,
		SuppressDeletion:   options.suppressDeletion,
	}
	if evictingCluster.Replicas > 0 {
		evictionTask.Replicas = &evictingCluster.Replicas
	}
	s.GracefulEvictionTasks = append(s.GracefulEvictionTasks, evictionTask)
}
