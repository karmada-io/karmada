package interpret

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/editor"
	"k8s.io/kubectl/pkg/cmd/util/editor/crlf"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/interpreter"
)

func (o *Options) completeEdit() error {
	if !o.Edit {
		return nil
	}

	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		return o.PrintFlags.ToPrinter()
	}
	return nil
}

// this logic is modified from: https://github.com/kubernetes/kubernetes/blob/70617042976dc168208a41b8a10caa61f9748617/staging/src/k8s.io/kubectl/pkg/cmd/util/editor/editoptions.go#L246-L479
// nolint:gocyclo
func (o *Options) runEdit() error {
	infos, err := o.CustomizationResult.Infos()
	if err != nil {
		return err
	}

	var info *resource.Info
	switch len(infos) {
	case 0:
		if len(o.Filenames) != 1 {
			return fmt.Errorf("no customizations found. If you want to create a new one, please set only one file with '-f'")
		}
		info = &resource.Info{
			Source: o.Filenames[0],
			Object: &configv1alpha1.ResourceInterpreterCustomization{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "config.karmada.io/v1alpha1",
					Kind:       "ResourceInterpreterCustomization",
				},
			},
		}
	case 1:
		info = infos[0]
	default:
		return fmt.Errorf("only one customization can be edited")
	}

	originalCustomization, err := asResourceInterpreterCustomization(info.Object)
	if err != nil {
		return err
	}
	editedCustomization := originalCustomization.DeepCopy()
	editedCustomization.Spec = configv1alpha1.ResourceInterpreterCustomizationSpec{}

	var (
		edit          = editor.NewDefaultEditor(editorEnvs())
		results       = editResults{}
		edited        = []byte{}
		file          string
		containsError = false
	)

	// loop until we succeed or cancel editing
	for {
		// generate the file to edit
		buf := &bytes.Buffer{}
		var w io.Writer = buf
		if o.WindowsLineEndings {
			w = crlf.NewCRLFWriter(w)
		}

		err = results.header.writeTo(w)
		if err != nil {
			return err
		}

		if !containsError {
			printCustomization(w, originalCustomization, o.Rules, o.ShowDoc)
		} else {
			// In case of an error, preserve the edited file.
			// Remove the comments (header) from it since we already
			// have included the latest header in the buffer above.
			buf.Write(stripComments(edited))
		}

		// launch the editor
		editedDiff := edited
		edited, file, err = edit.LaunchTempFile(fmt.Sprintf("%s-edit-", filepath.Base(os.Args[0])), ".lua", buf)
		if err != nil {
			return preservedFile(err, results.file, o.ErrOut)
		}

		// If we're retrying the loop because of an error, and no change was made in the file, short-circuit
		if containsError && bytes.Equal(stripComments(editedDiff), stripComments(edited)) {
			return preservedFile(fmt.Errorf("%s", "Edit cancelled, no valid changes were saved."), file, o.ErrOut)
		}
		// cleanup any file from the previous pass
		if len(results.file) > 0 {
			os.Remove(results.file)
		}
		klog.V(4).Infof("User edited:\n%s", string(edited))

		// build new customization from edited file
		err = parseEditedIntoCustomization(edited, editedCustomization, o.Rules)
		if err != nil {
			results = editResults{
				file: file,
			}
			containsError = true
			fmt.Fprintln(o.ErrOut, results.addError(apierrors.NewInvalid(corev1.SchemeGroupVersion.WithKind("").GroupKind(),
				"", field.ErrorList{field.Invalid(nil, "The edited file failed validation", fmt.Sprintf("%v", err))}), info))
			continue
		}

		// Compare content
		if isEqualsCustomization(originalCustomization, editedCustomization) {
			os.Remove(file)
			fmt.Fprintln(o.ErrOut, "Edit cancelled, no changes made.")
			return nil
		}

		results = editResults{
			file: file,
		}

		// TODO: validate edited customization

		// not a syntax error as it turns out...
		containsError = false

		// TODO: add last-applied-configuration annotation

		switch {
		case info.Source != "":
			err = o.saveToPath(info, editedCustomization, &results)
		default:
			err = o.saveToServer(info, editedCustomization, &results)
		}
		if err != nil {
			return preservedFile(err, results.file, o.ErrOut)
		}

		// Handle all possible errors
		//
		// 1. retryable: propose kubectl replace -f
		// 2. notfound: indicate the location of the saved configuration of the deleted resource
		// 3. invalid: retry those on the spot by looping ie. reloading the editor
		if results.retryable > 0 {
			fmt.Fprintf(o.ErrOut, "You can run `%s replace -f %s` to try this update again.\n", filepath.Base(os.Args[0]), file)
			return cmdutil.ErrExit
		}
		if results.notfound > 0 {
			fmt.Fprintf(o.ErrOut, "The edits you made on deleted resources have been saved to %q\n", file)
			return cmdutil.ErrExit
		}

		if len(results.edit) == 0 {
			if results.notfound == 0 {
				os.Remove(file)
			} else {
				fmt.Fprintf(o.Out, "The edits you made on deleted resources have been saved to %q\n", file)
			}
			return nil
		}

		if len(results.header.reasons) > 0 {
			containsError = true
		}
	}
}

