package karmada

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operator "github.com/karmada-io/karmada/operator/pkg"
	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// Action is an intention corresponding karmada resource modification
type Action string

const (
	// InitAction represents init karmada instance
	InitAction Action = "init"
	// DeInitAction represents delete karmada instance
	DeInitAction Action = "deInit"
)

// Planner represents a planner to build a job workflow and startup it.
// the karmada resource change and enqueue is corresponded to an action.
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

	if err := p.preRunJob(); err != nil {
		return err
	}
	if err := p.job.Run(); err != nil {
		klog.ErrorS(err, "failed to executed the workflow", "workflow", p.action, "karmada", klog.KObj(p.karmada))
		return p.runJobErr(err)
	}
	if err := p.afterRunJob(); err != nil {
		return err
	}

	klog.InfoS("Successfully executed the workflow", "workflow", p.action, "karmada", klog.KObj(p.karmada))
	return nil
}

func (p *Planner) preRunJob() error {
	if p.action == InitAction {
		operatorv1alpha1.KarmadaInProgressing(p.karmada, operatorv1alpha1.Ready, "karmada init job is in progressing")
	}
	if p.action == DeInitAction {
		operatorv1alpha1.KarmadaInProgressing(p.karmada, operatorv1alpha1.Ready, "karmada deinit job is in progressing")
	}

	return p.Client.Status().Update(context.TODO(), p.karmada)
}

func (p *Planner) runJobErr(err error) error {
	operatorv1alpha1.KarmadaFailed(p.karmada, operatorv1alpha1.Ready, err.Error())
	return p.Client.Status().Update(context.TODO(), p.karmada)
}

func (p *Planner) afterRunJob() error {
	if p.action == InitAction {
		operatorv1alpha1.KarmadaCompleted(p.karmada, operatorv1alpha1.Ready, "karmada init job is completed")
		return p.Client.Status().Update(context.TODO(), p.karmada)
	}

	// if it is deInit workflow, the cr will be deleted with karmada is be deleted, so we need not to
	// update the karmada status.

	return nil
}
