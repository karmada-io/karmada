package descheduler

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	addoninit "github.com/karmada-io/karmada/pkg/karmadactl/addons/init"
	addonutils "github.com/karmada-io/karmada/pkg/karmadactl/addons/utils"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/kubernetes"
	initutils "github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
)

var karmadaDeschedulerLabels = map[string]string{"app": addoninit.DeschedulerResourceName}

// AddonDescheduler describe the descheduler addon command process
var AddonDescheduler = &addoninit.Addon{
	Name:    addoninit.DeschedulerResourceName,
	Status:  status,
	Enable:  enableDescheduler,
	Disable: disableDescheduler,
}

var status = func(opts *addoninit.CommandAddonsListOption) (string, error) {
	deployment, err := opts.KubeClientSet.AppsV1().Deployments(opts.Namespace).Get(context.TODO(), addoninit.DeschedulerResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return addoninit.AddonDisabledStatus, nil
		}
		return addoninit.AddonUnknownStatus, err
	}
	if deployment.Status.Replicas != deployment.Status.ReadyReplicas ||
		deployment.Status.Replicas != deployment.Status.AvailableReplicas {
		return addoninit.AddonUnhealthyStatus, nil
	}

	return addoninit.AddonEnabledStatus, nil
}

var enableDescheduler = func(opts *addoninit.CommandAddonsEnableOption) error {
	// install karmada descheduler deployment on host cluster
	karmadaDeschedulerDeploymentBytes, err := addonutils.ParseTemplate(karmadaDeschedulerDeployment, DeploymentReplace{
		Namespace: opts.Namespace,
		Replicas:  &opts.KarmadaDeschedulerReplicas,
		Image:     opts.KarmadaDeschedulerImage,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada descheduler deployment template :%w", err)
	}

	karmadaDeschedulerDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaDeschedulerDeploymentBytes, karmadaDeschedulerDeployment); err != nil {
		return fmt.Errorf("decode descheduler deployment error: %w", err)
	}

	if err := cmdutil.CreateOrUpdateDeployment(opts.KubeClientSet, karmadaDeschedulerDeployment); err != nil {
		return fmt.Errorf("create karmada descheduler deployment error: %w", err)
	}

	if err := kubernetes.WaitPodReady(opts.KubeClientSet, opts.Namespace, initutils.MapToString(karmadaDeschedulerLabels), opts.WaitPodReadyTimeout); err != nil {
		return fmt.Errorf("wait karmada descheduler pod timeout: %w", err)
	}

	klog.Infof("Install karmada descheduler deployment on host cluster successfully")
	return nil
}

var disableDescheduler = func(opts *addoninit.CommandAddonsDisableOption) error {
	// uninstall karmada descheduler deployment on host cluster
	deployClient := opts.KubeClientSet.AppsV1().Deployments(opts.Namespace)
	if err := deployClient.Delete(context.TODO(), addoninit.DeschedulerResourceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	klog.Infof("Uninstall karmada descheduler deployment on host cluster successfully")
	return nil
}
