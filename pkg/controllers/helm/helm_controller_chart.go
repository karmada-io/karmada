/*
Copyright 2020 The Flux authors

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

package helm

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"

	"github.com/fluxcd/pkg/runtime/acl"
	"github.com/fluxcd/pkg/runtime/transform"
	"github.com/hashicorp/go-retryablehttp"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/strvals"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"

	v2 "github.com/fluxcd/helm-controller/api/v2beta1"
)

func (h *HelmController) reconcileChart(ctx context.Context, hr *v2.HelmRelease) (*sourcev1.HelmChart, error) {
	chartName := types.NamespacedName{
		Namespace: hr.Spec.Chart.GetNamespace(hr.Namespace),
		Name:      hr.GetHelmChartName(),
	}

	if h.NoCrossNamespaceRef && chartName.Namespace != hr.Namespace {
		return nil, acl.AccessDeniedError(fmt.Sprintf("can't access '%s/%s', cross-namespace references have been blocked",
			hr.Spec.Chart.Spec.SourceRef.Kind, types.NamespacedName{
				Namespace: hr.Spec.Chart.Spec.SourceRef.Namespace,
				Name:      hr.Spec.Chart.Spec.SourceRef.Name,
			}))
	}

	// Garbage collect the previous HelmChart if the namespace named changed.
	if hr.Status.HelmChart != "" && hr.Status.HelmChart != chartName.String() {
		if err := h.deleteHelmChart(ctx, hr); err != nil {
			return nil, err
		}
	}

	// Continue with the reconciliation of the current template.
	var helmChart sourcev1.HelmChart
	err := h.Client.Get(ctx, chartName, &helmChart)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	hc := buildHelmChartFromTemplate(hr)
	switch {
	case apierrors.IsNotFound(err):
		if err = h.Client.Create(ctx, hc); err != nil {
			return nil, err
		}
		hr.Status.HelmChart = chartName.String()
		return hc, nil
	case helmChartRequiresUpdate(hr, &helmChart):
		ctrl.LoggerFrom(ctx).Info("chart diverged from template", strings.ToLower(sourcev1.HelmChartKind), chartName.String())
		helmChart.Spec = hc.Spec
		if err = h.Client.Update(ctx, &helmChart); err != nil {
			return nil, err
		}
		hr.Status.HelmChart = chartName.String()
	}
	return &helmChart, nil
}

// getHelmChart retrieves the v1beta2.HelmChart for the given
// v2beta1.HelmRelease using the name that is advertised in the status
// object. It returns the v1beta2.HelmChart, or an error.
func (h *HelmController) getHelmChart(ctx context.Context, hr *v2.HelmRelease) (*sourcev1.HelmChart, error) {
	namespace, name := hr.Status.GetHelmChart()
	hc := &sourcev1.HelmChart{}
	if err := h.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, hc); err != nil {
		return nil, err
	}
	return hc, nil
}

// loadHelmChart attempts to download the artifact from the provided source,
// loads it into a chart.Chart, and removes the downloaded artifact.
// It returns the loaded chart.Chart on success, or an error.
func (h *HelmController) loadHelmChart(source *sourcev1.HelmChart) (*chart.Chart, error) {
	f, err := os.CreateTemp("", fmt.Sprintf("%s-%s-*.tgz", source.GetNamespace(), source.GetName()))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	defer os.Remove(f.Name())

	artifactURL := source.GetArtifact().URL
	if hostname := os.Getenv("SOURCE_CONTROLLER_LOCALHOST"); hostname != "" {
		u, err := url.Parse(artifactURL)
		if err != nil {
			return nil, err
		}
		u.Host = hostname
		artifactURL = u.String()
	}

	req, err := retryablehttp.NewRequest(http.MethodGet, artifactURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create a new request: %w", err)
	}

	resp, err := h.SourceClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download artifact, error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("artifact '%s' download failed (status code: %s)", source.GetArtifact().URL, resp.Status)
	}

	// verify checksum matches origin
	if err := h.copyAndVerifyArtifact(source.GetArtifact(), resp.Body, f); err != nil {
		return nil, err
	}

	return loader.Load(f.Name())
}

func (h *HelmController) copyAndVerifyArtifact(artifact *sourcev1.Artifact, reader io.Reader, writer io.Writer) error {
	hasher := sha256.New()

	// for backwards compatibility with source-controller v0.17.2 and older
	if len(artifact.Checksum) == 40 {
		hasher = sha1.New()
	}

	// compute checksum
	mw := io.MultiWriter(hasher, writer)
	if _, err := io.Copy(mw, reader); err != nil {
		return err
	}

	if checksum := fmt.Sprintf("%x", hasher.Sum(nil)); checksum != artifact.Checksum {
		return fmt.Errorf("failed to verify artifact: computed checksum '%s' doesn't match advertised '%s'",
			checksum, artifact.Checksum)
	}

	return nil
}

// deleteHelmChart deletes the v1beta2.HelmChart of the v2beta1.HelmRelease.
// input is a helmRelease instance, not a template
func (h *HelmController) deleteHelmChart(ctx context.Context, hr *v2.HelmRelease) error {
	if hr.Status.HelmChart == "" {
		return nil
	}
	// check helmRelease template exists
	hrTemplate := &v2.HelmRelease{}
	if getErr := h.Get(ctx, client.ObjectKey{Namespace: hr.Namespace, Name: hr.Name}, hrTemplate); getErr != nil {
		if apierrors.IsNotFound(getErr) {
			var hc sourcev1.HelmChart
			chartNS, chartName := hr.Status.GetHelmChart()
			err := h.Client.Get(ctx, types.NamespacedName{Namespace: chartNS, Name: chartName}, &hc)
			if err != nil {
				if apierrors.IsNotFound(err) {
					hr.Status.HelmChart = ""
					return nil
				}
				err = fmt.Errorf("failed to delete HelmChart '%s': %w", hr.Status.HelmChart, err)
				return err
			}
			if err = h.Client.Delete(ctx, &hc); err != nil {
				err = fmt.Errorf("failed to delete HelmChart '%s': %w", hr.Status.HelmChart, err)
				return err
			}
		} else {
			return fmt.Errorf("failed to get HelmRelease template, err is: %v", getErr)
		}
	}
	// Truncate the chart reference in the status object.
	hr.Status.HelmChart = ""
	return nil
}

// buildHelmChartFromTemplate builds a v1beta2.HelmChart from the
// v2beta1.HelmChartTemplate of the given v2beta1.HelmRelease.
func buildHelmChartFromTemplate(hr *v2.HelmRelease) *sourcev1.HelmChart {
	template := hr.Spec.Chart
	return &sourcev1.HelmChart{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hr.GetHelmChartName(),
			Namespace: hr.Spec.Chart.GetNamespace(hr.Namespace),
		},
		Spec: sourcev1.HelmChartSpec{
			Chart:   template.Spec.Chart,
			Version: template.Spec.Version,
			SourceRef: sourcev1.LocalHelmChartSourceReference{
				Name: template.Spec.SourceRef.Name,
				Kind: template.Spec.SourceRef.Kind,
			},
			Interval:          template.GetInterval(hr.Spec.Interval),
			ReconcileStrategy: template.Spec.ReconcileStrategy,
			ValuesFiles:       template.Spec.ValuesFiles,
			ValuesFile:        template.Spec.ValuesFile,
		},
	}
}

// helmChartRequiresUpdate compares the v2beta1.HelmChartTemplate of the
// v2beta1.HelmRelease to the given v1beta2.HelmChart to determine if an
// update is required.
func helmChartRequiresUpdate(hr *v2.HelmRelease, chart *sourcev1.HelmChart) bool {
	template := hr.Spec.Chart
	switch {
	case template.Spec.Chart != chart.Spec.Chart:
		return true
	// TODO(hidde): remove emptiness checks on next MINOR version
	case template.Spec.Version == "" && chart.Spec.Version != "*",
		template.Spec.Version != "" && template.Spec.Version != chart.Spec.Version:
		return true
	case template.Spec.SourceRef.Name != chart.Spec.SourceRef.Name:
		return true
	case template.Spec.SourceRef.Kind != chart.Spec.SourceRef.Kind:
		return true
	case template.GetInterval(hr.Spec.Interval) != chart.Spec.Interval:
		return true
	case template.Spec.ReconcileStrategy != chart.Spec.ReconcileStrategy:
		return true
	case !reflect.DeepEqual(template.Spec.ValuesFiles, chart.Spec.ValuesFiles):
		return true
	case template.Spec.ValuesFile != chart.Spec.ValuesFile:
		return true
	default:
		return false
	}
}

// composeValues attempts to resolve all v2beta1.ValuesReference resources
// and merges them as defined. Referenced resources are only retrieved once
// to ensure a single version is taken into account during the merge.
func (h *HelmController) composeValues(ctx context.Context, hr v2.HelmRelease) (chartutil.Values, error) {
	result := chartutil.Values{}

	configMaps := make(map[string]*corev1.ConfigMap)
	secrets := make(map[string]*corev1.Secret)

	for _, v := range hr.Spec.ValuesFrom {
		namespacedName := types.NamespacedName{Namespace: hr.Namespace, Name: v.Name}
		var valuesData []byte

		switch v.Kind {
		case "ConfigMap":
			resource, ok := configMaps[namespacedName.String()]
			if !ok {
				// The resource may not exist, but we want to act on a single version
				// of the resource in case the values reference is marked as optional.
				configMaps[namespacedName.String()] = nil

				resource = &corev1.ConfigMap{}
				if err := h.Get(ctx, namespacedName, resource); err != nil {
					if apierrors.IsNotFound(err) {
						if v.Optional {
							(ctrl.LoggerFrom(ctx)).
								Info(fmt.Sprintf("could not find optional %s '%s'", v.Kind, namespacedName))
							continue
						}
						return nil, fmt.Errorf("could not find %s '%s'", v.Kind, namespacedName)
					}
					return nil, err
				}
				configMaps[namespacedName.String()] = resource
			}
			if resource == nil {
				if v.Optional {
					(ctrl.LoggerFrom(ctx)).Info(fmt.Sprintf("could not find optional %s '%s'", v.Kind, namespacedName))
					continue
				}
				return nil, fmt.Errorf("could not find %s '%s'", v.Kind, namespacedName)
			}
			if data, ok := resource.Data[v.GetValuesKey()]; !ok {
				return nil, fmt.Errorf("missing key '%s' in %s '%s'", v.GetValuesKey(), v.Kind, namespacedName)
			} else {
				valuesData = []byte(data)
			}
		case "Secret":
			resource, ok := secrets[namespacedName.String()]
			if !ok {
				// The resource may not exist, but we want to act on a single version
				// of the resource in case the values reference is marked as optional.
				secrets[namespacedName.String()] = nil

				resource = &corev1.Secret{}
				if err := h.Get(ctx, namespacedName, resource); err != nil {
					if apierrors.IsNotFound(err) {
						if v.Optional {
							(ctrl.LoggerFrom(ctx)).
								Info(fmt.Sprintf("could not find optional %s '%s'", v.Kind, namespacedName))
							continue
						}
						return nil, fmt.Errorf("could not find %s '%s'", v.Kind, namespacedName)
					}
					return nil, err
				}
				secrets[namespacedName.String()] = resource
			}
			if resource == nil {
				if v.Optional {
					(ctrl.LoggerFrom(ctx)).Info(fmt.Sprintf("could not find optional %s '%s'", v.Kind, namespacedName))
					continue
				}
				return nil, fmt.Errorf("could not find %s '%s'", v.Kind, namespacedName)
			}
			if data, ok := resource.Data[v.GetValuesKey()]; !ok {
				return nil, fmt.Errorf("missing key '%s' in %s '%s'", v.GetValuesKey(), v.Kind, namespacedName)
			} else {
				valuesData = data
			}
		default:
			return nil, fmt.Errorf("unsupported ValuesReference kind '%s'", v.Kind)
		}
		switch v.TargetPath {
		case "":
			values, err := chartutil.ReadValues(valuesData)
			if err != nil {
				return nil, fmt.Errorf("unable to read values from key '%s' in %s '%s': %w", v.GetValuesKey(), v.Kind, namespacedName, err)
			}
			result = transform.MergeMaps(result, values)
		default:
			// TODO(hidde): this is a bit of hack, as it mimics the way the option string is passed
			// 	to Helm from a CLI perspective. Given the parser is however not publicly accessible
			// 	while it contains all logic around parsing the target path, it is a fair trade-off.
			stringValuesData := string(valuesData)
			const singleQuote = "'"
			const doubleQuote = "\""
			var err error
			if (strings.HasPrefix(stringValuesData, singleQuote) && strings.HasSuffix(stringValuesData, singleQuote)) || (strings.HasPrefix(stringValuesData, doubleQuote) && strings.HasSuffix(stringValuesData, doubleQuote)) {
				stringValuesData = strings.Trim(stringValuesData, singleQuote+doubleQuote)
				singleValue := v.TargetPath + "=" + stringValuesData
				err = strvals.ParseIntoString(singleValue, result)
			} else {
				singleValue := v.TargetPath + "=" + stringValuesData
				err = strvals.ParseInto(singleValue, result)
			}
			if err != nil {
				return nil, fmt.Errorf("unable to merge value from key '%s' in %s '%s' into target path '%s': %w", v.GetValuesKey(), v.Kind, namespacedName, v.TargetPath, err)
			}
		}
	}
	return transform.MergeMaps(result, hr.GetValues()), nil
}
