package karmada

import (
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operator "github.com/karmada-io/karmada/operator/pkg"
	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	workflow "github.com/karmada-io/karmada/operator/pkg/workflow"
)

// Action is a intention corresponding karmada resource modification
type Action string

const (
	// InitAction represents init karmada instance
	InitAction Action = "init"
	// DeInitAction represents delete karmada instance
	DeInitAction Action = "deInit"
)

// Planner represents a planner to build a job woflow and startup it.
// the karmada resource change and enqueue is correspond to a action.
// it will create different job workflow according to action.
type Planner struct {
	action Action
	client.Client
	karmada *operatorv1alpha1.Karmada
	job     *workflow.Job
}

// NewPlannerFor creates planner, it will recognize the karmada resource action
// and create different job.
func NewPlannerFor(karmada *operatorv1alpha1.Karmada, c client.Client, config *rest.Config) (*Planner, error) {
	var job *workflow.Job

	action := recognizeActionFor(karmada)
	switch action {
	case InitAction:
		opts := []operator.InitOpt{
			operator.NewInitOptWithKarmada(karmada),
			operator.NewInitOptWithKubeconfig(config),
		}

		options := operator.NewJobInitOptions(opts...)
		job = operator.NewInitJob(options)

	case DeInitAction:
		opts := []operator.DeInitOpt{
			operator.NewDeInitOptWithKarmada(karmada),
			operator.NewDeInitOptWithKubeconfig(config),
		}

		options := operator.NewJobDeInitOptions(opts...)
		job = operator.NewDeInitDataJob(options)
	default:
		return nil, fmt.Errorf("failed to recognize action for karmada %s", karmada.Name)
	}

	return &Planner{
		karmada: karmada,
		Client:  c,
		job:     job,
		action:  action,
	}, nil
}

func recognizeActionFor(karmada *operatorv1alpha1.Karmada) Action {
	if !karmada.DeletionTimestamp.IsZero() {
		return DeInitAction
	}

	// TODO: support more action.

	return InitAction
}

// Execute starts a job workflow. if the workflow is error,
// TODO: the karmada resource will requeue and reconcile
func (p *Planner) Execute() error {
	klog.InfoS("Start execute the workflow", "workflow", p.action, "karmada", klog.KObj(p.karmada))

	if err := p.job.Run(); err != nil {
		return err
	}

	klog.InfoS("Successfully executed the workflow", "workflow", p.action, "karmada", klog.KObj(p.karmada))
	return nil
}
