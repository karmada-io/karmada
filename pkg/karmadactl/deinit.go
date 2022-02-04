package karmadactl

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

// LabelSelector karmada bootstrapping label
const LabelSelector = "karmada.io/bootstrapping"

// CommandDeInitOption options for deinit.
type CommandDeInitOption struct {
	Recursive       bool
	Force           bool
	KarmadaDataPath string
	Namespace       string
	KubeConfig      string
	KubeClientSet   *kubernetes.Clientset
}

// NewCmdDeInit remove karmada on kubernetes
func NewCmdDeInit(cmdOut io.Writer, parentCommand string) *cobra.Command {
	opts := CommandDeInitOption{}
	cmd := &cobra.Command{
		Use:          "deinit",
		Short:        "remove karmada on kubernetes",
		Long:         "remove karmada on kubernetes",
		Example:      deInitExample(parentCommand),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Complete(); err != nil {
				return err
			}
			if err := opts.RunDeInit(); err != nil {
				return err
			}
			return nil
		},
	}
	flags := cmd.PersistentFlags()
	flags.StringVarP(&opts.Namespace, "namespace", "n", "karmada-system", "Kubernetes namespace")
	flags.StringVar(&opts.KubeConfig, "kubeconfig", filepath.Join(homeDir(), ".kube", "config"), "absolute path to the kubeconfig file")
	flags.StringVarP(&opts.KarmadaDataPath, "karmada-data", "d", "/etc/karmada", "karmada data path. kubeconfig cert and crds files")
	flags.BoolVarP(&opts.Force, "force", "f", false, "skip delete confirmation")
	flags.BoolVarP(&opts.Recursive, "recursive", "r", false, "delete recursively, delete all resources in the namespace, including persistent storage.")
	return cmd
}
func deInitExample(parentCommand string) string {
	return fmt.Sprintf(`
# Remove Karmada in Kubernetes cluster.
%s deinit

# Skip the prompt and just delete Karmada.
%s deinit --force

# Recursively delete, delete the namespace and all resources under the namespace.
%s deinit --recursive
`, parentCommand, parentCommand, parentCommand)
}

// Complete the conditions required to be able to run deinit.
func (d *CommandDeInitOption) Complete() error {
	restConfig, err := utils.RestConfig(d.KubeConfig)
	if err != nil {
		return err
	}
	fmt.Printf("kubeconfig file: %s, kubernetes: %s\n", d.KubeConfig, restConfig.Host)
	clientSet, err := utils.NewClientSet(restConfig)
	if err != nil {
		return err
	}
	d.KubeClientSet = clientSet
	if _, err := d.KubeClientSet.CoreV1().Namespaces().Get(context.TODO(), d.Namespace, metav1.GetOptions{}); err != nil {
		return err
	}

	return nil
}

// Validate Check that there are enough flags to run the command.
func (d *CommandDeInitOption) Validate() error {
	if !isExist(d.KubeConfig) {
		return fmt.Errorf("%q file does not exist.absolute path to the kubeconfig file", d.KubeConfig)
	}

	return nil
}

