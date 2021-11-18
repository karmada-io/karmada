package karmadactl

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

const printColumnClusterNum = 1

var (
	getIn  = os.Stdin
	getOut = os.Stdout
	getErr = os.Stderr

	podColumns = []metav1.TableColumnDefinition{
		{Name: "Cluster", Type: "string", Format: "", Priority: 0},
		{Name: "ADOPTION", Type: "string", Format: "", Priority: 0},
	}

	noPushModeMessage = "The karmadactl get command now only supports Push mode, [ %s ] are not push mode\n"
	getShort          = `Display one or many resources`
	defaultKubeConfig = filepath.Join(os.Getenv("HOME"), ".kube/config")
)

// NewCmdGet New get command
func NewCmdGet(out io.Writer, karmadaConfig KarmadaConfig, parentCommand string) *cobra.Command {
	ioStreams := genericclioptions.IOStreams{In: getIn, Out: getOut, ErrOut: getErr}
	o := NewCommandGetOptions("karmadactl", ioStreams)
	cmd := &cobra.Command{
		Use:                   "get [NAME | -l label | -n namespace]  [flags]",
		DisableFlagsInUseLine: true,
		Short:                 getShort,
		SilenceUsage:          true,
		Example:               getExample(parentCommand),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(cmd, args); err != nil {
				return err
			}
			if err := o.Run(karmadaConfig, cmd, args); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "default", "-n=namespace or -n namespace")
	cmd.Flags().StringVarP(&o.LabelSelector, "labels", "l", "", "-l=label or -l label")
	cmd.Flags().StringSliceVarP(&o.Clusters, "clusters", "C", []string{}, "-C=member1,member2")
	o.GlobalCommandOptions.AddFlags(cmd.Flags())
	return cmd
}

// CommandGetOptions contains the input to the get command.
type CommandGetOptions struct {
	// global flags
	options.GlobalCommandOptions

	Clusters []string

	PrintFlags             *get.PrintFlags
	ToPrinter              func(*meta.RESTMapping, *bool, bool, bool) (printers.ResourcePrinterFunc, error)
	IsHumanReadablePrinter bool
	PrintWithOpenAPICols   bool

	CmdParent string

	resource.FilenameOptions

	Raw       string
	ChunkSize int64

	OutputWatchEvents bool

	LabelSelector     string
	FieldSelector     string
	AllNamespaces     bool
	Namespace         string
	ExplicitNamespace bool

	ServerPrint bool

	NoHeaders      bool
	Sort           bool
	IgnoreNotFound bool
	Export         bool

	genericclioptions.IOStreams
}

// NewCommandGetOptions returns a GetOptions with default chunk size 500.
func NewCommandGetOptions(parent string, streams genericclioptions.IOStreams) *CommandGetOptions {
	return &CommandGetOptions{
		PrintFlags: get.NewGetPrintFlags(),

		CmdParent: parent,

		IOStreams:   streams,
		ChunkSize:   500,
		ServerPrint: true,
	}
}

// Complete takes the command arguments and infers any remaining options.
func (g *CommandGetOptions) Complete(cmd *cobra.Command, args []string) error {
	newScheme := gclient.NewSchema()
	// human readable printers have special conversion rules, so we determine if we're using one.
	g.IsHumanReadablePrinter = true

	// check karmada config path
	env := os.Getenv("KUBECONFIG")
	if env != "" {
		g.KubeConfig = env
	}

	if g.KubeConfig == "" {
		g.KubeConfig = defaultKubeConfig
	}
	if !Exists(g.KubeConfig) {
		return ErrEmptyConfig
	}

	g.ToPrinter = func(mapping *meta.RESTMapping, outputObjects *bool, withNamespace bool, withKind bool) (printers.ResourcePrinterFunc, error) {
		// make a new copy of current flags / opts before mutating
		printFlags := g.PrintFlags.Copy()

		if withNamespace {
			_ = printFlags.EnsureWithNamespace()
		}
		if withKind {
			_ = printFlags.EnsureWithKind()
		}

		printer, err := printFlags.ToPrinter()
		if err != nil {
			return nil, err
		}
		printer, err = printers.NewTypeSetter(newScheme).WrapToPrinter(printer, nil)
		if err != nil {
			return nil, err
		}

		if g.ServerPrint {
			printer = &get.TablePrinter{Delegate: printer}
		}
		return printer.PrintObj, nil
	}
	return nil
}

// Obj cluster info
type Obj struct {
	Cluster string
	Infos   runtime.Object
	Mapping *meta.RESTMapping
}

// RBInfo resourcebinding info and print info
var RBInfo map[string]*OtherPrint

