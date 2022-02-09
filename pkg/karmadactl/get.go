package karmadactl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
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
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const printColumnClusterNum = 1

var (
	getIn  = os.Stdin
	getOut = os.Stdout
	getErr = os.Stderr

	podColumns = []metav1.TableColumnDefinition{
		{Name: "CLUSTER", Type: "string", Format: "", Priority: 0},
		{Name: "ADOPTION", Type: "string", Format: "", Priority: 0},
	}

	noPushModeMessage = "The karmadactl get command now only supports Push mode, [ %s ] are not push mode\n"
	getShort          = `Display one or many resources`
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

	o.GlobalCommandOptions.AddFlags(cmd.Flags())
	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "default", "-n=namespace or -n namespace")
	cmd.Flags().StringVarP(&o.LabelSelector, "labels", "l", "", "-l=label or -l label")
	cmd.Flags().StringSliceVarP(&o.Clusters, "clusters", "C", []string{}, "-C=member1,member2")
	cmd.Flags().StringVar(&o.ClusterNamespace, "cluster-namespace", options.DefaultKarmadaClusterNamespace, "Namespace in the control plane where member cluster are stored.")
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")

	return cmd
}

// CommandGetOptions contains the input to the get command.
type CommandGetOptions struct {
	// global flags
	options.GlobalCommandOptions

	// ClusterNamespace holds the namespace name where the member cluster objects are stored.
	ClusterNamespace string

	Clusters []string

	PrintFlags             *get.PrintFlags
	ToPrinter              func(*meta.RESTMapping, *bool, bool, bool) (printers.ResourcePrinterFunc, error)
	IsHumanReadablePrinter bool

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
		PrintFlags:  get.NewGetPrintFlags(),
		CmdParent:   parent,
		IOStreams:   streams,
		ChunkSize:   500,
		ServerPrint: true,
	}
}

// Complete takes the command arguments and infers any remaining options.
func (g *CommandGetOptions) Complete(cmd *cobra.Command, args []string) error {
	newScheme := gclient.NewSchema()

	outputOption := cmd.Flags().Lookup("output").Value.String()
	if strings.Contains(outputOption, "custom-columns") || outputOption == "yaml" || strings.Contains(outputOption, "json") {
		g.ServerPrint = false
	}

	templateArg := ""
	if g.PrintFlags.TemplateFlags != nil && g.PrintFlags.TemplateFlags.TemplateArgument != nil {
		templateArg = *g.PrintFlags.TemplateFlags.TemplateArgument
	}

	// human readable printers have special conversion rules, so we determine if we're using one.
	if (len(*g.PrintFlags.OutputFormat) == 0 && len(templateArg) == 0) || *g.PrintFlags.OutputFormat == "wide" {
		g.IsHumanReadablePrinter = true
	}

	g.ToPrinter = func(mapping *meta.RESTMapping, outputObjects *bool, withNamespace bool, withKind bool) (printers.ResourcePrinterFunc, error) {
		// make a new copy of current flags / opts before mutating
		printFlags := g.PrintFlags.Copy()

		if mapping != nil {
			printFlags.SetKind(mapping.GroupVersionKind.GroupKind())
		}

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

	clusterInfos := make(map[string]*ClusterInfo)
	RBInfo = make(map[string]*OtherPrint)

	if g.AllNamespaces {
		g.ExplicitNamespace = false
	}

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
			return fmt.Errorf("method getSecretTokenInKarmada get Secret info in karmada failed, err is: %w", err)
		}
		f := getFactory(g.Clusters[idx], clusterInfos)
		go g.getObjInfo(&wg, &mux, f, g.Clusters[idx], &objs, &allErrs, args)
	}
	wg.Wait()
	if len(noPushModeCluster) != 0 {
		fmt.Println(fmt.Sprintf(noPushModeMessage, strings.Join(noPushModeCluster, ",")))
	}

	// sort objects by resource kind to classify them
	sort.Slice(objs, func(i, j int) bool {
		return objs[i].Mapping.Resource.String() < objs[j].Mapping.Resource.String()
	})

	if err := g.printObjs(objs, &allErrs, args); err != nil {
		return err
	}

	return utilerrors.NewAggregate(allErrs)
}