func isExist(fileName string) bool {
	_, err := os.Stat(fileName)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// DeleteDeployments Delete deployment by label LabelSelector
func (d *CommandDeInitOption) DeleteDeployments() error {
	deploymentClient := d.KubeClientSet.AppsV1().Deployments(d.Namespace)
	deployments, err := deploymentClient.List(context.TODO(), metav1.ListOptions{LabelSelector: LabelSelector})
	if err != nil {
		return err
	}
	if len(deployments.Items) == 0 {
		fmt.Printf("Deployment not found by label %q\n", LabelSelector)
		return nil
	}

	for _, deployment := range deployments.Items {
		fmt.Printf("delete deployment %q\n", deployment.Name)
		if err := deploymentClient.Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// DeleteStatefulSets Delete StatefulSet by label LabelSelector
func (d *CommandDeInitOption) DeleteStatefulSets() error {
	statefulSetClient := d.KubeClientSet.AppsV1().StatefulSets(d.Namespace)
	statefulSets, err := statefulSetClient.List(context.TODO(), metav1.ListOptions{LabelSelector: LabelSelector})
	if err != nil {
		return err
	}
	if len(statefulSets.Items) == 0 {
		fmt.Printf("StatefulSet not found by label %q\n", LabelSelector)
		return nil
	}

	for _, statefulSet := range statefulSets.Items {
		fmt.Printf("delete StatefulSet %q\n", statefulSet.Name)
		if err := statefulSetClient.Delete(context.TODO(), statefulSet.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// DeleteServices Delete Service by label LabelSelector
func (d *CommandDeInitOption) DeleteServices() error {
	serviceClient := d.KubeClientSet.CoreV1().Services(d.Namespace)
	services, err := serviceClient.List(context.TODO(), metav1.ListOptions{LabelSelector: LabelSelector})
	if err != nil {
		return err
	}
	if len(services.Items) == 0 {
		fmt.Printf("Service not found by label %q\n", LabelSelector)
		return nil
	}

	for _, service := range services.Items {
		fmt.Printf("delete Service %q\n", service.Name)
		if err := serviceClient.Delete(context.TODO(), service.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// DeleteSecrets Delete Secret by label LabelSelector
func (d *CommandDeInitOption) DeleteSecrets() error {
	secretClient := d.KubeClientSet.CoreV1().Secrets(d.Namespace)
	secrets, err := secretClient.List(context.TODO(), metav1.ListOptions{LabelSelector: LabelSelector})
	if err != nil {
		return err
	}
	if len(secrets.Items) == 0 {
		fmt.Printf("Secrets not found by label %q\n", LabelSelector)
		return nil
	}

	for _, secret := range secrets.Items {
		fmt.Printf("delete Secrets %q\n", secret.Name)
		if err := secretClient.Delete(context.TODO(), secret.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// DeleteClusterRole Delete ClusterRole by label LabelSelector
func (d *CommandDeInitOption) DeleteClusterRole() error {
	clusterRoleClient := d.KubeClientSet.RbacV1().ClusterRoles()
	clusterRoles, err := clusterRoleClient.List(context.TODO(), metav1.ListOptions{LabelSelector: LabelSelector})
	if err != nil {
		return err
	}
	if len(clusterRoles.Items) == 0 {
		fmt.Printf("ClusterRole not found by label %q\n", LabelSelector)
		return nil
	}

	for _, service := range clusterRoles.Items {
		fmt.Printf("delete ClusterRole %q\n", service.Name)
		if err := clusterRoleClient.Delete(context.TODO(), service.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// RemoveNodeLabels removes labels from node which were created by karmadactl init
func (d *CommandDeInitOption) RemoveNodeLabels() error {
	karmadaNodeLabel := "karmada.io/etcd"
	nodeClient := d.KubeClientSet.CoreV1().Nodes()
	nodes, err := nodeClient.List(context.TODO(), metav1.ListOptions{LabelSelector: karmadaNodeLabel})
	if err != nil {
		return err
	}
	if len(nodes.Items) == 0 {
		fmt.Printf("node not found by label %q\n", karmadaNodeLabel)
		return nil
	}

	for v := range nodes.Items {
		removeLabels(&nodes.Items[v], karmadaNodeLabel)
		fmt.Printf("remove node %q labels %q\n", nodes.Items[v].Name, karmadaNodeLabel)
		if _, err := nodeClient.Update(context.TODO(), &nodes.Items[v], metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func removeLabels(node *corev1.Node, removesLabel string) {
	for label := range node.Labels {
		if strings.Contains(label, removesLabel) {
			delete(node.Labels, label)
		}
	}
}

// RenameKarmadaDataPath backup karmada directory
func (d *CommandDeInitOption) RenameKarmadaDataPath() error {
	if !isExist(d.KarmadaDataPath) {
		fmt.Printf("WARNING %q file does not exist.\n", d.KarmadaDataPath)
		return nil
	}
	timeStr := time.Now().Format("20060102150405")
	backupName := fmt.Sprintf("%s-%s", d.KarmadaDataPath, timeStr)
	if err := os.Rename(d.KarmadaDataPath, backupName); err != nil {
		return err
	}
	return nil
}

// DeleteNamespace Delete namespace
// TODO k8s will recursively delete all resources under namespace,including etcd persistent storage.
func (d *CommandDeInitOption) DeleteNamespace() error {
	fmt.Printf("Delete namespace %q\n", d.Namespace)
	if err := d.KubeClientSet.CoreV1().Namespaces().Delete(context.TODO(), d.Namespace, metav1.DeleteOptions{}); err != nil {
		return err
	}
	return nil
}

// deleteConfirmation delete confirmation karmada
func deleteConfirmation() bool {
	fmt.Println("Please type (y)es or (n)o and then press enter:")
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	switch strings.ToLower(response) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return deleteConfirmation()
	}
}

// RunDeInit start delete action
func (d *CommandDeInitOption) RunDeInit() error {
	// delete confirmation,exit the delete action when false.
	fmt.Println("remove karmada on kubernetes, please make data backup.")
	if !d.Force {
		d.Force = deleteConfirmation()
	}
	if !d.Force {
		os.Exit(0)
	}

	if err := d.DeleteDeployments(); err != nil {
		return err
	}

	if err := d.DeleteStatefulSets(); err != nil {
		return err
	}

	if err := d.DeleteServices(); err != nil {
		return err
	}

	if err := d.DeleteSecrets(); err != nil {
		return err
	}

	if err := d.DeleteClusterRole(); err != nil {
		return err
	}

	if err := d.RemoveNodeLabels(); err != nil {
		return err
	}

	if err := d.RenameKarmadaDataPath(); err != nil {
		return err
	}

	if d.Recursive {
		if err := d.DeleteNamespace(); err != nil {
			return err
		}
	}
	fmt.Println("Remove karmada from kubernetes successfully.")
	return nil
}