func (o *Options) saveToPath(originalInfo *resource.Info, editedObj runtime.Object, results *editResults) error {
	var w io.Writer
	var writer printers.ResourcePrinter = &printers.YAMLPrinter{}
	source := originalInfo.Source
	switch {
	case source == "":
		return fmt.Errorf("resource %s/%s is not from file", originalInfo.Namespace, originalInfo.Name)
	case source == "STDIN" || strings.HasPrefix(source, "http"):
		w = os.Stdout
	default:
		f, err := os.OpenFile(originalInfo.Source, os.O_RDWR|os.O_TRUNC, 0)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f

		_, _, isJSON := yaml.GuessJSONStream(f, 4096)
		if isJSON {
			writer = &printers.JSONPrinter{}
		}
	}

	err := writer.PrintObj(editedObj, w)
	if err != nil {
		fmt.Fprint(o.ErrOut, results.addError(err, originalInfo))
	}

	printer, err := o.ToPrinter("edited")
	if err != nil {
		return err
	}
	return printer.PrintObj(originalInfo.Object, o.Out)
}

func (o *Options) saveToServer(originalInfo *resource.Info, editedObj runtime.Object, results *editResults) error {
	originalJS, err := json.Marshal(originalInfo.Object)
	if err != nil {
		return err
	}

	editedJS, err := json.Marshal(editedObj)
	if err != nil {
		return err
	}

	preconditions := []mergepatch.PreconditionFunc{
		mergepatch.RequireKeyUnchanged("apiVersion"),
		mergepatch.RequireKeyUnchanged("kind"),
		mergepatch.RequireMetadataKeyUnchanged("name"),
		mergepatch.RequireKeyUnchanged("managedFields"),
	}

	patchType := types.MergePatchType
	patch, err := jsonpatch.CreateMergePatch(originalJS, editedJS)
	if err != nil {
		klog.V(4).Infof("Unable to calculate diff, no merge is possible: %v", err)
		return err
	}
	var patchMap map[string]interface{}
	err = json.Unmarshal(patch, &patchMap)
	if err != nil {
		klog.V(4).Infof("Unable to calculate diff, no merge is possible: %v", err)
		return err
	}
	for _, precondition := range preconditions {
		if !precondition(patchMap) {
			klog.V(4).Infof("Unable to calculate diff, no merge is possible: %v", err)
			return fmt.Errorf("%s", "At least one of apiVersion, kind and name was changed")
		}
	}

	if o.OutputPatch {
		fmt.Fprintf(o.Out, "Patch: %s\n", string(patch))
	}

	patched, err := resource.NewHelper(originalInfo.Client, originalInfo.Mapping).
		WithFieldManager(o.FieldManager).
		WithFieldValidation(o.ValidationDirective).
		WithSubresource(o.Subresource).
		Patch(originalInfo.Namespace, originalInfo.Name, patchType, patch, nil)
	if err != nil {
		fmt.Fprintln(o.ErrOut, results.addError(err, originalInfo))
		return nil
	}
	err = originalInfo.Refresh(patched, true)
	if err != nil {
		return err
	}
	printer, err := o.ToPrinter("edited")
	if err != nil {
		return err
	}
	return printer.PrintObj(originalInfo.Object, o.Out)
}

