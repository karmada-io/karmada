package genericmanager

import (
	"context"
	"time"

	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	generatedclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	generatedinformer "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	generatedpolicylister "github.com/karmada-io/karmada/pkg/generated/listers/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

// GeneratedInformerManager manages karmada shared informer for propagationPolicy and clusterPropagationPolicy.
type GeneratedInformerManager interface {
	// ForPropagationPolicy builds a propagationPolicy informer,the event handler will be appended to the informer.
	ForPropagationPolicy(handler cache.ResourceEventHandler)
	// ForClusterPropagationPolicy builds a clusterPropagationPolicy informer,the event handler will be appended to the informer.
	ForClusterPropagationPolicy(handler cache.ResourceEventHandler)
	// PropagationPolicyLister builds a clusterPropagationPolicy lister.
	PropagationPolicyLister() generatedpolicylister.PropagationPolicyLister
	// ClusterPropagationPolicyLister builds a clusterPropagationPolicy lister.
	ClusterPropagationPolicyLister() generatedpolicylister.ClusterPropagationPolicyLister
	// Start will run all informers, the informers will keep running until the channel closed.
	Start()
	// Stop stops all informers. Once it is stopped, it will be not able
	// to Start again.
	Stop()
	// WaitForCacheSync waits for all caches to populate.
	WaitForCacheSync()
	// Context returns the single cluster context.
	Context() context.Context
	// GetClient returns the karmada clientset client.
	GetClient() generatedclientset.Interface
}

// NewGeneratedInformerManager constructs a new instance of generatedInformerManagerImpl.
// defaultResync with value '0' means no re-sync.
func NewGeneratedInformerManager(client generatedclientset.Interface, defaultResync time.Duration, parentCh <-chan struct{}) GeneratedInformerManager {
	ctx, cancel := util.ContextForChannel(parentCh)
	return &generatedInformerManagerImpl{
		generatedInformer: generatedinformer.NewSharedInformerFactory(client, defaultResync),
		ctx:               ctx,
		cancel:            cancel,
		client:            client,
	}
}

type generatedInformerManagerImpl struct {
	ctx    context.Context
	cancel context.CancelFunc

	generatedInformer generatedinformer.SharedInformerFactory

	client generatedclientset.Interface
}

func (g *generatedInformerManagerImpl) ForPropagationPolicy(handler cache.ResourceEventHandler) {
	_, err := g.generatedInformer.Policy().V1alpha1().PropagationPolicies().Informer().AddEventHandler(handler)
	if err != nil {
		klog.Errorf("Failed to add handler for propagationPolicy: %v", err)
		return
	}
}

func (g *generatedInformerManagerImpl) ForClusterPropagationPolicy(handler cache.ResourceEventHandler) {
	_, err := g.generatedInformer.Policy().V1alpha1().ClusterPropagationPolicies().Informer().AddEventHandler(handler)
	if err != nil {
		klog.Errorf("Failed to add handler for clusterPropagationPolicy: %v", err)
		return
	}
}

func (g *generatedInformerManagerImpl) PropagationPolicyLister() generatedpolicylister.PropagationPolicyLister {
	return g.generatedInformer.Policy().V1alpha1().PropagationPolicies().Lister()
}

func (g *generatedInformerManagerImpl) ClusterPropagationPolicyLister() generatedpolicylister.ClusterPropagationPolicyLister {
	return g.generatedInformer.Policy().V1alpha1().ClusterPropagationPolicies().Lister()
}

func (g *generatedInformerManagerImpl) Start() {
	g.generatedInformer.Start(g.ctx.Done())
}

func (g *generatedInformerManagerImpl) Stop() {
	g.cancel()
}

func (g *generatedInformerManagerImpl) WaitForCacheSync() {
	g.generatedInformer.WaitForCacheSync(g.ctx.Done())
}

func (g *generatedInformerManagerImpl) Context() context.Context {
	return g.ctx
}

func (g *generatedInformerManagerImpl) GetClient() generatedclientset.Interface {
	return g.client
}
