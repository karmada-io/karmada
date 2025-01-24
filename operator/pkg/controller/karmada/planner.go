/*
Copyright 2023 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package karmada

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operator "github.com/karmada-io/karmada/operator/pkg"
	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/util"
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
	config  *rest.Config
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
		config:  config,
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
	var errs []error
	errs = append(errs, err)

	operatorv1alpha1.KarmadaFailed(p.karmada, operatorv1alpha1.Ready, err.Error())
	errs = append(errs, p.Client.Status().Update(context.TODO(), p.karmada))

	return utilerrors.NewAggregate(errs)
}

func (p *Planner) afterRunJob() error {
	if p.action == InitAction {
		// Update the karmada condition to Ready and set kubeconfig of karmada apiserver to karmada status.
		operatorv1alpha1.KarmadaCompleted(p.karmada, operatorv1alpha1.Ready, "karmada init job is completed")

		if !util.IsInCluster(p.karmada.Spec.HostCluster) {
			localClusterClient, err := clientset.NewForConfig(p.config)
			if err != nil {
				return fmt.Errorf("error when creating local cluster client, err: %w", err)
			}

			remoteClient, err := util.BuildClientFromSecretRef(localClusterClient, p.karmada.Spec.HostCluster.SecretRef)
			if err != nil {
				return fmt.Errorf("error when creating cluster client to install karmada, err: %w", err)
			}

			secret, err := remoteClient.CoreV1().Secrets(p.karmada.GetNamespace()).Get(context.TODO(), util.AdminKarmadaConfigSecretName(p.karmada.GetName()), metav1.GetOptions{})
			if err != nil {
				return err
			}

			_, err = localClusterClient.CoreV1().Secrets(p.karmada.GetNamespace()).Create(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: p.karmada.GetNamespace(),
					Name:      util.AdminKarmadaConfigSecretName(p.karmada.GetName()),
				},
				Data: secret.Data,
			}, metav1.CreateOptions{})
			if err != nil {
				return err
			}
		}

		p.karmada.Status.SecretRef = &operatorv1alpha1.LocalSecretReference{
			Namespace: p.karmada.GetNamespace(),
			Name:      util.AdminKarmadaConfigSecretName(p.karmada.GetName()),
		}
		p.karmada.Status.APIServerService = &operatorv1alpha1.APIServerService{
			Name: util.KarmadaAPIServerName(p.karmada.GetName()),
		}
		return p.Client.Status().Update(context.TODO(), p.karmada)
	}
	// if it is deInit workflow, the cr will be deleted with karmada is be deleted, so we need not to
	// update the karmada status.
	return nil
}