// printObjs print objects in multi clusters
func (g *CommandGetOptions) printObjs(objs []Obj, allErrs *[]error, args []string) error {
	var err error
	errs := sets.NewString()

	printWithKind := multipleGVKsRequested(objs)

	var printer printers.ResourcePrinter
	var lastMapping *meta.RESTMapping

	// track if we write any output
	trackingWriter := &trackingWriterWrapper{Delegate: g.Out}
	// output an empty line separating output
	separatorWriter := &separatorWriterWrapper{Delegate: trackingWriter}

	w := printers.GetNewTabWriter(separatorWriter)
	sameKind := make([]Obj, 0)
	for ix := range objs {
		mapping := objs[ix].Mapping
		sameKind = append(sameKind, objs[ix])

		printWithNamespace := g.checkPrintWithNamespace(mapping)

		if shouldGetNewPrinterForMapping(printer, lastMapping, mapping) {
			w.Flush()
			w.SetRememberedWidths(nil)

			// add linebreaks between resource groups (if there is more than one)
			// when it satisfies all following 3 conditions:
			// 1) it's not the first resource group
			// 2) it has row header
			// 3) we've written output since the last time we started a new set of headers
			if lastMapping != nil && !g.NoHeaders && trackingWriter.Written > 0 {
				separatorWriter.SetReady(true)
			}

			printer, err = g.ToPrinter(mapping, nil, printWithNamespace, printWithKind)
			if err != nil {
				if !errs.Has(err.Error()) {
					errs.Insert(err.Error())
					*allErrs = append(*allErrs, err)
				}
				continue
			}
			lastMapping = mapping
		}

		if ix == len(objs)-1 || objs[ix].Mapping.Resource != objs[ix+1].Mapping.Resource {
			table := &metav1.Table{}
			allTableRows, mapping, err := g.reconstructionRow(sameKind, table)
			if err != nil {
				return err
			}
			table.Rows = allTableRows

			setNoAdoption(mapping)
			setColumnDefinition(table)

			printObj, err := helper.ToUnstructured(table)
			if err != nil {
				return err
			}

			err = printer.PrintObj(printObj, w)
			if err != nil {
				return err
			}

			sameKind = make([]Obj, 0)
		}
	}

	w.Flush()

	return nil
}

// checkPrintWithNamespace check if print objects with namespace
func (g *CommandGetOptions) checkPrintWithNamespace(mapping *meta.RESTMapping) bool {
	if mapping != nil && mapping.Scope.Name() == meta.RESTScopeNameRoot {
		return false
	}
	return g.AllNamespaces
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

	if !g.IsHumanReadablePrinter {
		err := g.printGeneric(r)

		*allErrs = append(*allErrs, err)
		return
	}

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
			rbKey := getRBKey(mapping.GroupVersionKind, table.Rows[rowIdx], objs[ix].Cluster)

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

func (g *CommandGetOptions) printGeneric(r *resource.Result) error {
	// we flattened the data from the builder, so we have individual items, but now we'd like to either:
	// 1. if there is more than one item, combine them all into a single list
	// 2. if there is a single item and that item is a list, leave it as its specific list
	// 3. if there is a single item and it is not a list, leave it as a single item
	var errs []error
	singleItemImplied := false

	infos, err := g.extractInfosFromResource(r, &errs, &singleItemImplied)
	if err != nil {
		return err
	}

	printer, err := g.ToPrinter(nil, nil, false, false)
	if err != nil {
		return err
	}

	var obj runtime.Object
	if !singleItemImplied || len(infos) != 1 {
		// we have zero or multple items, so coerce all items into a list.
		// we don't want an *unstructured.Unstructured list yet, as we
		// may be dealing with non-unstructured objects. Compose all items
		// into an corev1.List, and then decode using an unstructured scheme.
		list := corev1.List{
			TypeMeta: metav1.TypeMeta{
				Kind:       "List",
				APIVersion: "v1",
			},
			ListMeta: metav1.ListMeta{},
		}
		for _, info := range infos {
			list.Items = append(list.Items, runtime.RawExtension{Object: info.Object})
		}

		listData, err := json.Marshal(list)
		if err != nil {
			return err
		}

		converted, err := runtime.Decode(unstructured.UnstructuredJSONScheme, listData)
		if err != nil {
			return err
		}

		obj = converted
	} else {
		obj = infos[0].Object
	}

	isList := meta.IsListType(obj)
	if isList {
		items, err := meta.ExtractList(obj)
		if err != nil {
			return err
		}

		// take the items and create a new list for display
		list := &unstructured.UnstructuredList{
			Object: map[string]interface{}{
				"kind":       "List",
				"apiVersion": "v1",
				"metadata":   map[string]interface{}{},
			},
		}
		if listMeta, err := meta.ListAccessor(obj); err == nil {
			list.Object["metadata"] = map[string]interface{}{
				"selfLink":        listMeta.GetSelfLink(),
				"resourceVersion": listMeta.GetResourceVersion(),
			}
		}

		for _, item := range items {
			list.Items = append(list.Items, *item.(*unstructured.Unstructured))
		}
		if err := printer.PrintObj(list, g.Out); err != nil {
			errs = append(errs, err)
		}
		return utilerrors.Reduce(utilerrors.Flatten(utilerrors.NewAggregate(errs)))
	}

	if printErr := printer.PrintObj(obj, g.Out); printErr != nil {
		errs = append(errs, printErr)
	}

	return utilerrors.Reduce(utilerrors.Flatten(utilerrors.NewAggregate(errs)))
}

func (g *CommandGetOptions) extractInfosFromResource(r *resource.Result, errs *[]error, singleItemImplied *bool) ([]*resource.Info, error) {
	infos, err := r.IntoSingleItemImplied(singleItemImplied).Infos()
	if err != nil {
		if *singleItemImplied {
			return nil, err
		}
		*errs = append(*errs, err)
	}

	if len(infos) == 0 && g.IgnoreNotFound {
		return nil, utilerrors.Reduce(utilerrors.Flatten(utilerrors.NewAggregate(*errs)))
	}

	return infos, nil
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
		return nil, fmt.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			g.KarmadaContext, g.KubeConfig, err)
	}

	if err := getClusterInKarmada(karmadaclient, clusterInfos); err != nil {
		return nil, fmt.Errorf("method getClusterInKarmada get cluster info in karmada failed, err is: %w", err)
	}

	if err := g.getRBInKarmada(g.Namespace, karmadaclient); err != nil {
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
	if !g.ServerPrint || !g.IsHumanReadablePrinter {
		return
	}

	req.SetHeader("Accept", strings.Join([]string{
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1.SchemeGroupVersion.Version, metav1.GroupName),
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1beta1.SchemeGroupVersion.Version, metav1beta1.GroupName),
		"application/json",
	}, ","))
}

