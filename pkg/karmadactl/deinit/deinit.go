package deinit

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
)

const (
	karmadaBootstrappingLabelKey = "karmada.io/bootstrapping"
	karmadaNodeLabel             = "karmada.io/etcd"
)

var (
	deInitExample = templates.Examples(`
		# Remove Karmada from the Kubernetes cluster.
		%[1]s deinit`)
)

// CommandDeInitOption options for deinit.
type CommandDeInitOption struct {
	// KubeConfig holds host cluster KUBECONFIG file path.
	KubeConfig string
	Context    string
	Namespace  string

	// DryRun tells if run the command in dry-run mode, without making any server requests.
	DryRun bool
	Force  bool

	KubeClientSet *kubernetes.Clientset
}

// NewCmdDeInit removes Karmada from Kubernetes
func NewCmdDeInit(parentCommand string) *cobra.Command {
	opts := CommandDeInitOption{}
	cmd := &cobra.Command{
		Use:                   "deinit",
		Short:                 "Removes Karmada from Kubernetes",
		Long:                  "Removes Karmada from Kubernetes",
		Example:               fmt.Sprintf(deInitExample, parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(); err != nil {
				return err
			}
			if err := opts.Run(); err != nil {
				return err
			}
			return nil
		},
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterRegistration,
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Namespace, "namespace", "n", "karmada-system", "namespace where Karmada components are installed.")
	flags.StringVar(&opts.KubeConfig, "kubeconfig", "", "Path to the host cluster kubeconfig file.")
	flags.StringVar(&opts.Context, "context", "", "The name of the kubeconfig context to use")
	flags.BoolVar(&opts.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
	flags.BoolVarP(&opts.Force, "force", "f", false, "Reset cluster without prompting for confirmation.")
	return cmd
}

// Complete the conditions required to be able to run deinit.
func (o *CommandDeInitOption) Complete() error {
	restConfig, err := apiclient.RestConfig(o.Context, o.KubeConfig)
	if err != nil {
		return err
	}

	o.KubeClientSet, err = apiclient.NewClientSet(restConfig)
	if err != nil {
		return err
	}

	if _, err := o.KubeClientSet.CoreV1().Namespaces().Get(context.TODO(), o.Namespace, metav1.GetOptions{}); err != nil {
		return err
	}

	return nil
}

// delete removes Karmada from Kubernetes
func (o *CommandDeInitOption) delete() error {
	if err := o.deleteWorkload(); err != nil {
		return err
	}

	// Delete Service by karmadaBootstrappingLabelKey
	serviceClient := o.KubeClientSet.CoreV1().Services(o.Namespace)
	services, err := serviceClient.List(context.TODO(), metav1.ListOptions{LabelSelector: karmadaBootstrappingLabelKey})
	if err != nil {
		return err
	}

	for _, service := range services.Items {
		fmt.Printf("delete Service %q\n", service.Name)
		if o.DryRun {
			continue
		}
		if err := serviceClient.Delete(context.TODO(), service.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	// Delete Secret by karmadaBootstrappingLabelKey
	secretClient := o.KubeClientSet.CoreV1().Secrets(o.Namespace)
	secrets, err := secretClient.List(context.TODO(), metav1.ListOptions{LabelSelector: karmadaBootstrappingLabelKey})
	if err != nil {
		return err
	}
	for _, secret := range secrets.Items {
		fmt.Printf("delete Secrets %q\n", secret.Name)
		if o.DryRun {
			continue
		}
		if err := secretClient.Delete(context.TODO(), secret.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	// Delete namespace where Karmada components are installed
	fmt.Printf("delete Namespace %q\n", o.Namespace)
	if o.DryRun {
		return nil
	}
	if err = o.KubeClientSet.CoreV1().Namespaces().Delete(context.Background(), o.Namespace, metav1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

func (o *CommandDeInitOption) deleteWorkload() error {
	// Delete deployment by karmadaBootstrappingLabelKey
	deploymentClient := o.KubeClientSet.AppsV1().Deployments(o.Namespace)
	deployments, err := deploymentClient.List(context.TODO(), metav1.ListOptions{LabelSelector: karmadaBootstrappingLabelKey})
	if err != nil {
		return err
	}
	for _, deployment := range deployments.Items {
		fmt.Printf("delete deployment %q\n", deployment.Name)
		if o.DryRun {
			continue
		}
		if err := deploymentClient.Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	// Delete StatefulSet by label LabelSelector
	statefulSetClient := o.KubeClientSet.AppsV1().StatefulSets(o.Namespace)
	statefulSets, err := statefulSetClient.List(context.TODO(), metav1.ListOptions{LabelSelector: karmadaBootstrappingLabelKey})
	if err != nil {
		return err
	}

	for _, statefulSet := range statefulSets.Items {
		fmt.Printf("delete StatefulSet: %q\n", statefulSet.Name)
		if o.DryRun {
			continue
		}
		if err := statefulSetClient.Delete(context.TODO(), statefulSet.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// removeNodeLabels removes labels from node which were created by karmadactl init
func (o *CommandDeInitOption) removeNodeLabels() error {
	nodeClient := o.KubeClientSet.CoreV1().Nodes()
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
		if o.DryRun {
			continue
		}
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

// deleteConfirmation delete karmada confirmation
func deleteConfirmation() bool {
	fmt.Print("Please type (y)es or (n)o and then press enter:")
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

// Run start delete
func (o *CommandDeInitOption) Run() error {
	fmt.Println("removes Karmada from Kubernetes")
	// delete confirmation,exit the delete action when false.
	if !o.Force && !deleteConfirmation() {
		return nil
	}

	if err := o.delete(); err != nil {
		return err
	}

	if err := o.removeNodeLabels(); err != nil {
		return err
	}

	fmt.Println("remove Karmada from Kubernetes successfully.\n" +
		"\ndeinit will not delete etcd data, if the etcd data is persistent, please delete it yourself." +
		"\nthe default persistence path for etcd data is '/var/lib/karmada-etcd'")
	return nil
}