func isEqualsCustomization(a, b *configv1alpha1.ResourceInterpreterCustomization) bool {
	return a.Namespace == b.Namespace && a.Name == b.Name && reflect.DeepEqual(a.Spec, b.Spec)
}

const (
	luaCommentPrefix        = "--"
	luaAnnotationPrefix     = "---@"
	luaAnnotationName       = luaAnnotationPrefix + "name:"
	luaAnnotationAPIVersion = luaAnnotationPrefix + "apiVersion:"
	luaAnnotationKind       = luaAnnotationPrefix + "kind:"
	luaAnnotationRule       = luaAnnotationPrefix + "rule:"
)

func printCustomization(w io.Writer, c *configv1alpha1.ResourceInterpreterCustomization, rules interpreter.Rules, showDoc bool) {
	fmt.Fprintf(w, "%s %s\n", luaAnnotationName, c.Name)
	fmt.Fprintf(w, "%s %s\n", luaAnnotationAPIVersion, c.Spec.Target.APIVersion)
	fmt.Fprintf(w, "%s %s\n", luaAnnotationKind, c.Spec.Target.Kind)
	for _, r := range rules {
		fmt.Fprintf(w, "%s %s\n", luaAnnotationRule, r.Name())
		if showDoc {
			if doc := r.Document(); doc != "" {
				fmt.Fprintf(w, "%s %s\n", luaCommentPrefix, commentOnLineBreak(doc))
			}
		}
		if script := r.GetScript(c); script != "" {
			fmt.Fprintf(w, "%s", script)
		}
	}
}

// The file is like:
// -- Please edit the object below. Lines beginning with a '--' will be ignored,
// -- and an empty file will abort the edit. If an error occurs while saving this file will be
// -- reopened with the relevant failures.
// --
// ---@name: foo
// ---@apiVersion: apps/v1
// ---@kind: Deployment
// ---@rule: Retain
// -- This rule is used to retain runtime values to the desired specification.
// -- The script should implement a function as follows:
// -- function Retain(desiredObj, observedObj)
// --      desiredObj.spec.fieldFoo = observedObj.spec.fieldFoo
// --      return desiredObj
// -- end
// function Retain(desiredObj, runtimeObj)
//
//	desiredObj.spec.fieldFoo = runtimeObj.spec.fieldFoo
//	return desiredObj
//
// end
func parseEditedIntoCustomization(file []byte, into *configv1alpha1.ResourceInterpreterCustomization, rules interpreter.Rules) error {
	var currRule interpreter.Rule
	var script string
	scanner := bufio.NewScanner(bytes.NewBuffer(file))
	for scanner.Scan() {
		line := scanner.Text()
		trimline := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimline, luaAnnotationName):
			into.Name = strings.TrimSpace(strings.TrimPrefix(trimline, luaAnnotationName))
		case strings.HasPrefix(trimline, luaAnnotationAPIVersion):
			into.Spec.Target.APIVersion = strings.TrimSpace(strings.TrimPrefix(trimline, luaAnnotationAPIVersion))
		case strings.HasPrefix(trimline, luaAnnotationKind):
			into.Spec.Target.Kind = strings.TrimSpace(strings.TrimPrefix(trimline, luaAnnotationKind))
		case strings.HasPrefix(trimline, luaAnnotationRule):
			name := strings.TrimSpace(strings.TrimPrefix(trimline, luaAnnotationRule))
			r := rules.Get(name)
			if r != nil {
				if currRule != nil {
					currRule.SetScript(into, script)
				}
				currRule = r
				script = ""
			}
		case strings.HasPrefix(trimline, luaCommentPrefix):
			// comments are skipped
		default:
			if currRule == nil {
				return fmt.Errorf("unexpected line %q", line)
			}
			script += string(line) + "\n"
		}
	}

	if currRule != nil {
		currRule.SetScript(into, script)
	}
	return nil
}

