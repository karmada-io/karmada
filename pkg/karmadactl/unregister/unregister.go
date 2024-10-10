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
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"

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
		Remove a Pull mode cluster from Karmada control plane.`)

	unregisterExample = templates.Examples(`
		# Unregister cluster from karmada control plane
		%[1]s unregister CLUSTER_NAME

		# Unregister cluster from karmada control plane with timeout
		%[1]s unregister CLUSTER_NAME --wait 2m

		# Unregister cluster from karmada control plane, explicitly specifying the kubeconfig and context of the member cluster
		%[1]s unregister CLUSTER_NAME --kubeconfig=<CLUSTER_KUBECONFIG> [--context=<CLUSTER_CONTEXT>]`)
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

	// KubeConfig holds the KUBECONFIG file path for the unregistering cluster.
	KubeConfig string

	// Context is the name of the cluster context in KUBECONFIG file.
	// Default value is the current-context.
	Context string

	// Namespace is the namespace that karmada-agent component deployed.
	Namespace string

	// ClusterNamespace holds namespace where the member cluster secrets are stored
	ClusterNamespace string

	// Wait tells maximum command execution time
	Wait time.Duration

	// DryRun tells if run the command in dry-run mode, without making any server requests.
	DryRun bool

	controlPlaneClient  *karmadaclientset.Clientset
	memberClusterClient *kubeclient.Clientset
}

// AddFlags adds flags to the specified FlagSet.
func (j *CommandUnregisterOption) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&j.KubeConfig, "kubeconfig", "", "Path to the kubeconfig file of member cluster.")
	flags.StringVar(&j.Context, "context", "", "Name of the cluster context in kubeconfig file.")
	flags.StringVarP(&j.Namespace, "namespace", "n", "karmada-system", "Namespace the karmada-agent component deployed.")
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
	return nil
}

// Validate ensures that command unjoin options are valid.
func (j *CommandUnregisterOption) Validate(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("only the cluster name is allowed as an argument")
	}
	if len(j.ClusterName) == 0 {
		return fmt.Errorf("cluster name is required")
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
	restConfig, err := apiclient.RestConfig(j.Context, j.KubeConfig)
	if err != nil {
		return fmt.Errorf("failed to read member cluster rest config: %w", err)
	}
	j.memberClusterClient, err = apiclient.NewClientSet(restConfig)
	if err != nil {
		return fmt.Errorf("failed to build member cluster clientset: %w", err)
	}

	// 2. build karmada control plane client (read kubeconfig from member cluster's secret which stores the karmada kubeconfig for karmada agent)
	karmadaConfigSecret, err := j.memberClusterClient.CoreV1().Secrets(j.Namespace).Get(context.TODO(), register.KarmadaKubeconfigName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get the secret which stores the karmada kubeconfig")
	}
	karmadaCfg, err := clientcmd.Load(karmadaConfigSecret.Data[register.KarmadaKubeconfigName])
	if err != nil {
		return err
	}
	if specifiedKarmadaContext := j.getSpecifiedKarmadaContext(j.memberClusterClient); specifiedKarmadaContext != "" {
		karmadaCfg.CurrentContext = specifiedKarmadaContext
	}
	j.controlPlaneClient, err = register.ToKarmadaClient(karmadaCfg)
	if err != nil {
		return err
	}

	return j.RunUnregisterCluster()
}

type obj struct{ Kind, Name, Namespace string }

func (o *obj) ToString() string {
	if o.Namespace == "" {
		return fmt.Sprintf("%s/%s", o.Kind, o.Name)
	}
	return fmt.Sprintf("%s/%s/%s", o.Kind, o.Namespace, o.Name)
}

// RunUnregisterCluster unregister the pull mode cluster from karmada.
func (j *CommandUnregisterOption) RunUnregisterCluster() error {
	if j.DryRun {
		return nil
	}

	// 1. delete the cluster object in host cluster that associates the unregistering cluster
	if err := cmdutil.DeleteClusterObject(j.controlPlaneClient, j.ClusterName, j.Wait, j.DryRun); err != nil {
		klog.Errorf("Failed to delete cluster object. cluster name: %s, error: %v", j.ClusterName, err)
		return err
	}
	klog.Infof("Successfully delete cluster object (%s) from control plane.", j.ClusterName)

	// 2. delete resource created by karmada in member cluster
	clusterResources := []obj{
		// 2.1 delete rbac resource which should upload to control plane to serve the controller-manager and aggregated-apiserver
		{Kind: "ClusterRole", Name: names.GenerateRoleName(names.GenerateServiceAccountName(j.ClusterName))},
		{Kind: "ClusterRoleBinding", Name: names.GenerateRoleName(names.GenerateServiceAccountName(j.ClusterName))},
		{Kind: "ServiceAccount", Namespace: j.ClusterNamespace, Name: names.GenerateServiceAccountName(j.ClusterName)},
		{Kind: "Secret", Namespace: j.ClusterNamespace, Name: names.GenerateServiceAccountName(j.ClusterName)},
		{Kind: "ServiceAccount", Namespace: j.ClusterNamespace, Name: names.GenerateServiceAccountName("impersonator")},
		{Kind: "Secret", Namespace: j.ClusterNamespace, Name: names.GenerateServiceAccountName("impersonator")},
		{Kind: "Namespace", Name: j.ClusterNamespace},

		// 2.2 delete karmada-agent deployment, its own rbac resource and secret/karmada-kubeconfig
		{Kind: "ClusterRole", Name: register.KarmadaAgentName},
		{Kind: "ClusterRoleBinding", Name: register.KarmadaAgentName},
		{Kind: "ServiceAccount", Namespace: j.Namespace, Name: register.KarmadaAgentServiceAccountName},
		{Kind: "Secret", Namespace: j.Namespace, Name: register.KarmadaKubeconfigName},
		{Kind: "Deployment", Namespace: j.Namespace, Name: register.KarmadaAgentName},
	}

	var err error
	for _, resource := range clusterResources {
		switch resource.Kind {
		case "ClusterRole":
			err = util.DeleteClusterRole(j.memberClusterClient, resource.Name)
		case "ClusterRoleBinding":
			err = util.DeleteClusterRoleBinding(j.memberClusterClient, resource.Name)
		case "ServiceAccount":
			err = util.DeleteServiceAccount(j.memberClusterClient, resource.Namespace, resource.Name)
		case "Secret":
			err = util.DeleteSecret(j.memberClusterClient, resource.Namespace, resource.Name)
		case "Deployment":
			err = deleteDeployment(j.memberClusterClient, resource.Namespace, resource.Name)
		case "Namespace":
			err = util.DeleteNamespace(j.memberClusterClient, resource.Name)
		}

		if err != nil {
			klog.Errorf("Failed to delete (%v) in unregistering cluster %s: %+v.", resource, j.ClusterName, err)
			return err
		}
		klog.Infof("Successfully delete resource (%v) from member cluster %s.", resource, j.ClusterName)
	}

	return nil
}

func (j *CommandUnregisterOption) getSpecifiedKarmadaContext(client kubeclient.Interface) string {
	agent, err := client.AppsV1().Deployments(j.Namespace).Get(context.TODO(), register.KarmadaAgentName, metav1.GetOptions{})
	if err != nil {
		return ""
	}
	const karmadaContextPrefix = "--karmada-context="
	for _, cmd := range agent.Spec.Template.Spec.Containers[0].Command {
		if strings.HasPrefix(cmd, karmadaContextPrefix) {
			return cmd[len(karmadaContextPrefix):]
		}
	}
	return ""
}

// deleteDeployment just try to delete the Deployment.
func deleteDeployment(client kubeclient.Interface, namespace, name string) error {
	err := client.AppsV1().Deployments(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
