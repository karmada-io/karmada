/*
Copyright 2021 The Flux authors

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

// This file is lifted by fluxcd/helm-controller. DO NOT CHANGE.

package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/postrender"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	apiextension "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/klog/v2"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v2 "github.com/fluxcd/helm-controller/api/v2beta1"
)

const (
	HelmDriver = "secret"
)

// Runner represents a Helm action runner capable of performing Helm
// operations for a v2beta1.HelmRelease.
type Runner struct {
	mu     sync.Mutex
	config *action.Configuration
}

// NewRunner constructs a new Runner configured to run Helm actions with the
// given genericclioptions.RESTClientGetter, and the release and storage
// namespace configured to the provided values.
func NewRunner(getter genericclioptions.RESTClientGetter, storageNamespace string) (*Runner, error) {
	runner := &Runner{}
	runner.config = new(action.Configuration)
	if err := runner.config.Init(getter, storageNamespace, HelmDriver, klog.V(6).Infof); err != nil {
		return nil, err
	}
	return runner, nil
}

// Create post renderer instances from HelmRelease and combine them into
// a single combined post renderer.
func postRenderers(hr v2.HelmRelease) (postrender.PostRenderer, error) {
	var combinedRenderer = newCombinedPostRenderer()
	for _, r := range hr.Spec.PostRenderers {
		if r.Kustomize != nil {
			combinedRenderer.addRenderer(newPostRendererKustomize(r.Kustomize))
		}
	}
	combinedRenderer.addRenderer(newPostRendererOriginLabels(&hr))
	if len(combinedRenderer.renderers) == 0 {
		return nil, nil
	}
	return &combinedRenderer, nil
}

// Install runs an Helm install action for the given v2beta1.HelmRelease.
func (r *Runner) Install(hr v2.HelmRelease, chart *chart.Chart, values chartutil.Values) (*release.Release, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	install := action.NewInstall(r.config)
	install.ReleaseName = hr.GetReleaseName()
	install.Namespace = hr.GetReleaseNamespace()
	install.Timeout = hr.Spec.GetInstall().GetTimeout(hr.GetTimeout()).Duration
	install.Wait = !hr.Spec.GetInstall().DisableWait
	install.WaitForJobs = !hr.Spec.GetInstall().DisableWaitForJobs
	install.DisableHooks = hr.Spec.GetInstall().DisableHooks
	install.DisableOpenAPIValidation = hr.Spec.GetInstall().DisableOpenAPIValidation
	install.Replace = hr.Spec.GetInstall().Replace
	var legacyCRDsPolicy = v2.Create
	if hr.Spec.GetInstall().SkipCRDs {
		legacyCRDsPolicy = v2.Skip
	}
	cRDsPolicy, err := r.validateCRDsPolicy(hr.Spec.GetInstall().CRDs, legacyCRDsPolicy)
	if err != nil {
		return nil, err
	}
	if cRDsPolicy == v2.Skip || cRDsPolicy == v2.CreateReplace {
		install.SkipCRDs = true
	}
	install.Devel = true
	renderer, err := postRenderers(hr)
	if err != nil {
		return nil, err
	}
	install.PostRenderer = renderer
	if hr.Spec.TargetNamespace != "" {
		install.CreateNamespace = hr.Spec.GetInstall().CreateNamespace
	}

	if cRDsPolicy == v2.CreateReplace {
		crds := chart.CRDObjects()
		if len(crds) > 0 {
			if err := r.applyCRDs(cRDsPolicy, hr, chart); err != nil {
				return nil, err
			}
		}
	}

	rel, err := install.Run(chart, values.AsMap())
	return rel, err
}

// Upgrade runs an Helm upgrade action for the given v2beta1.HelmRelease.
func (r *Runner) Upgrade(hr v2.HelmRelease, chart *chart.Chart, values chartutil.Values) (*release.Release, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	upgrade := action.NewUpgrade(r.config)
	upgrade.Namespace = hr.GetReleaseNamespace()
	upgrade.ResetValues = !hr.Spec.GetUpgrade().PreserveValues
	upgrade.ReuseValues = hr.Spec.GetUpgrade().PreserveValues
	upgrade.MaxHistory = hr.GetMaxHistory()
	upgrade.Timeout = hr.Spec.GetUpgrade().GetTimeout(hr.GetTimeout()).Duration
	upgrade.Wait = !hr.Spec.GetUpgrade().DisableWait
	upgrade.WaitForJobs = !hr.Spec.GetUpgrade().DisableWaitForJobs
	upgrade.DisableHooks = hr.Spec.GetUpgrade().DisableHooks
	upgrade.Force = hr.Spec.GetUpgrade().Force
	upgrade.CleanupOnFail = hr.Spec.GetUpgrade().CleanupOnFail
	upgrade.Devel = true
	renderer, err := postRenderers(hr)
	if err != nil {
		return nil, err
	}
	upgrade.PostRenderer = renderer
	// If user opted-in to upgrade CRDs, upgrade them first.
	cRDsPolicy, err := r.validateCRDsPolicy(hr.Spec.GetUpgrade().CRDs, v2.Skip)
	if err != nil {
		return nil, err
	}
	if cRDsPolicy != v2.Skip {
		crds := chart.CRDObjects()
		if len(crds) > 0 {
			if err := r.applyCRDs(cRDsPolicy, hr, chart); err != nil {
				return nil, err
			}
		}
	}
	rel, err := upgrade.Run(hr.GetReleaseName(), chart, values.AsMap())
	return rel, err
}

func (r *Runner) validateCRDsPolicy(policy v2.CRDsPolicy, defaultValue v2.CRDsPolicy) (v2.CRDsPolicy, error) {
	switch policy {
	case "":
		return defaultValue, nil
	case v2.Skip:
		break
	case v2.Create:
		break
	case v2.CreateReplace:
		break
	default:
		return policy, fmt.Errorf("invalid CRD upgrade policy '%s' defined in field upgradeCRDs, valid values are '%s', '%s' or '%s'",
			policy, v2.Skip, v2.Create, v2.CreateReplace,
		)
	}
	return policy, nil
}

type rootScoped struct{}

func (*rootScoped) Name() meta.RESTScopeName {
	return meta.RESTScopeNameRoot
}

// This has been adapted from https://github.com/helm/helm/blob/v3.5.4/pkg/action/install.go#L127
func (r *Runner) applyCRDs(policy v2.CRDsPolicy, hr v2.HelmRelease, chart *chart.Chart) error {
	cfg := r.config
	cfg.Log("apply CRDs with policy %s", policy)
	// Collect all CRDs from all files in `crds` directory.
	allCrds := make(kube.ResourceList, 0)
	for _, obj := range chart.CRDObjects() {
		// Read in the resources
		res, err := cfg.KubeClient.Build(bytes.NewBuffer(obj.File.Data), false)
		if err != nil {
			cfg.Log("failed to parse CRDs from %s: %s", obj.Name, err)
			return errors.New(fmt.Sprintf("failed to parse CRDs from %s: %s", obj.Name, err))
		}
		allCrds = append(allCrds, res...)
	}
	totalItems := []*resource.Info{}
	switch policy {
	case v2.Skip:
		break
	case v2.Create:
		for i := range allCrds {
			if rr, err := cfg.KubeClient.Create(allCrds[i : i+1]); err != nil {
				crdName := allCrds[i].Name
				// If the error is CRD already exists, continue.
				if apierrors.IsAlreadyExists(err) {
					cfg.Log("CRD %s is already present. Skipping.", crdName)
					if rr != nil && rr.Created != nil {
						totalItems = append(totalItems, rr.Created...)
					}
					continue
				}
				cfg.Log("failed to create CRD %s: %s", crdName, err)
				return errors.New(fmt.Sprintf("failed to create CRD %s: %s", crdName, err))
			} else {
				if rr != nil && rr.Created != nil {
					totalItems = append(totalItems, rr.Created...)
				}
			}
		}
		break
	case v2.CreateReplace:
		config, err := r.config.RESTClientGetter.ToRESTConfig()
		if err != nil {
			klog.V(10).Infof("Error while creating Kubernetes client config: %s", err)
			return err
		}
		clientset, err := apiextension.NewForConfig(config)
		if err != nil {
			klog.V(10).Infof("Error while creating Kubernetes clientset for apiextension: %s", err)
			return err
		}
		client := clientset.ApiextensionsV1().CustomResourceDefinitions()
		original := make(kube.ResourceList, 0)
		// Note, we build the originals from the current set of CRDs
		// and therefore this upgrade will never delete CRDs that existed in the former release
		// but no longer exist in the current release.
		for _, r := range allCrds {
			if o, err := client.Get(context.TODO(), r.Name, v1.GetOptions{}); err == nil && o != nil {
				o.GetResourceVersion()
				original = append(original, &resource.Info{
					Client: clientset.ApiextensionsV1().RESTClient(),
					Mapping: &meta.RESTMapping{
						Resource: schema.GroupVersionResource{
							Group:    "apiextensions.k8s.io",
							Version:  r.Mapping.GroupVersionKind.Version,
							Resource: "customresourcedefinition",
						},
						GroupVersionKind: schema.GroupVersionKind{
							Kind:    "CustomResourceDefinition",
							Group:   "apiextensions.k8s.io",
							Version: r.Mapping.GroupVersionKind.Version,
						},
						Scope: &rootScoped{},
					},
					Namespace:       o.ObjectMeta.Namespace,
					Name:            o.ObjectMeta.Name,
					Object:          o,
					ResourceVersion: o.ObjectMeta.ResourceVersion,
				})
			} else if !apierrors.IsNotFound(err) {
				cfg.Log("failed to get CRD %s: %s", r.Name, err)
				return err
			}
		}
		// Send them to Kube
		if rr, err := cfg.KubeClient.Update(original, allCrds, true); err != nil {
			cfg.Log("failed to apply CRD %s", err)
			return errors.New(fmt.Sprintf("failed to apply CRD %s", err))
		} else {
			if rr != nil {
				if rr.Created != nil {
					totalItems = append(totalItems, rr.Created...)
				}
				if rr.Updated != nil {
					totalItems = append(totalItems, rr.Updated...)
				}
				if rr.Deleted != nil {
					totalItems = append(totalItems, rr.Deleted...)
				}
			}
		}
		break
	}
	if len(totalItems) > 0 {
		// Invalidate the local cache, since it will not have the new CRDs
		// present.
		discoveryClient, err := cfg.RESTClientGetter.ToDiscoveryClient()
		if err != nil {
			cfg.Log("Error in cfg.RESTClientGetter.ToDiscoveryClient(): %s", err)
			return err
		}
		cfg.Log("Clearing discovery cache")
		discoveryClient.Invalidate()
		// Give time for the CRD to be recognized.
		if err := cfg.KubeClient.Wait(totalItems, 60*time.Second); err != nil {
			cfg.Log("Error waiting for items: %s", err)
			return err
		}
		// Make sure to force a rebuild of the cache.
		discoveryClient.ServerGroups()
	}
	return nil
}

// Test runs an Helm test action for the given v2beta1.HelmRelease.
func (r *Runner) Test(hr v2.HelmRelease) (*release.Release, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	test := action.NewReleaseTesting(r.config)
	test.Namespace = hr.GetReleaseNamespace()
	test.Timeout = hr.Spec.GetTest().GetTimeout(hr.GetTimeout()).Duration

	rel, err := test.Run(hr.GetReleaseName())
	return rel, err
}

// Rollback runs an Helm rollback action for the given v2beta1.HelmRelease.
func (r *Runner) Rollback(hr v2.HelmRelease) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rollback := action.NewRollback(r.config)
	rollback.Timeout = hr.Spec.GetRollback().GetTimeout(hr.GetTimeout()).Duration
	rollback.Wait = !hr.Spec.GetRollback().DisableWait
	rollback.WaitForJobs = !hr.Spec.GetRollback().DisableWaitForJobs
	rollback.DisableHooks = hr.Spec.GetRollback().DisableHooks
	rollback.Force = hr.Spec.GetRollback().Force
	rollback.Recreate = hr.Spec.GetRollback().Recreate
	rollback.CleanupOnFail = hr.Spec.GetRollback().CleanupOnFail

	err := rollback.Run(hr.GetReleaseName())
	return err
}

// Uninstall runs an Helm uninstall action for the given v2beta1.HelmRelease.
func (r *Runner) Uninstall(hr v2.HelmRelease) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	uninstall := action.NewUninstall(r.config)
	uninstall.Timeout = hr.Spec.GetUninstall().GetTimeout(hr.GetTimeout()).Duration
	uninstall.DisableHooks = hr.Spec.GetUninstall().DisableHooks
	uninstall.KeepHistory = hr.Spec.GetUninstall().KeepHistory
	uninstall.Wait = !hr.Spec.GetUninstall().DisableWait

	_, err := uninstall.Run(hr.GetReleaseName())
	return err
}

// ObserveLastRelease observes the last revision, if there is one,
// for the actual Helm release associated with the given v2beta1.HelmRelease.
func (r *Runner) ObserveLastRelease(hr v2.HelmRelease) (*release.Release, error) {
	rel, err := r.config.Releases.Last(hr.GetReleaseName())
	if err != nil && errors.Is(err, driver.ErrReleaseNotFound) {
		err = nil
	}
	return rel, err
}