// OtherPrint applied is used in the display column
type OtherPrint struct {
	Applied interface{}
}

// Run performs the get operation.
func (g *CommandGetOptions) Run(karmadaConfig KarmadaConfig, cmd *cobra.Command, args []string) error {
	mux := sync.Mutex{}
	var wg sync.WaitGroup

	var objs []Obj
	var allErrs []error
	errs := sets.NewString()

	clusterInfos := make(map[string]*ClusterInfo)
	RBInfo = make(map[string]*OtherPrint)

	karmadaclient, err := clusterInfoInit(g, karmadaConfig, clusterInfos)
	if err != nil {
		return err
	}

	var noPushModeCluster []string
	wg.Add(len(g.Clusters))
	for idx := range g.Clusters {
		if clusterInfos[g.Clusters[idx]].ClusterSyncMode != clusterv1alpha1.Push {
			noPushModeCluster = append(noPushModeCluster, g.Clusters[idx])
			wg.Done()
			continue
		}

		if err := g.getSecretTokenInKarmada(karmadaclient, g.Clusters[idx], clusterInfos); err != nil {
			return fmt.Errorf("Method getSecretTokenInKarmada get Secret info in karmada failed, err is: %w", err)
		}
		f := getFactory(g.Clusters[idx], clusterInfos)
		go g.getObjInfo(&wg, &mux, f, g.Clusters[idx], &objs, &allErrs, args)
	}
	wg.Wait()
	if len(noPushModeCluster) != 0 {
		fmt.Println(fmt.Sprintf(noPushModeMessage, strings.Join(noPushModeCluster, ",")))
	}

	table := &metav1.Table{}
	allTableRows, mapping, err := g.reconstructionRow(objs, table)
	if err != nil {
		return err
	}
	table.Rows = allTableRows

	setNoAdoption(mapping)
	setColumnDefinition(table)

	if len(table.Rows) == 0 {
		msg := fmt.Sprintf("%v from server (NotFound)", args)
		fmt.Println(msg)
		return nil
	}
	printObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(table)
	if err != nil {
		return err
	}
	newPrintObj := &unstructured.Unstructured{Object: printObj}

	var printer printers.ResourcePrinter
	var lastMapping *meta.RESTMapping

	// track if we write any output
	trackingWriter := &trackingWriterWrapper{Delegate: g.Out}
	// output an empty line separating output
	separatorWriter := &separatorWriterWrapper{Delegate: trackingWriter}

	w := printers.GetNewTabWriter(separatorWriter)
	if shouldGetNewPrinterForMapping(printer, lastMapping, mapping) {
		w.Flush()
		w.SetRememberedWidths(nil)

		// add linebreaks between resource groups (if there is more than one)
		// when it satisfies all following 3 conditions:
		// 1) it's not the first resource group
		// 2) it has row header
		// 3) we've written output since the last time we started a new set of headers
		if !g.NoHeaders && trackingWriter.Written > 0 {
			separatorWriter.SetReady(true)
		}

		printer, err = g.ToPrinter(mapping, nil, false, false)
		if err != nil {
			if !errs.Has(err.Error()) {
				errs.Insert(err.Error())
				allErrs = append(allErrs, err)
			}
			return err
		}
		//lastMapping = mapping
	}
	err = printer.PrintObj(newPrintObj, w)
	if err != nil {
		return err
	}
	w.Flush()

	return utilerrors.NewAggregate(allErrs)
}

// getObjInfo get obj info in member cluster
func (g *CommandGetOptions) getObjInfo(wg *sync.WaitGroup, mux *sync.Mutex, f cmdutil.Factory,
	cluster string, objs *[]Obj, allErrs *[]error, args []string) {
	defer wg.Done()
	chunkSize := g.ChunkSize
	r := f.NewBuilder().
		Unstructured().
		NamespaceParam(g.Namespace).DefaultNamespace().AllNamespaces(g.AllNamespaces).
		FilenameParam(g.ExplicitNamespace, &g.FilenameOptions).
		LabelSelectorParam(g.LabelSelector).
		FieldSelectorParam(g.FieldSelector).
		RequestChunksOf(chunkSize).
		ResourceTypeOrNameArgs(true, args...).
		ContinueOnError().
		Latest().
		Flatten().
		TransformRequests(g.transformRequests).
		Do()

	r.IgnoreErrors(apierrors.IsNotFound)

	infos, err := r.Infos()
	if err != nil {
		*allErrs = append(*allErrs, err)
	}
	mux.Lock()
	var objInfo Obj
	for ix := range infos {
		objInfo = Obj{
			Cluster: cluster,
			Infos:   infos[ix].Object,
			Mapping: infos[ix].Mapping,
		}
		*objs = append(*objs, objInfo)
	}
	mux.Unlock()
}