func stripComments(file []byte) []byte {
	stripped := make([]byte, 0, len(file))

	lines := bytes.Split(file, []byte("\n"))
	for _, line := range lines {
		trimline := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimline, []byte(luaCommentPrefix)) && !bytes.HasPrefix(trimline, []byte(luaAnnotationPrefix)) {
			continue
		}
		if len(stripped) != 0 {
			stripped = append(stripped, '\n')
		}
		stripped = append(stripped, line...)
	}
	return stripped
}

// editReason preserves a message about the reason this file must be edited again
type editReason struct {
	head  string
	other []string
}

// editHeader includes a list of reasons the edit must be retried
type editHeader struct {
	reasons []editReason
}

// writeTo outputs the current header information into a stream
func (h *editHeader) writeTo(w io.Writer) error {
	writeComment(w, `Please edit the object below. Lines beginning with a '--' will be ignored,
and an empty file will abort the edit. If an error occurs while saving this file will be
reopened with the relevant failures.

`)

	for _, r := range h.reasons {
		if len(r.other) > 0 {
			writeComment(w, r.head+":\n")
		} else {
			writeComment(w, r.head+"\n")
		}
		for _, o := range r.other {
			writeComment(w, o+"\n")
		}
		fmt.Fprintln(w, luaCommentPrefix)
	}
	return nil
}

// editResults capture the result of an update
type editResults struct {
	header    editHeader
	retryable int
	notfound  int
	edit      []*resource.Info
	file      string
}

func (r *editResults) addError(err error, info *resource.Info) string {
	switch {
	case apierrors.IsInvalid(err):
		r.edit = append(r.edit, info)
		reason := editReason{
			head: fmt.Sprintf("%s %q was not valid", customizationResourceName, info.Name),
		}
		if err, ok := err.(apierrors.APIStatus); ok {
			if details := err.Status().Details; details != nil {
				for _, cause := range details.Causes {
					reason.other = append(reason.other, fmt.Sprintf("%s: %s", cause.Field, cause.Message))
				}
			}
		}
		r.header.reasons = append(r.header.reasons, reason)
		return fmt.Sprintf("error: %s %q is invalid", customizationResourceName, info.Name)
	case apierrors.IsNotFound(err):
		r.notfound++
		return fmt.Sprintf("error: %s %q could not be found on the server", customizationResourceName, info.Name)
	default:
		r.retryable++
		return fmt.Sprintf("error: %s %q could not be patched: %v", customizationResourceName, info.Name, err)
	}
}

func writeComment(w io.Writer, comment string) {
	fmt.Fprintf(w, "%s %s", luaCommentPrefix, commentOnLineBreak(comment))
}

// commentOnLineBreak returns a string built from the provided string by inserting any necessary '-- '
// characters after '\n' characters, indicating a comment.
func commentOnLineBreak(s string) string {
	r := ""
	for i, ch := range s {
		j := i + 1
		if j < len(s) && ch == '\n' && s[j] != '-' {
			r += "\n-- "
		} else {
			r += string(ch)
		}
	}
	return r
}

// editorEnvs returns an ordered list of env vars to check for editor preferences.
func editorEnvs() []string {
	return []string{
		"KUBE_EDITOR",
		"EDITOR",
	}
}

// preservedFile writes out a message about the provided file if it exists to the
// provided output stream when an error happens. Used to notify the user where
// their updates were preserved.
func preservedFile(err error, path string, out io.Writer) error {
	if len(path) > 0 {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			fmt.Fprintf(out, "A copy of your changes has been stored to %q\n", path)
		}
	}
	return err
}
