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
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/interrupt"
	utilpointer "k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	printColumnClusterNum = 1
	proxyURL              = "/apis/cluster.karmada.io/v1alpha1/clusters/%s/proxy/"
)

var (
	getIn  = os.Stdin
	getOut = os.Stdout
	getErr = os.Stderr

	podColumns = []metav1.TableColumnDefinition{
		{Name: "CLUSTER", Type: "string", Format: "", Priority: 0},
		{Name: "ADOPTION", Type: "string", Format: "", Priority: 0},
	}
	eventColumn = metav1.TableColumnDefinition{Name: "EVENT", Type: "string", Format: "", Priority: 0}

	getShort = `Display one or many resources`
)

// NewCmdGet New get command
func NewCmdGet(karmadaConfig KarmadaConfig, parentCommand string) *cobra.Command {
	ioStreams := genericclioptions.IOStreams{In: getIn, Out: getOut, ErrOut: getErr}
	o := NewCommandGetOptions("karmadactl", ioStreams)
	cmd := &cobra.Command{
		Use:                   "get [NAME | -l label | -n namespace]  [flags]",
		DisableFlagsInUseLine: true,
		Short:                 getShort,
		SilenceUsage:          true,
		Example:               getExample(parentCommand),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(cmd); err != nil {
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
	cmd.Flags().StringVar(&o.ClusterNamespace, "cluster-namespace", options.DefaultKarmadaClusterNamespace, "Namespace in the control plane where member cluster secrets are stored.")
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().BoolVar(&o.IgnoreNotFound, "ignore-not-found", o.IgnoreNotFound, "If the requested object does not exist the command will return exit code 0.")
	cmd.Flags().BoolVarP(&o.Watch, "watch", "w", o.Watch, "After listing/getting the requested object, watch for changes. Uninitialized objects are excluded if no object name is provided.")
	cmd.Flags().BoolVar(&o.WatchOnly, "watch-only", o.WatchOnly, "Watch for changes to the requested object(s), without listing/getting first.")
	cmd.Flags().BoolVar(&o.OutputWatchEvents, "output-watch-events", o.OutputWatchEvents, "Output watch event objects when --watch or --watch-only is used. Existing objects are output as initial ADDED events.")

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

	Watch     bool
	WatchOnly bool
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
func (g *CommandGetOptions) Complete() error {
	newScheme := gclient.NewSchema()

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

		if outputObjects != nil {
			printer = &skipPrinter{delegate: printer, output: outputObjects}
		}
		if g.ServerPrint {
			printer = &get.TablePrinter{Delegate: printer}
		}

		return printer.PrintObj, nil
	}
	return nil
}

// Validate checks the set of flags provided by the user.
func (g *CommandGetOptions) Validate(cmd *cobra.Command) error {
	if cmdutil.GetFlagBool(cmd, "show-labels") {
		outputOption := cmd.Flags().Lookup("output").Value.String()
		if outputOption != "" && outputOption != "wide" {
			return fmt.Errorf("--show-labels option cannot be used with %s printer", outputOption)
		}
	}
	if g.OutputWatchEvents && !(g.Watch || g.WatchOnly) {
		return fmt.Errorf("--output-watch-events option can only be used with --watch or --watch-only")
	}
	return nil
}

// Obj cluster info
type Obj struct {
	Cluster string
	Info    *resource.Info
}

// WatchObj is a obj that is watched
type WatchObj struct {
	Cluster string
	r       *resource.Result
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
	var watchObjs []WatchObj
	var allErrs []error

	if g.AllNamespaces {
		g.ExplicitNamespace = false
	}

	outputOption := cmd.Flags().Lookup("output").Value.String()
	if strings.Contains(outputOption, "custom-columns") || outputOption == "yaml" || strings.Contains(outputOption, "json") {
		g.ServerPrint = false
	}

	clusterInfos := make(map[string]*ClusterInfo)
	RBInfo = make(map[string]*OtherPrint)

	karmadaRestConfig, err := clusterInfoInit(g, karmadaConfig, clusterInfos)
	if err != nil {
		return err
	}

	wg.Add(len(g.Clusters))
	for idx := range g.Clusters {
		g.setClusterProxyInfo(karmadaRestConfig, g.Clusters[idx], clusterInfos)
		f := getFactory(g.Clusters[idx], clusterInfos)
		go g.getObjInfo(&wg, &mux, f, g.Clusters[idx], &objs, &watchObjs, &allErrs, args)
	}
	wg.Wait()

	if g.Watch || g.WatchOnly {
		return g.watch(watchObjs)
	}

	if !g.IsHumanReadablePrinter {
		// have printed objects in yaml or json format above
		return nil
	}

	// sort objects by resource kind to classify them
	sort.Slice(objs, func(i, j int) bool {
		return objs[i].Info.Mapping.Resource.String() < objs[j].Info.Mapping.Resource.String()
	})

	g.printObjs(objs, &allErrs, args)

	return utilerrors.NewAggregate(allErrs)
}

// printObjs print objects in multi clusters
func (g *CommandGetOptions) printObjs(objs []Obj, allErrs *[]error, args []string) {
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
	allResourcesNamespaced := !g.AllNamespaces
	sameKind := make([]Obj, 0)
	for ix := range objs {
		mapping := objs[ix].Info.Mapping
		sameKind = append(sameKind, objs[ix])

		allResourcesNamespaced = allResourcesNamespaced && objs[ix].Info.Namespaced()
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

		if ix == len(objs)-1 || objs[ix].Info.Mapping.Resource != objs[ix+1].Info.Mapping.Resource {
			table := &metav1.Table{}
			allTableRows, mapping, err := g.reconstructionRow(sameKind, table)
			if err != nil {
				*allErrs = append(*allErrs, err)
				return
			}
			table.Rows = allTableRows

			setNoAdoption(mapping)
			g.setColumnDefinition(table)

			printObj, err := helper.ToUnstructured(table)
			if err != nil {
				*allErrs = append(*allErrs, err)
				return
			}

			err = printer.PrintObj(printObj, w)
			if err != nil {
				*allErrs = append(*allErrs, err)
				return
			}

			sameKind = make([]Obj, 0)
		}
	}
	w.Flush()

	g.printIfNotFindResource(trackingWriter.Written, allErrs, allResourcesNamespaced)
}

// printIfNotFindResource is sure we output something if we wrote no output, and had no errors, and are not ignoring NotFound
func (g *CommandGetOptions) printIfNotFindResource(written int, allErrs *[]error, allResourcesNamespaced bool) {
	if written == 0 && !g.IgnoreNotFound && len(*allErrs) == 0 {
		if allResourcesNamespaced {
			fmt.Fprintf(g.ErrOut, "No resources found in %s namespace.\n", g.Namespace)
		} else {
			fmt.Fprintln(g.ErrOut, "No resources found")
		}
	}
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
	cluster string, objs *[]Obj, watchObjs *[]WatchObj, allErrs *[]error, args []string) {
	defer wg.Done()

	restClient, err := f.RESTClient()
	if err != nil {
		*allErrs = append(*allErrs, err)
		return
	}

	// check if it is authorized to proxy this member cluster
	request := restClient.Get().RequestURI(fmt.Sprintf(proxyURL, cluster) + "api")
	if _, err := request.DoRaw(context.TODO()); err != nil {
		*allErrs = append(*allErrs, fmt.Errorf("cluster(%s) is inaccessible, please check authorization or network", cluster))
		return
	}

	r := f.NewBuilder().
		Unstructured().
		NamespaceParam(g.Namespace).DefaultNamespace().AllNamespaces(g.AllNamespaces).
		FilenameParam(g.ExplicitNamespace, &g.FilenameOptions).
		LabelSelectorParam(g.LabelSelector).
		FieldSelectorParam(g.FieldSelector).
		RequestChunksOf(g.ChunkSize).
		ResourceTypeOrNameArgs(true, args...).
		ContinueOnError().
		Latest().
		Flatten().
		TransformRequests(g.transformRequests).
		Do()

	if g.IgnoreNotFound {
		r.IgnoreErrors(apierrors.IsNotFound)
	}

	if err := r.Err(); err != nil {
		*allErrs = append(*allErrs, fmt.Errorf("cluster(%s): %s", cluster, err))
		return
	}

	if g.Watch || g.WatchOnly {
		mux.Lock()
		watchObjsInfo := WatchObj{
			Cluster: cluster,
			r:       r,
		}
		*watchObjs = append(*watchObjs, watchObjsInfo)
		mux.Unlock()
		return
	}

	if !g.IsHumanReadablePrinter {
		if err := g.printGeneric(r); err != nil {
			*allErrs = append(*allErrs, fmt.Errorf("cluster(%s): %s", cluster, err))
		}
		return
	}

	infos, err := r.Infos()
	if err != nil {
		*allErrs = append(*allErrs, fmt.Errorf("cluster(%s): %s", cluster, err))
		return
	}

	mux.Lock()
	var objInfo Obj
	for ix := range infos {
		objInfo = Obj{
			Cluster: cluster,
			Info:    infos[ix],
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
		mapping = objs[ix].Info.Mapping
		unstr, ok := objs[ix].Info.Object.(*unstructured.Unstructured)
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

// reconstructObj reconstruct runtime.object row
func (g *CommandGetOptions) reconstructObj(obj runtime.Object, mapping *meta.RESTMapping, cluster string, event string) (*metav1.Table, error) {
	table := &metav1.Table{}
	var allTableRows []metav1.TableRow

	unstr, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("attempt to decode non-Unstructured object")
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.Object, table); err != nil {
		return nil, err
	}

	for rowIdx := range table.Rows {
		var tempRow metav1.TableRow
		rbKey := getRBKey(mapping.GroupVersionKind, table.Rows[rowIdx], cluster)

		if g.OutputWatchEvents {
			tempRow.Cells = append(append(tempRow.Cells, event, table.Rows[rowIdx].Cells[0], cluster), table.Rows[rowIdx].Cells[1:]...)
		} else {
			tempRow.Cells = append(append(tempRow.Cells, table.Rows[rowIdx].Cells[0], cluster), table.Rows[rowIdx].Cells[1:]...)
		}
		if _, ok := RBInfo[rbKey]; ok {
			tempRow.Cells = append(tempRow.Cells, "Y")
		} else {
			tempRow.Cells = append(tempRow.Cells, "N")
		}
		table.Rows[rowIdx].Cells = tempRow.Cells
	}
	allTableRows = append(allTableRows, table.Rows...)

	table.Rows = allTableRows

	setNoAdoption(mapping)
	g.setColumnDefinition(table)

	return table, nil
}

// watch starts a client-side watch of one or more resources.
func (g *CommandGetOptions) watch(watchObjs []WatchObj) error {
	if len(watchObjs) <= 0 {
		return fmt.Errorf("not to find obj that is watched")
	}
	infos, err := watchObjs[0].r.Infos()
	if err != nil {
		return err
	}

	var objs []Obj
	for ix := range infos {
		objs = append(objs, Obj{Cluster: watchObjs[0].Cluster, Info: infos[ix]})
	}

	if multipleGVKsRequested(objs) {
		return fmt.Errorf("watch is only supported on individual resources and resource collections - more than 1 resource was found")
	}

	info := infos[0]
	mapping := info.ResourceMapping()
	outputObjects := utilpointer.BoolPtr(!g.WatchOnly)

	printer, err := g.ToPrinter(mapping, outputObjects, g.AllNamespaces, false)
	if err != nil {
		return err
	}
	writer := printers.GetNewTabWriter(g.Out)

	// print the current object
	for idx := range watchObjs {
		var objsToPrint []runtime.Object
		obj, err := watchObjs[idx].r.Object()
		if err != nil {
			return err
		}

		isList := meta.IsListType(obj)

		if isList {
			tmpObj, _ := meta.ExtractList(obj)
			objsToPrint = append(objsToPrint, tmpObj...)
		} else {
			objsToPrint = append(objsToPrint, obj)
		}

		for _, objToPrint := range objsToPrint {
			objrow, err := g.reconstructObj(objToPrint, mapping, watchObjs[idx].Cluster, string(watch.Added))
			if err != nil {
				return err
			}

			if idx > 0 {
				// only print ColumnDefinitions once
				objrow.ColumnDefinitions = nil
			}

			printObj, err := helper.ToUnstructured(objrow)
			if err != nil {
				return err
			}

			if err := printer.PrintObj(printObj, writer); err != nil {
				return fmt.Errorf("unable to output the provided object: %v", err)
			}
		}
	}
	writer.Flush()

	g.watchMultiClusterObj(watchObjs, mapping, outputObjects, printer)

	return nil
}

//watchMultiClusterObj watch objects in multi clusters by goroutines
func (g *CommandGetOptions) watchMultiClusterObj(watchObjs []WatchObj, mapping *meta.RESTMapping, outputObjects *bool, printer printers.ResourcePrinterFunc) {
	var wg sync.WaitGroup

	writer := printers.GetNewTabWriter(g.Out)

	wg.Add(len(watchObjs))
	for _, watchObj := range watchObjs {
		go func(watchObj WatchObj) {
			obj, err := watchObj.r.Object()
			if err != nil {
				panic(err)
			}

			rv := "0"
			isList := meta.IsListType(obj)
			if isList {
				// the resourceVersion of list objects is ~now but won't return
				// an initial watch event
				rv, err = meta.NewAccessor().ResourceVersion(obj)
				if err != nil {
					panic(err)
				}
			}

			if isList {
				// we can start outputting objects now, watches started from lists don't emit synthetic added events
				*outputObjects = true
			} else {
				// suppress output, since watches started for individual items emit a synthetic ADDED event first
				*outputObjects = false
			}

			if isList {
				// we can start outputting objects now, watches started from lists don't emit synthetic added events
				*outputObjects = true
			} else {
				// suppress output, since watches started for individual items emit a synthetic ADDED event first
				*outputObjects = false
			}

			// print watched changes
			w, err := watchObj.r.Watch(rv)
			if err != nil {
				panic(err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			intr := interrupt.New(nil, cancel)
			_ = intr.Run(func() error {
				_, err := watchtools.UntilWithoutRetry(ctx, w, func(e watch.Event) (bool, error) {
					objToPrint := e.Object

					objrow, err := g.reconstructObj(objToPrint, mapping, watchObj.Cluster, string(e.Type))
					if err != nil {
						return false, err
					}
					// not need to print ColumnDefinitions
					objrow.ColumnDefinitions = nil

					printObj, err := helper.ToUnstructured(objrow)
					if err != nil {
						return false, err
					}

					if err := printer.PrintObj(printObj, writer); err != nil {
						return false, err
					}
					writer.Flush()
					// after processing at least one event, start outputting objects
					*outputObjects = true
					return false, nil
				})
				return err
			})
		}(watchObj)
	}
	wg.Wait()
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
	KubeConfig string
	Context    string

	APIEndpoint     string
	ClusterSyncMode clusterv1alpha1.ClusterSyncMode
}

func clusterInfoInit(g *CommandGetOptions, karmadaConfig KarmadaConfig, clusterInfos map[string]*ClusterInfo) (*rest.Config, error) {
	karmadaRestConfig, err := karmadaConfig.GetRestConfig(g.KarmadaContext, g.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			g.KarmadaContext, g.KubeConfig, err)
	}

	if err := getClusterInKarmada(karmadaRestConfig, clusterInfos); err != nil {
		return nil, fmt.Errorf("method getClusterInKarmada get cluster info in karmada failed, err is: %w", err)
	}

	if err := g.getRBInKarmada(g.Namespace, karmadaRestConfig); err != nil {
		return nil, err
	}

	if len(g.Clusters) <= 0 {
		for c := range clusterInfos {
			g.Clusters = append(g.Clusters, c)
		}
	}
	return karmadaRestConfig, nil
}

func getFactory(clusterName string, clusterInfos map[string]*ClusterInfo) cmdutil.Factory {
	kubeConfigFlags := NewConfigFlags(true).WithDeprecatedPasswordFlag()
	// Build member cluster kubeConfigFlags
	kubeConfigFlags.APIServer = stringptr(clusterInfos[clusterName].APIEndpoint)

	// Use kubeconfig to access member cluster
	kubeConfigFlags.KubeConfig = stringptr(clusterInfos[clusterName].KubeConfig)
	kubeConfigFlags.Context = stringptr(clusterInfos[clusterName].Context)
	kubeConfigFlags.usePersistentConfig = true

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

// setClusterProxyInfo set proxy information of cluster
func (g *CommandGetOptions) setClusterProxyInfo(karmadaRestConfig *rest.Config, name string, clusterInfos map[string]*ClusterInfo) {
	clusterInfos[name].APIEndpoint = karmadaRestConfig.Host + fmt.Sprintf(proxyURL, name)
	clusterInfos[name].KubeConfig = g.KubeConfig
	clusterInfos[name].Context = g.KarmadaContext
	if clusterInfos[name].KubeConfig == "" {
		env := os.Getenv("KUBECONFIG")
		if env != "" {
			clusterInfos[name].KubeConfig = env
		} else {
			clusterInfos[name].KubeConfig = defaultKubeConfig
		}
	}
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
	gvk := objs[0].Info.Mapping.GroupVersionKind
	for _, obj := range objs {
		if obj.Info.Mapping.GroupVersionKind != gvk {
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
func (g *CommandGetOptions) setColumnDefinition(table *metav1.Table) {
	var tempColumnDefinition []metav1.TableColumnDefinition
	if len(table.ColumnDefinitions) > 0 {
		if g.OutputWatchEvents {
			tempColumnDefinition = append(append(append(tempColumnDefinition, eventColumn, table.ColumnDefinitions[0], podColumns[0]), table.ColumnDefinitions[1:]...), podColumns[1:]...)
		} else {
			tempColumnDefinition = append(append(append(tempColumnDefinition, table.ColumnDefinitions[0], podColumns[0]), table.ColumnDefinitions[1:]...), podColumns[1:]...)
		}
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

// skipPrinter allows conditionally suppressing object output via the output field.
// table objects are suppressed by setting their Rows to nil (allowing column definitions to propagate to the delegate).
// non-table objects are suppressed by not calling the delegate at all.
type skipPrinter struct {
	delegate printers.ResourcePrinter
	output   *bool
}

func (p *skipPrinter) PrintObj(obj runtime.Object, writer io.Writer) error {
	if *p.output {
		return p.delegate.PrintObj(obj, writer)
	}

	table, isTable := obj.(*metav1.Table)
	if !isTable {
		return nil
	}

	table = table.DeepCopy()
	table.Rows = nil
	return p.delegate.PrintObj(table, writer)
}
