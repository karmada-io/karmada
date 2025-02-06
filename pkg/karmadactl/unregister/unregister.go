/*
Copyright 2024 The Karmada Authors.

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

package unregister

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/cluster/validation"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/register"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

var (
	unregisterLong = templates.LongDesc(`
		Unregister removes a member cluster from Karmada, it will clean up the cluster object in the control plane and Karmada resources in the member cluster.`)

	unregisterExample = templates.Examples(`
		# Unregister cluster from karmada control plane
		%[1]s unregister CLUSTER_NAME --cluster-kubeconfig=<CLUSTER_KUBECONFIG> [--cluster-context=<CLUSTER_CONTEXT>]

		# Unregister cluster from karmada control plane with timeout
		%[1]s unregister CLUSTER_NAME --cluster-kubeconfig=<CLUSTER_KUBECONFIG> --wait 2m

		# Unregister cluster from karmada control plane, manually specify the location of the karmada config
		%[1]s unregister CLUSTER_NAME --karmada-config=<KARMADA_CONFIG> [--karmada-context=<KARMADA_CONTEXT>] --cluster-kubeconfig=<CLUSTER_KUBECONFIG> [--cluster-context=<CLUSTER_CONTEXT>]
    `)
)

// NewCmdUnregister defines the `unregister` command that removes registration of a pull mode cluster from control plane.
func NewCmdUnregister(parentCommand string) *cobra.Command {
	opts := CommandUnregisterOption{}

	cmd := &cobra.Command{
		Use:                   "unregister CLUSTER_NAME",
		Short:                 "Remove a pull mode cluster from Karmada control plane",
		Long:                  unregisterLong,
		Example:               fmt.Sprintf(unregisterExample, parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if err := opts.Complete(args); err != nil {
				return err
			}
			if err := opts.Validate(args); err != nil {
				return err
			}
			if err := opts.Run(); err != nil {
				return err
			}
			return nil
		},
		Annotations: map[string]string{
			cmdutil.TagCommandGroup: cmdutil.GroupClusterRegistration,
		},
	}

	flags := cmd.Flags()
	opts.AddFlags(flags)

	return cmd
}

// CommandUnregisterOption holds all command options.
type CommandUnregisterOption struct {
	// ClusterName is the cluster's name that we are going to unregister.
	ClusterName string

	// KarmadaConfig is the path of config to access karmada-apiserver.
	KarmadaConfig string

	// KarmadaContext is the context in KarmadaConfig file to access karmada-apiserver.
	KarmadaContext string

	// ClusterKubeConfig is the KUBECONFIG file path to access unregistering member cluster.
	ClusterKubeConfig string

	// ClusterContext is the context in ClusterKubeConfig to access unregistering member cluster.
	ClusterContext string

	// Namespace is the namespace that karmada-agent component deployed.
	Namespace string

	// AgentName is the deployment name that karmada-agent component deployed.
	AgentName string

	// ClusterNamespace holds namespace where the member cluster secrets are stored
	ClusterNamespace string

	// Wait tells maximum command execution time
	Wait time.Duration

	// DryRun tells if run the command in dry-run mode, without making any server requests.
	DryRun bool

	// ControlPlaneClient control plane client set
	ControlPlaneClient karmadaclientset.Interface

	// ControlPlaneKubeClient control plane kube client set
	ControlPlaneKubeClient kubeclient.Interface

	// MemberClusterClient member cluster client set
	MemberClusterClient kubeclient.Interface

	// rbacResources contains RBAC resources that grant the necessary permissions for the unregistering cluster to access to Karmada control plane.
	rbacResources *register.RBACResources
}

// AddFlags adds flags to the specified FlagSet.
func (j *CommandUnregisterOption) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&j.ClusterKubeConfig, "cluster-kubeconfig", "", "KUBECONFIG file path to access unregistering member cluster, required.")
	flags.StringVar(&j.ClusterContext, "cluster-context", "", "Context in cluster-kubeconfig to access unregistering member cluster, optional, defaults to current context.")
	flags.StringVar(&j.KarmadaConfig, "karmada-config", "", "Path of config to access karmada-apiserver, optional, defaults to fetch automatically from member cluster.")
	flags.StringVar(&j.KarmadaContext, "karmada-context", "", "Context in karmada-config to access karmada-apiserver, optional, defaults to current context.")

	flags.StringVarP(&j.Namespace, "namespace", "n", "karmada-system", "Namespace of the karmada-agent component deployed.")
	flags.StringVarP(&j.AgentName, "agent-name", "", names.KarmadaAgentComponentName, "Deployment name of the karmada-agent component deployed.")
	flags.StringVar(&j.ClusterNamespace, "cluster-namespace", options.DefaultKarmadaClusterNamespace, "Namespace in the control plane where member cluster secrets are stored.")
	flags.DurationVar(&j.Wait, "wait", 60*time.Second, "wait for the unjoin command execution process(default 60s), if there is no success after this time, timeout will be returned.")
	flags.BoolVar(&j.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
}

// Complete ensures that options are valid and marshals them if necessary.
func (j *CommandUnregisterOption) Complete(args []string) error {
	// Get cluster name from the command args.
	if len(args) > 0 {
		j.ClusterName = args[0]
	}

	j.rbacResources = register.GenerateRBACResources(j.ClusterName, j.ClusterNamespace)
	return nil
}

// Validate ensures that command unregister options are valid..
func (j *CommandUnregisterOption) Validate(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("only the cluster name is allowed as an argument")
	}
	if errMsgs := validation.ValidateClusterName(j.ClusterName); len(errMsgs) != 0 {
		return fmt.Errorf("invalid cluster name(%s): %s", j.ClusterName, strings.Join(errMsgs, ";"))
	}
	if j.ClusterKubeConfig == "" {
		return fmt.Errorf("--cluster-kubeconfig is required to specify KUBECONFIG file path to access unregistering member cluster")
	}
	if j.Wait <= 0 {
		return fmt.Errorf(" --wait %v  must be a positive duration, e.g. 1m0s ", j.Wait)
	}
	return nil
}

// Run is the implementation of the 'unregister' command.
func (j *CommandUnregisterOption) Run() error {
	klog.V(1).Infof("Unregistering cluster. cluster name: %s", j.ClusterName)
	klog.V(1).Infof("Unregistering cluster. karmada-agent deployed in namespace: %s", j.Namespace)
	klog.V(1).Infof("Unregistering cluster. member cluster secrets stored in namespace: %s", j.ClusterNamespace)

	// 1. build member cluster client
	err := j.buildClusterClientSet()
	if err != nil {
		return err
	}

	// 2. build karmada control plane client
	if j.KarmadaConfig != "" {
		err = j.buildKarmadaClientSetFromFile()
	} else {
		err = j.buildKarmadaClientSetFromAgent()
	}
	if err != nil {
		return err
	}

	return j.RunUnregisterCluster()
}

func (j *CommandUnregisterOption) buildClusterClientSet() error {
	restConfig, err := apiclient.RestConfig(j.ClusterContext, j.ClusterKubeConfig)
	if err != nil {
		return fmt.Errorf("failed to read member cluster rest config: %w", err)
	}
	j.MemberClusterClient, err = apiclient.NewClientSet(restConfig)
	if err != nil {
		return fmt.Errorf("failed to build member cluster clientset: %w", err)
	}
	return nil
}

func (j *CommandUnregisterOption) buildKarmadaClientSetFromFile() error {
	karmadaCfg, err := clientcmd.LoadFromFile(j.KarmadaConfig)
	if err != nil {
		return fmt.Errorf("failed to load karmada config: %w", err)
	}
	if j.KarmadaContext != "" {
		karmadaCfg.CurrentContext = j.KarmadaContext
	}
	j.ControlPlaneClient, err = register.ToKarmadaClient(karmadaCfg)
	if err != nil {
		return fmt.Errorf("failed to build karmada control plane clientset: %w", err)
	}
	j.ControlPlaneKubeClient, err = register.ToClientSet(karmadaCfg)
	if err != nil {
		return fmt.Errorf("failed to build kube control plane clientset: %w", err)
	}
	return nil
}

func (j *CommandUnregisterOption) buildKarmadaClientSetFromAgent() error {
	// 1. get karmada-agent deployment
	agent, err := j.MemberClusterClient.AppsV1().Deployments(j.Namespace).Get(context.TODO(), j.AgentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment karmada-agent in member cluster: %w", err)
	}

	// 2. get karmada config from karmada-agent deployment
	karmadaCfg, err := j.getKarmadaAgentConfig(agent)
	if err != nil {
		return fmt.Errorf("failed to get karmada config from karmada-agent deployment: %w", err)
	}

	// 3. get karmada context from karmada-agent deployment
	const karmadaContextPrefix = "--karmada-context="
	for _, cmd := range agent.Spec.Template.Spec.Containers[0].Command {
		if strings.HasPrefix(cmd, karmadaContextPrefix) {
			karmadaCfg.CurrentContext = cmd[len(karmadaContextPrefix):]
		}
	}

	j.ControlPlaneClient, err = register.ToKarmadaClient(karmadaCfg)
	if err != nil {
		return fmt.Errorf("failed to build karmada control plane clientset: %w", err)
	}
	j.ControlPlaneKubeClient, err = register.ToClientSet(karmadaCfg)
	if err != nil {
		return fmt.Errorf("failed to build kube control plane clientset: %w", err)
	}
	return nil
}

func (j *CommandUnregisterOption) getKarmadaAgentConfig(agent *appsv1.Deployment) (*clientcmdapi.Config, error) {
	var mountPath, fileName, volumeName, agentConfigSecretName string

	const karmadaConfigPrefix = "--karmada-kubeconfig="
	for _, cmd := range agent.Spec.Template.Spec.Containers[0].Command {
		if strings.HasPrefix(cmd, karmadaConfigPrefix) {
			karmadaConfigPath := cmd[len(karmadaConfigPrefix):]
			mountPath, fileName = filepath.Dir(karmadaConfigPath), filepath.Base(karmadaConfigPath)
		}
	}

	for _, mount := range agent.Spec.Template.Spec.Containers[0].VolumeMounts {
		if filepath.Clean(mount.MountPath) == mountPath {
			volumeName = mount.Name
		}
	}

	for _, volume := range agent.Spec.Template.Spec.Volumes {
		if volume.Name == volumeName {
			agentConfigSecretName = volume.Secret.SecretName
		}
	}

	if agentConfigSecretName == "" {
		return nil, fmt.Errorf("failed to get secret name of karmada agent config")
	}

	agentConfigSecret, err := j.MemberClusterClient.CoreV1().Secrets(j.Namespace).Get(context.TODO(), agentConfigSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get the secret which stores the karmada agent config")
	}
	if len(agentConfigSecret.Data[fileName]) == 0 {
		return nil, fmt.Errorf("empty data, secretName: %s, keyName: %s", agentConfigSecretName, fileName)
	}

	return clientcmd.Load(agentConfigSecret.Data[fileName])
}

// RunUnregisterCluster unregister the pull mode cluster from karmada.
func (j *CommandUnregisterOption) RunUnregisterCluster() error {
	target, err := j.ControlPlaneClient.ClusterV1alpha1().Clusters().Get(context.TODO(), j.ClusterName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return fmt.Errorf("no cluster object %s found in karmada control Plane", j.ClusterName)
	}
	if err != nil {
		klog.Errorf("Failed to get cluster object. cluster name: %s, error: %v", j.ClusterName, err)
		return err
	}
	if target.Spec.SyncMode == clusterv1alpha1.Push {
		return fmt.Errorf("cluster %s is a %s mode member cluster, please use command `unjoin` if you want to continue unjoining the cluster", j.ClusterName, target.Spec.SyncMode)
	}

	if j.DryRun {
		return nil
	}

	start := time.Now()
	// 1. delete the work object from the Karmada control plane
	// When deleting a cluster, the deletion triggers the removal of executionSpace, which can lead to the deletion of RBAC roles related to work.
	// Therefore, the deletion of work should be performed before deleting the cluster.
	err = cmdutil.EnsureWorksDeleted(j.ControlPlaneClient, names.GenerateExecutionSpaceName(j.ClusterName), j.Wait)
	if err != nil {
		klog.Errorf("Failed to delete works object. cluster name: %s, error: %v", j.ClusterName, err)
		return err
	}
	j.Wait = j.Wait - time.Since(start)

	// 2. delete the cluster object from the Karmada control plane
	//TODO: add flag --force to implement force deletion.
	if err = cmdutil.DeleteClusterObject(j.ControlPlaneKubeClient, j.ControlPlaneClient, j.ClusterName, j.Wait, j.DryRun, false); err != nil {
		klog.Errorf("Failed to delete cluster object. cluster name: %s, error: %v", j.ClusterName, err)
		return err
	}
	klog.Infof("Successfully delete cluster object (%s) from control plane.", j.ClusterName)

	if j.KarmadaConfig != "" {
		if err = j.rbacResources.Delete(j.ControlPlaneKubeClient); err != nil {
			klog.Errorf("Failed to delete karmada-agent RBAC resources from control plane. cluster name: %s, error: %v", j.ClusterName, err)
			return err
		}
		klog.Infof("Successfully delete karmada-agent RBAC resources from control plane. cluster name: %s", j.ClusterName)
	} else {
		klog.Warningf("The RBAC resources on the control plane need to be manually cleaned up, including the following resources:\n%s", j.rbacResources.ToString())
	}

	// 3. delete resource created by karmada in member cluster
	if err = j.cleanupMemberClusterResources(); err != nil {
		return err
	}

	// 4. delete local obsolete files generated by karmadactl
	localObsoleteFiles := []register.Obj{
		{Kind: "File", Name: filepath.Join(register.KarmadaDir, register.KarmadaAgentKubeConfigFileName)},
		{Kind: "File", Name: register.CACertPath},
	}
	for _, obj := range localObsoleteFiles {
		if err = os.Remove(obj.Name); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			klog.Errorf("Failed to delete local file (%v) in current node: %+v.", obj.Name, err)
			return err
		}
		klog.Infof("Successfully delete local file (%v) in current node.", obj.Name)
	}

	return nil
}

// cleanupMemberClusterResources clean up the resources which created by karmada in member cluster
func (j *CommandUnregisterOption) cleanupMemberClusterResources() error {
	var err error
	for _, resource := range j.listMemberClusterResources() {
		switch resource.Kind {
		case "ClusterRole":
			err = util.DeleteClusterRole(j.MemberClusterClient, resource.Name)
		case "ClusterRoleBinding":
			err = util.DeleteClusterRoleBinding(j.MemberClusterClient, resource.Name)
		case "ServiceAccount":
			err = util.DeleteServiceAccount(j.MemberClusterClient, resource.Namespace, resource.Name)
		case "Secret":
			err = util.DeleteSecret(j.MemberClusterClient, resource.Namespace, resource.Name)
		case "Deployment":
			err = deleteDeployment(j.MemberClusterClient, resource.Namespace, resource.Name)
		case "Namespace":
			err = util.DeleteNamespace(j.MemberClusterClient, resource.Name)
		}

		if err != nil {
			klog.Errorf("Failed to delete (%v) in unregistering cluster (%s): %+v.", resource, j.ClusterName, err)
			return err
		}
		klog.Infof("Successfully delete resource (%v) from member cluster (%s).", resource, j.ClusterName)
	}
	return err
}

// listMemberClusterResources lists resources to be deleted which created by karmada in member cluster
func (j *CommandUnregisterOption) listMemberClusterResources() []register.Obj {
	return []register.Obj{
		// the rbac resource prepared for karmada-controller-manager to access member cluster's kube-apiserver
		{Kind: "ServiceAccount", Namespace: j.ClusterNamespace, Name: names.GenerateServiceAccountName(j.ClusterName)},
		{Kind: "ClusterRole", Name: names.GenerateRoleName(names.GenerateServiceAccountName(j.ClusterName))},
		{Kind: "ClusterRoleBinding", Name: names.GenerateRoleName(names.GenerateServiceAccountName(j.ClusterName))},
		{Kind: "Secret", Namespace: j.ClusterNamespace, Name: names.GenerateServiceAccountName(j.ClusterName)},
		// the rbac resource prepared for karmada-aggregated-apiserver to access member cluster's kube-apiserver
		{Kind: "ServiceAccount", Namespace: j.ClusterNamespace, Name: names.GenerateServiceAccountName("impersonator")},
		{Kind: "Secret", Namespace: j.ClusterNamespace, Name: names.GenerateServiceAccountName("impersonator")},
		// the namespace to store above rbac resources
		{Kind: "Namespace", Name: j.ClusterNamespace},

		// the deployment of karmada-agent
		{Kind: "Deployment", Namespace: j.Namespace, Name: names.KarmadaAgentComponentName},
		// the rbac resources used by karmada-agent to access the member cluster's kube-apiserver
		{Kind: "ServiceAccount", Namespace: j.Namespace, Name: register.KarmadaAgentServiceAccountName},
		{Kind: "ClusterRole", Name: names.KarmadaAgentComponentName},
		{Kind: "ClusterRoleBinding", Name: names.KarmadaAgentComponentName},
		// the karmada config used by karmada-agent to access karmada-apiserver
		{Kind: "Secret", Namespace: j.Namespace, Name: register.KarmadaKubeconfigName},
	}
}

// deleteDeployment just try to delete the Deployment.
func deleteDeployment(client kubeclient.Interface, namespace, name string) error {
	err := client.AppsV1().Deployments(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