// reconstructionRow reconstruction tableRow
func (g *CommandGetOptions) reconstructionRow(objs []Obj, table *metav1.Table) ([]metav1.TableRow, *meta.RESTMapping, error) {
	var allTableRows []metav1.TableRow
	var mapping *meta.RESTMapping
	for ix := range objs {
		mapping = objs[ix].Mapping
		unstr, ok := objs[ix].Infos.(*unstructured.Unstructured)
		if !ok {
			return nil, nil, fmt.Errorf("attempt to decode non-Unstructured object")
		}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.Object, table); err != nil {
			return nil, nil, err
		}
		for rowIdx := range table.Rows {
			var tempRow metav1.TableRow
			rbKey := getRBKey(mapping.Resource, table.Rows[rowIdx], objs[ix].Cluster)
			tempRow.Cells = append(append(tempRow.Cells, table.Rows[rowIdx].Cells[0], objs[ix].Cluster), table.Rows[rowIdx].Cells[1:]...)
			if _, ok := RBInfo[rbKey]; ok {
				tempRow.Cells = append(tempRow.Cells, "Y")
			} else {
				tempRow.Cells = append(tempRow.Cells, "N")
			}
			table.Rows[rowIdx].Cells = tempRow.Cells
		}
		allTableRows = append(allTableRows, table.Rows...)
	}
	return allTableRows, mapping, nil
}

type trackingWriterWrapper struct {
	Delegate io.Writer
	Written  int
}

func (t *trackingWriterWrapper) Write(p []byte) (n int, err error) {
	t.Written += len(p)
	return t.Delegate.Write(p)
}

type separatorWriterWrapper struct {
	Delegate io.Writer
	Ready    bool
}

func (s *separatorWriterWrapper) Write(p []byte) (n int, err error) {
	// If we're about to write non-empty bytes and `s` is ready,
	// we prepend an empty line to `p` and reset `s.Read`.
	if len(p) != 0 && s.Ready {
		fmt.Fprintln(s.Delegate)
		s.Ready = false
	}
	return s.Delegate.Write(p)
}

func (s *separatorWriterWrapper) SetReady(state bool) {
	s.Ready = state
}

func shouldGetNewPrinterForMapping(printer printers.ResourcePrinter, lastMapping, mapping *meta.RESTMapping) bool {
	return printer == nil || lastMapping == nil || mapping == nil || mapping.Resource != lastMapping.Resource
}

// ClusterInfo Information about the member in the karmada cluster.
type ClusterInfo struct {
	APIEndpoint     string
	BearerToken     string
	CAData          string
	ClusterSyncMode clusterv1alpha1.ClusterSyncMode
}

func clusterInfoInit(g *CommandGetOptions, karmadaConfig KarmadaConfig, clusterInfos map[string]*ClusterInfo) (*rest.Config, error) {
	karmadaclient, err := karmadaConfig.GetRestConfig(g.KarmadaContext, g.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("Func GetRestConfig get karmada client failed, err is: %w", err)
	}

	if err := getClusterInKarmada(karmadaclient, clusterInfos); err != nil {
		return nil, fmt.Errorf("Method getClusterInKarmada get cluster info in karmada failed, err is: %w", err)
	}

	if err := getRBInKarmada(g.Namespace, karmadaclient); err != nil {
		return nil, err
	}

	if len(g.Clusters) <= 0 {
		for c := range clusterInfos {
			g.Clusters = append(g.Clusters, c)
		}
	}
	return karmadaclient, nil
}

func getFactory(clusterName string, clusterInfos map[string]*ClusterInfo) cmdutil.Factory {
	kubeConfigFlags := NewConfigFlags(true).WithDeprecatedPasswordFlag()
	// Build member cluster kubeConfigFlags
	kubeConfigFlags.BearerToken = stringptr(clusterInfos[clusterName].BearerToken)
	kubeConfigFlags.APIServer = stringptr(clusterInfos[clusterName].APIEndpoint)
	kubeConfigFlags.CaBundle = stringptr(clusterInfos[clusterName].CAData)
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	return cmdutil.NewFactory(matchVersionKubeConfigFlags)
}

func (g *CommandGetOptions) transformRequests(req *rest.Request) {
	// We need full objects if printing with openapi columns
	if g.PrintWithOpenAPICols {
		return
	}
	if !g.ServerPrint || !g.IsHumanReadablePrinter {
		return
	}

	req.SetHeader("Accept", strings.Join([]string{
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1.SchemeGroupVersion.Version, metav1.GroupName),
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1beta1.SchemeGroupVersion.Version, metav1beta1.GroupName),
		"application/json",
	}, ","))
}