func (g *CommandGetOptions) getRBInKarmada(namespace string, config *rest.Config) error {
	rbList := &workv1alpha2.ResourceBindingList{}
	crbList := &workv1alpha2.ClusterResourceBindingList{}

	gClient, err := gclient.NewForConfig(config)
	if err != nil {
		return err
	}

	if !g.AllNamespaces {
		err = gClient.List(context.TODO(), rbList, &client.ListOptions{
			Namespace: namespace,
		})
	} else {
		err = gClient.List(context.TODO(), rbList, &client.ListOptions{})
	}
	if err != nil {
		return err
	}

	if err = gClient.List(context.TODO(), crbList, &client.ListOptions{}); err != nil {
		return err
	}

	for idx := range rbList.Items {
		rbKey := rbList.Items[idx].GetName()
		val := rbList.Items[idx].Status.AggregatedStatus
		for i := range val {
			if val[i].Applied && val[i].ClusterName != "" {
				newRBKey := fmt.Sprintf("%s-%s", val[i].ClusterName, rbKey)
				RBInfo[newRBKey] = &OtherPrint{
					Applied: val[i].Applied,
				}
			}
		}
	}
	for idx := range crbList.Items {
		rbKey := crbList.Items[idx].GetName()
		val := crbList.Items[idx].Status.AggregatedStatus
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

func getRBKey(gvk schema.GroupVersionKind, row metav1.TableRow, cluster string) string {
	resourceName, _ := row.Cells[0].(string)
	rbKey := names.GenerateBindingName(gvk.Kind, resourceName)

	return fmt.Sprintf("%s-%s", cluster, rbKey)
}

func multipleGVKsRequested(objs []Obj) bool {
	if len(objs) < 2 {
		return false
	}
	gvk := objs[0].Mapping.GroupVersionKind
	for _, obj := range objs {
		if obj.Mapping.GroupVersionKind != gvk {
			return true
		}
	}
	return false
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

# List all pods in ps output format with more information (such as node name)` + "\n" +
		fmt.Sprintf("%s get pods -o wide", parentCommand) + `

# List all pods of member1 cluster in ps output format` + "\n" +
		fmt.Sprintf("%s get pods -C member1", parentCommand) + `

# List a single replicasets controller with specified NAME in ps output format ` + "\n" +
		fmt.Sprintf("%s get replicasets nginx", parentCommand) + `

# List deployments in JSON output format, in the "v1" version of the "apps" API group ` + "\n" +
		fmt.Sprintf("%s get deployments.v1.apps -o json", parentCommand) + `

# Return only the phase value of the specified resource ` + "\n" +
		fmt.Sprintf("%s get -o template deployment/nginx -C member1 --template={{.spec.replicas}}", parentCommand) + `

# List all replication controllers and services together in ps output format ` + "\n" +
		fmt.Sprintf("%s get rs,services", parentCommand) + `

# List one or more resources by their type and names ` + "\n" +
		fmt.Sprintf("%s get rs/nginx-cb87b6d88 service/kubernetes", parentCommand)
	return example
}
