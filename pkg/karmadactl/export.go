package karmadactl

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// DefaultServiceNamespace defines the namespace specify by default when users export/import service.
const DefaultServiceNamespace = "default"

var (
	exportShort   = `Export a service from a group member clusters`
	exportLong    = `Export a service from a group member clusters`
	exportExample = `
%s export SERVICE_NAME --namespace=default --clusters="m1,m2"
`
)

// CommandExportOption holds all command options
type CommandExportOption struct {
	options.GlobalCommandOptions

	// ServiceName is the service's name that we are going to export.
	ServiceName string

	// ServiceNamespace is the service's namespace that we are going to export.
	// Default value is "default".
	ServiceNamespace string

	// Clusters are a group member clusters that used for service export.
	Clusters []string
}

// Complete ensures that options are valid and marshals them if necessary.
func (o *CommandExportOption) Complete(args []string) error {
	if len(args) == 0 {
		return errors.New("service name is required")
	}
	o.ServiceName = args[0]
	return nil
}

// Validate checks option and return a slice of found errs.
func (o *CommandExportOption) Validate() []error {
	return nil
}

// AddFlags adds flags to the specified FlagSet.
func (o *CommandExportOption) AddFlags(flags *pflag.FlagSet) {
	o.GlobalCommandOptions.AddFlags(flags)

	flags.StringVarP(&o.ServiceNamespace, "namespace", "n", DefaultServiceNamespace, "Namespace of service that are going to export")
	flags.StringSliceVar(&o.Clusters, "clusters", []string{}, "clusters are a group member clusters that used for service export")
}

// NewCmdExport defines the `export` command that export a service.
func NewCmdExport(cmdOut io.Writer, karmadaConfig KarmadaConfig, cmdStr string) *cobra.Command {
	opts := CommandExportOption{}

	cmd := &cobra.Command{
		Use:     "export SERVICE_NAME --namespace=<NAMESPACE> --clusters=<CLUSTERS>",
		Short:   exportShort,
		Long:    exportLong,
		Example: fmt.Sprintf(exportExample, cmdStr),
		Run: func(cmd *cobra.Command, args []string) {
			err := opts.Complete(args)
			if err != nil {
				klog.Errorf("Error: %v", err)
				return
			}

			errs := opts.Validate()
			if len(errs) != 0 {
				klog.Error(utilerrors.NewAggregate(errs).Error())
				return
			}

			err = RunExport(cmdOut, karmadaConfig, opts)
			if err != nil {
				klog.Errorf("Error: %v", err)
				return
			}
		},
	}

	flags := cmd.Flags()
	opts.AddFlags(flags)

	return cmd
}

// RunExport is the implementation of the 'export' command.
func RunExport(cmdOut io.Writer, karmadaConfig KarmadaConfig, opts CommandExportOption) error {
	if len(opts.Clusters) == 0 {
		klog.Infof("There are no clusters to export service.")
		return nil
	}

	// Get control plane karmada-apiserver client
	controlPlaneRestConfig, err := karmadaConfig.GetRestConfig(opts.KarmadaContext, opts.KubeConfig)
	if err != nil {
		klog.Errorf("Failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			opts.KarmadaContext, opts.KubeConfig, err)
		return err
	}
	karmadaClient := gclient.NewForConfigOrDie(controlPlaneRestConfig)

	svcNamespacedName := types.NamespacedName{Namespace: opts.ServiceNamespace, Name: opts.ServiceName}
	svcExport := helper.NewServiceExport(opts.ServiceNamespace, opts.ServiceName)
	err = createServiceExportIfNotFound(karmadaClient, svcNamespacedName, svcExport)
	if err != nil {
		klog.Errorf("Failed to create ServiceExport if not found: %v", err)
		return err
	}

	policyNamespacedName := svcNamespacedName
	policyExport := helper.NewPropagationPolicy(opts.ServiceNamespace, opts.ServiceName, []policyv1alpha1.ResourceSelector{
		{
			APIVersion: svcExport.APIVersion,
			Kind:       svcExport.Kind,
			Name:       svcExport.Name,
			Namespace:  svcExport.Namespace,
		},
	}, policyv1alpha1.Placement{
		ClusterAffinity: &policyv1alpha1.ClusterAffinity{
			ClusterNames: opts.Clusters,
		},
	})
	err = createOrUpdateExportPolicy(karmadaClient, policyNamespacedName, policyExport)
	if err != nil {
		klog.Errorf("Failed to create or update export policy: %v", err)
	}

	return nil
}

func createServiceExportIfNotFound(karmadaClient client.Client, svcKey types.NamespacedName, svcExport *mcsv1alpha1.ServiceExport) error {
	oldSvcExport := &mcsv1alpha1.ServiceExport{}
	err := karmadaClient.Get(context.TODO(), svcKey, oldSvcExport)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return karmadaClient.Create(context.TODO(), svcExport)
		}

		klog.Errorf("Failed to get Service(%s): %v", svcKey.String(), err)
		return err
	}

	// ServiceExport already exist, just update PropagationPolicy
	return nil
}

func createOrUpdateExportPolicy(karmadaClient client.Client, policyKey types.NamespacedName, policyExport *policyv1alpha1.PropagationPolicy) error {
	oldPolicy := &policyv1alpha1.PropagationPolicy{}
	err := karmadaClient.Get(context.TODO(), policyKey, oldPolicy)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return karmadaClient.Create(context.TODO(), policyExport)
		}

		klog.Errorf("Failed to get PropagationPolicy(%s): %v", policyKey.String(), err)
		return err
	}

	mergeClusterNames := mergeClusterAffinityClusterNames(oldPolicy.Spec.Placement.ClusterAffinity.ClusterNames, policyExport.Spec.Placement.ClusterAffinity.ClusterNames)
	if len(oldPolicy.Spec.Placement.ClusterAffinity.ClusterNames) == len(mergeClusterNames) {
		klog.V(4).Infof("There are no new clusterName added.")
		return nil
	}

	oldPolicy.Spec.Placement.ClusterAffinity.ClusterNames = mergeClusterNames
	return karmadaClient.Update(context.TODO(), oldPolicy)
}

func mergeClusterAffinityClusterNames(oldClusterNames, newClusterNames []string) []string {
	clusterNames := make([]string, len(oldClusterNames))
	copy(clusterNames, oldClusterNames)

	for _, newCluster := range newClusterNames {
		exist := false
		for _, oldCluster := range oldClusterNames {
			if newCluster == oldCluster {
				exist = true
				break
			}
		}
		if !exist {
			clusterNames = append(clusterNames, newCluster)
		}
	}

	return clusterNames
}