func getRBInKarmada(namespace string, config *rest.Config) error {
	resourceList := &workv1alpha2.ResourceBindingList{}
	gClient, err := gclient.NewForConfig(config)
	if err != nil {
		return err
	}
	if err = gClient.List(context.TODO(), resourceList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			policyv1alpha1.PropagationPolicyNamespaceLabel: namespace,
		})}); err != nil {
		return err
	}

	for idx := range resourceList.Items {
		rbKey := resourceList.Items[idx].GetName()
		val := resourceList.Items[idx].Status.AggregatedStatus
		for i := range val {
			if val[i].Applied && val[i].ClusterName != "" {
				newRBKey := fmt.Sprintf("%s-%s", val[i].ClusterName, rbKey)
				RBInfo[newRBKey] = &OtherPrint{
					Applied: val[i].Applied,
				}
			}
		}
	}
	return nil
}

// getSecretTokenInKarmada get token ca in karmada cluster
func (g *CommandGetOptions) getSecretTokenInKarmada(client *rest.Config, name string, clusterInfos map[string]*ClusterInfo) error {
	clusterClient, err := kubernetes.NewForConfig(client)
	if err != nil {
		return err
	}
	secret, err := clusterClient.CoreV1().Secrets(g.ClusterNamespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	clusterInfos[name].BearerToken = string(secret.Data[clusterv1alpha1.SecretTokenKey])
	clusterInfos[name].CAData = string(secret.Data[clusterv1alpha1.SecretCADataKey])
	return nil
}

// getClusterInKarmada get cluster info in karmada cluster
func getClusterInKarmada(client *rest.Config, clusterInfos map[string]*ClusterInfo) error {
	clusterList := &clusterv1alpha1.ClusterList{}
	gClient, err := gclient.NewForConfig(client)
	if err != nil {
		return err
	}
	if err = gClient.List(context.TODO(), clusterList); err != nil {
		return err
	}

	for i := range clusterList.Items {
		cluster := &ClusterInfo{
			APIEndpoint:     clusterList.Items[i].Spec.APIEndpoint,
			ClusterSyncMode: clusterList.Items[i].Spec.SyncMode,
		}
		clusterInfos[clusterList.Items[i].GetName()] = cluster
	}
	return nil
}

func getRBKey(groupResource schema.GroupVersionResource, row metav1.TableRow, cluster string) string {
	rbKey, _ := row.Cells[0].(string)
	var suffix string
	switch groupResource.Resource {
	case "deployments":
		suffix = "deployment"
	case "services":
		suffix = "service"
	case "daemonsets":
		suffix = "daemonset"
	default:
		suffix = groupResource.Resource
	}
	return fmt.Sprintf("%s-%s-%s", cluster, rbKey, suffix)
}

// setNoAdoption set pod no print adoption
func setNoAdoption(mapping *meta.RESTMapping) {
	if mapping != nil && mapping.Resource.Resource == "pods" {
		podColumns[printColumnClusterNum].Priority = 1
	}
}

// setColumnDefinition set print ColumnDefinition
func setColumnDefinition(table *metav1.Table) {
	var tempColumnDefinition []metav1.TableColumnDefinition
	if len(table.ColumnDefinitions) > 0 {
		tempColumnDefinition = append(append(append(tempColumnDefinition, table.ColumnDefinitions[0], podColumns[0]), table.ColumnDefinitions[1:]...), podColumns[1:]...)
		table.ColumnDefinitions = tempColumnDefinition
	}
}

// Exists determine if path exists
func Exists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return os.IsExist(err)
	}
	return true
}

// getExample get examples by cmd type
func getExample(parentCommand string) string {
	example := `
# List all pods in ps output format` + "\n" +
		fmt.Sprintf("%s get pods", parentCommand) + `

# List a single replicasets controller with specified NAME in ps output format ` + "\n" +
		fmt.Sprintf("%s get replicasets nginx", parentCommand) + `

# List deployments in JSON output format, in the "v1" version of the "apps" API group ` + "\n" +
		fmt.Sprintf("%s get deployments.v1.apps ", parentCommand) + `

# List all replication controllers and services together in ps output format ` + "\n" +
		fmt.Sprintf("%s get rs,services", parentCommand) + `

# List one or more resources by their type and names ` + "\n" +
		fmt.Sprintf("%s get rs/nginx-cb87b6d88 service/kubernetes", parentCommand)
	return example
}
