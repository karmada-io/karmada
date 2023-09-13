package patcher

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
)

// Patcher defines multiple variables that need to be patched.
type Patcher struct {
	labels       map[string]string
	annotations  map[string]string
	extraArgs    map[string]string
	featureGates map[string]bool
	volume       *operatorv1alpha1.VolumeData
	resources    corev1.ResourceRequirements
}

// NewPatcher returns a patcher.
func NewPatcher() *Patcher {
	return &Patcher{}
}

// WithLabels sets labels to the patcher.
func (p *Patcher) WithLabels(labels labels.Set) *Patcher {
	p.labels = labels
	return p
}

// WithAnnotations sets annotations to the patcher.
func (p *Patcher) WithAnnotations(annotations labels.Set) *Patcher {
	p.annotations = annotations
	return p
}

// WithExtraArgs sets extraArgs to the patcher.
func (p *Patcher) WithExtraArgs(extraArgs map[string]string) *Patcher {
	p.extraArgs = extraArgs
	return p
}

// WithFeatureGates sets featureGates to the patcher.
func (p *Patcher) WithFeatureGates(featureGates map[string]bool) *Patcher {
	p.featureGates = featureGates
	return p
}

// WithVolumeData sets VolumeData to the patcher.
func (p *Patcher) WithVolumeData(volume *operatorv1alpha1.VolumeData) *Patcher {
	p.volume = volume
	return p
}

// WithResources sets resources to the patcher.
func (p *Patcher) WithResources(resources corev1.ResourceRequirements) *Patcher {
	p.resources = resources
	return p
}

// ForDeployment patches the deployment manifest.
func (p *Patcher) ForDeployment(deployment *appsv1.Deployment) {
	deployment.Labels = labels.Merge(deployment.Labels, p.labels)
	deployment.Spec.Template.Labels = labels.Merge(deployment.Spec.Template.Labels, p.labels)

	deployment.Annotations = labels.Merge(deployment.Annotations, p.annotations)
	deployment.Spec.Template.Annotations = labels.Merge(deployment.Spec.Template.Annotations, p.annotations)

	if p.resources.Size() > 0 {
		// It's considered the first container is the karmada component by default.
		deployment.Spec.Template.Spec.Containers[0].Resources = p.resources
	}
	if len(p.extraArgs) != 0 || len(p.featureGates) != 0 {
		// It's considered the first container is the karmada component by default.
		baseArguments := deployment.Spec.Template.Spec.Containers[0].Command
		argsMap := parseArgumentListToMap(baseArguments)

		overrideArgs := map[string]string{}

		// merge featureGates and build to an argument.
		if len(p.featureGates) != 0 {
			baseFeatureGates := map[string]bool{}

			if argument, ok := argsMap["feature-gates"]; ok {
				baseFeatureGates = parseFeatrueGatesArgumentToMap(argument)
			}
			overrideArgs["feature-gates"] = buildFeatureGatesArgumentFromMap(baseFeatureGates, p.featureGates)
		}

		for key, val := range p.extraArgs {
			overrideArgs[key] = val
		}

		// the first argument is most often the binary name
		command := []string{baseArguments[0]}
		command = append(command, buildArgumentListFromMap(argsMap, overrideArgs)...)
		deployment.Spec.Template.Spec.Containers[0].Command = command
	}
}

// ForStatefulSet patches the statefulset manifest.
func (p *Patcher) ForStatefulSet(sts *appsv1.StatefulSet) {
	sts.Labels = labels.Merge(sts.Labels, p.labels)
	sts.Spec.Template.Labels = labels.Merge(sts.Spec.Template.Labels, p.labels)

	sts.Annotations = labels.Merge(sts.Annotations, p.annotations)
	sts.Spec.Template.Annotations = labels.Merge(sts.Spec.Template.Annotations, p.annotations)

	if p.volume != nil {
		patchVolumeForStatefulSet(sts, p.volume)
	}

	if p.resources.Size() > 0 {
		// It's considered the first container is the karmada component by default.
		sts.Spec.Template.Spec.Containers[0].Resources = p.resources
	}
	if len(p.extraArgs) != 0 {
		// It's considered the first container is the karmada component by default.
		baseArguments := sts.Spec.Template.Spec.Containers[0].Command
		argsMap := parseArgumentListToMap(baseArguments)
		sts.Spec.Template.Spec.Containers[0].Command = buildArgumentListFromMap(argsMap, p.extraArgs)
	}
}

