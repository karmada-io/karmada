/*
Copyright 2022 The Karmada Authors.

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

package init

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/strings/slices"

	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/version"
)

// CommandAddonsEnableOption options for addons list.
type CommandAddonsEnableOption struct {
	GlobalCommandOptions

	// Karmada-search config
	KarmadaSearchImage    string
	KarmadaSearchReplicas int32
	SearchPriorityClass   string

	// Karmada-descheduler config
	KarmadaDeschedulerImage         string
	KarmadaDeschedulerReplicas      int32
	KarmadaDeschedulerPriorityClass string

	// Karmada-metrics-adapter config
	KarmadaMetricsAdapterImage         string
	KarmadaMetricsAdapterReplicas      int32
	KarmadaMetricsAdapterPriorityClass string

	// Karmada-estimator config
	KarmadaSchedulerEstimatorImage string
	KarmadaEstimatorReplicas       int32
	EstimatorPriorityClass         string

	KarmadaKubeClientSet *kubernetes.Clientset

	ImageRegistry string

	WaitComponentReadyTimeout int

	WaitAPIServiceReadyTimeout int

	MemberKubeConfig string

	MemberContext string

	HostClusterDomain string
}

var (
	// DefaultKarmadaDeschedulerImage Karmada descheduler image
	DefaultKarmadaDeschedulerImage string
	// DefaultKarmadaSchedulerEstimatorImage Karmada scheduler-estimator image
	DefaultKarmadaSchedulerEstimatorImage string
	// DefaultKarmadaSearchImage Karmada search image
	DefaultKarmadaSearchImage string
	// DefaultKarmadaMetricsAdapterImage Karmada metrics adapter image
	DefaultKarmadaMetricsAdapterImage string

	karmadaRelease string
)

func init() {
	releaseVer, err := version.ParseGitVersion(version.Get().GitVersion)
	if err != nil {
		klog.Infof("No default release version found. build version: %s", version.Get().String())
		releaseVer = &version.ReleaseVersion{} // initialize to avoid panic
	}
	karmadaRelease = releaseVer.ReleaseVersion()

	DefaultKarmadaDeschedulerImage = fmt.Sprintf("docker.io/karmada/karmada-descheduler:%s", releaseVer.ReleaseVersion())
	DefaultKarmadaSchedulerEstimatorImage = fmt.Sprintf("docker.io/karmada/karmada-scheduler-estimator:%s", releaseVer.ReleaseVersion())
	DefaultKarmadaSearchImage = fmt.Sprintf("docker.io/karmada/karmada-search:%s", releaseVer.ReleaseVersion())
	DefaultKarmadaMetricsAdapterImage = fmt.Sprintf("docker.io/karmada/karmada-metrics-adapter:%s", releaseVer.ReleaseVersion())
}

// KarmadaDeschedulerImage get karmada descheduler image
func KarmadaDeschedulerImage(o *CommandAddonsEnableOption) string {
	if o.ImageRegistry != "" && o.KarmadaDeschedulerImage == DefaultKarmadaDeschedulerImage {
		return o.ImageRegistry + "/karmada-descheduler:" + karmadaRelease
	}
	return o.KarmadaDeschedulerImage
}

// KarmadaSchedulerEstimatorImage get karmada scheduler-estimator image
func KarmadaSchedulerEstimatorImage(o *CommandAddonsEnableOption) string {
	if o.ImageRegistry != "" && o.KarmadaSchedulerEstimatorImage == DefaultKarmadaSchedulerEstimatorImage {
		return o.ImageRegistry + "/karmada-scheduler-estimator:" + karmadaRelease
	}
	return o.KarmadaSchedulerEstimatorImage
}

// KarmadaSearchImage get karmada search image
func KarmadaSearchImage(o *CommandAddonsEnableOption) string {
	if o.ImageRegistry != "" && o.KarmadaSearchImage == DefaultKarmadaSearchImage {
		return o.ImageRegistry + "/karmada-search:" + karmadaRelease
	}
	return o.KarmadaSearchImage
}

// KarmadaMetricsAdapterImage get karmada metrics adapter image
func KarmadaMetricsAdapterImage(o *CommandAddonsEnableOption) string {
	if o.ImageRegistry != "" && o.KarmadaMetricsAdapterImage == DefaultKarmadaMetricsAdapterImage {
		return o.ImageRegistry + "/karmada-metrics-adapter:" + karmadaRelease
	}
	return o.KarmadaMetricsAdapterImage
}

// Complete the conditions required to be able to run enable.
func (o *CommandAddonsEnableOption) Complete() error {
	err := o.GlobalCommandOptions.Complete()
	if err != nil {
		return err
	}

	o.KarmadaKubeClientSet, err = apiclient.NewClientSet(o.KarmadaRestConfig)
	if err != nil {
		return err
	}

	return nil
}

// Validate Check that there are enough conditions to run addon enable.
func (o *CommandAddonsEnableOption) Validate(args []string) error {
	err := validAddonNames(args)
	if err != nil {
		return err
	}

	secretClient := o.KubeClientSet.CoreV1().Secrets(o.Namespace)
	for _, addon := range getEnablingAddons(args) {
		if addon.Name == names.KarmadaSchedulerEstimatorComponentName {
			// estimator not rely on karmada config secret
			continue
		}
		karmadaConfigSecretName := util.KarmadaConfigName(addon.Name)
		_, err = secretClient.Get(context.TODO(), karmadaConfigSecretName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("secrets `%s` is not found in namespace %s, please execute karmadactl init to deploy karmada first", karmadaConfigSecretName, o.Namespace)
			}
		}
	}

	if o.Cluster == "" {
		if slices.Contains(args, names.KarmadaSchedulerEstimatorComponentName) {
			return fmt.Errorf("member cluster is needed when enable karmada-scheduler-estimator,use `--cluster=member --member-kubeconfig /root/.kube/config --member-context member1` to enable karmada-scheduler-estimator")
		}
	} else {
		if !slices.Contains(args, names.KarmadaSchedulerEstimatorComponentName) && !slices.Contains(args, "all") {
			return fmt.Errorf("cluster is needed only when enable karmada-scheduler-estimator or enable all")
		}
		if o.MemberKubeConfig == "" {
			return fmt.Errorf("member config is needed when enable karmada-scheduler-estimator,use `--cluster=member --member-kubeconfig /root/.kube/member.config --member-context member1` to enable karmada-scheduler-estimator")
		}

		// Check member kubeconfig and context is valid
		memberConfig, err := apiclient.RestConfig(o.MemberContext, o.MemberKubeConfig)
		if err != nil {
			return fmt.Errorf("failed to get member cluster config. error: %v", err)
		}
		memberKubeClient := kubernetes.NewForConfigOrDie(memberConfig)
		_, err = memberKubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to get nodes from cluster %s with member-kubeconfig and member-context. error: %v, Please check the Role or ClusterRole of the serviceAccount in your member-kubeconfig", o.Cluster, err)
		}
		_, err = memberKubeClient.CoreV1().Pods(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to get pods from cluster %s with member-kubeconfig and member-context. error: %v, Please check the Role or ClusterRole of the serviceAccount in your member-kubeconfig", o.Cluster, err)
		}
	}
	return nil
}

// Run start enable Karmada addons
func (o *CommandAddonsEnableOption) Run(args []string) error {
	// enable addons
	for name, addon := range getEnablingAddons(args) {
		klog.Infof("Start to enable addon %s", name)
		if err := addon.Enable(o); err != nil {
			klog.Errorf("Install addon %s failed", name)
			return err
		}
		klog.Infof("Successfully enable addon %s", name)
	}

	return nil
}

// validAddonNames valid whether addon names is supported now
func validAddonNames(addonNames []string) error {
	if len(addonNames) == 0 {
		return fmt.Errorf("addonNames must be not be null")
	}
	for _, addonName := range addonNames {
		if addonName == "all" {
			continue
		}
		_, ok := Addons[addonName]
		if !ok {
			return fmt.Errorf("addon %s is not be supported now", addonName)
		}
	}
	return nil
}

func getEnablingAddons(addonNames []string) map[string]*Addon {
	var enablingAddons = map[string]*Addon{}

	for _, name := range addonNames {
		if name == "all" {
			enablingAddons = Addons
			break
		}
		if addon := Addons[name]; addon != nil {
			enablingAddons[name] = addon
		}
	}
	return enablingAddons
}