func buildArgumentListFromMap(baseArguments, overrideArguments map[string]string) []string {
	var command []string
	var keys []string

	argsMap := make(map[string]string)

	for k, v := range baseArguments {
		argsMap[k] = v
	}
	for k, v := range overrideArguments {
		argsMap[k] = v
	}

	for k := range argsMap {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		command = append(command, fmt.Sprintf("--%s=%s", k, argsMap[k]))
	}

	return command
}

func parseFeatrueGatesArgumentToMap(featureGates string) map[string]bool {
	featureGateSlice := strings.Split(featureGates, ",")

	featureGatesMap := map[string]bool{}
	for _, featureGate := range featureGateSlice {
		key, val, err := parseFeatureGate(featureGate)
		if err != nil {
			continue
		}
		featureGatesMap[key] = val
	}

	return featureGatesMap
}

func buildFeatureGatesArgumentFromMap(baseFeatureGates, overrideFeatureGates map[string]bool) string {
	var featureGates []string
	var keys []string

	featureGateMap := make(map[string]bool)

	for k, v := range baseFeatureGates {
		featureGateMap[k] = v
	}
	for k, v := range overrideFeatureGates {
		featureGateMap[k] = v
	}

	for k := range featureGateMap {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		featureGates = append(featureGates, fmt.Sprintf("%s=%s", k, strconv.FormatBool(featureGateMap[k])))
	}

	return strings.Join(featureGates, ",")
}

func parseArgumentListToMap(arguments []string) map[string]string {
	resultingMap := map[string]string{}
	for i, arg := range arguments {
		key, val, err := parseArgument(arg)
		// Ignore if the first argument doesn't satisfy the criteria, it's most often the binary name
		// Warn in all other cases, but don't error out. This can happen only if the user has edited the argument list by hand, so they might know what they are doing
		if err != nil {
			if i != 0 {
				klog.Warningf("WARNING: The component argument %q could not be parsed correctly. The argument must be of the form %q. Skipping...\n", arg, "--")
			}
			continue
		}

		resultingMap[key] = val
	}
	return resultingMap
}

func parseArgument(arg string) (string, string, error) {
	if !strings.HasPrefix(arg, "--") {
		return "", "", errors.New("the argument should start with '--'")
	}
	if !strings.Contains(arg, "=") {
		return "", "", errors.New("the argument should have a '=' between the flag and the value")
	}

	arg = strings.TrimPrefix(arg, "--")
	keyvalSlice := strings.SplitN(arg, "=", 2)

	if len(keyvalSlice) != 2 {
		return "", "", errors.New("the argument must have both a key and a value")
	}
	if len(keyvalSlice[0]) == 0 {
		return "", "", errors.New("the argument must have a key")
	}

	return keyvalSlice[0], keyvalSlice[1], nil
}

func parseFeatureGate(featureGate string) (string, bool, error) {
	if !strings.Contains(featureGate, "=") {
		return "", false, errors.New("the featureGate should have a '=' between the flag and the value")
	}

	keyvalSlice := strings.SplitN(featureGate, "=", 2)

	if len(keyvalSlice) != 2 {
		return "", false, errors.New("the featureGate must have both a key and a value")
	}
	if len(keyvalSlice[0]) == 0 {
		return "", false, errors.New("the featureGate must have a key")
	}

	val, err := strconv.ParseBool(keyvalSlice[1])
	if err != nil {
		return "", false, errors.New("the featureGate value must have a value of type bool")
	}

	return keyvalSlice[0], val, nil
}

func patchVolumeForStatefulSet(sts *appsv1.StatefulSet, volume *operatorv1alpha1.VolumeData) {
	if volume.EmptyDir != nil {
		volumes := sts.Spec.Template.Spec.Volumes
		volumes = append(volumes, corev1.Volume{
			Name: constants.EtcdDataVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
		sts.Spec.Template.Spec.Volumes = volumes
	}

	if volume.HostPath != nil {
		volumes := sts.Spec.Template.Spec.Volumes
		volumes = append(volumes, corev1.Volume{
			Name: constants.EtcdDataVolumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: volume.HostPath.Path,
					Type: volume.HostPath.Type,
				},
			},
		})
		sts.Spec.Template.Spec.Volumes = volumes
	}

	if volume.VolumeClaim != nil {
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: constants.EtcdDataVolumeName,
				},
				Spec: volume.VolumeClaim.Spec,
			},
		}
	}
}
