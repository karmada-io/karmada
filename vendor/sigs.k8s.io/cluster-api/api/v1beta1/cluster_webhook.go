/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta1

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/util/version"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (c *Cluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-cluster-x-k8s-io-v1beta1-cluster,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=clusters,versions=v1beta1,name=validation.cluster.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-cluster-x-k8s-io-v1beta1-cluster,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=clusters,versions=v1beta1,name=default.cluster.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &Cluster{}
var _ webhook.Validator = &Cluster{}

// Default satisfies the defaulting webhook interface.
func (c *Cluster) Default() {
	if c.Spec.InfrastructureRef != nil && len(c.Spec.InfrastructureRef.Namespace) == 0 {
		c.Spec.InfrastructureRef.Namespace = c.Namespace
	}

	if c.Spec.ControlPlaneRef != nil && len(c.Spec.ControlPlaneRef.Namespace) == 0 {
		c.Spec.ControlPlaneRef.Namespace = c.Namespace
	}

	// If the Cluster uses a managed topology
	if c.Spec.Topology != nil {
		// tolerate version strings without a "v" prefix: prepend it if it's not there
		if !strings.HasPrefix(c.Spec.Topology.Version, "v") {
			c.Spec.Topology.Version = "v" + c.Spec.Topology.Version
		}
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (c *Cluster) ValidateCreate() error {
	return c.validate(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (c *Cluster) ValidateUpdate(old runtime.Object) error {
	oldCluster, ok := old.(*Cluster)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a Cluster but got a %T", old))
	}
	return c.validate(oldCluster)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (c *Cluster) ValidateDelete() error {
	return nil
}

func (c *Cluster) validate(old *Cluster) error {
	var allErrs field.ErrorList
	if c.Spec.InfrastructureRef != nil && c.Spec.InfrastructureRef.Namespace != c.Namespace {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "infrastructureRef", "namespace"),
				c.Spec.InfrastructureRef.Namespace,
				"must match metadata.namespace",
			),
		)
	}

	if c.Spec.ControlPlaneRef != nil && c.Spec.ControlPlaneRef.Namespace != c.Namespace {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "controlPlaneRef", "namespace"),
				c.Spec.ControlPlaneRef.Namespace,
				"must match metadata.namespace",
			),
		)
	}

	// Validate the managed topology, if defined.
	if c.Spec.Topology != nil {
		if topologyErrs := c.validateTopology(old); len(topologyErrs) > 0 {
			allErrs = append(allErrs, topologyErrs...)
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Cluster").GroupKind(), c.Name, allErrs)
}

func (c *Cluster) validateTopology(old *Cluster) field.ErrorList {
	// NOTE: ClusterClass and managed topologies are behind ClusterTopology feature gate flag; the web hook
	// must prevent the usage of Cluster.Topology in case the feature flag is disabled.
	if !feature.Gates.Enabled(feature.ClusterTopology) {
		return field.ErrorList{
			field.Forbidden(
				field.NewPath("spec", "topology"),
				"can be set only if the ClusterTopology feature flag is enabled",
			),
		}
	}

	var allErrs field.ErrorList

	// class should be defined.
	if len(c.Spec.Topology.Class) == 0 {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "topology", "class"),
				c.Spec.Topology.Class,
				"cannot be empty",
			),
		)
	}

	// version should be valid.
	if !version.KubeSemver.MatchString(c.Spec.Topology.Version) {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "topology", "version"),
				c.Spec.Topology.Version,
				"must be a valid semantic version",
			),
		)
	}

	// MachineDeployment names must be unique.
	if c.Spec.Topology.Workers != nil {
		names := sets.String{}
		for _, md := range c.Spec.Topology.Workers.MachineDeployments {
			if names.Has(md.Name) {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "topology", "workers", "machineDeployments"),
						md,
						fmt.Sprintf("MachineDeployment names should be unique. MachineDeployment with name %q is defined more than once.", md.Name),
					),
				)
			}
			names.Insert(md.Name)
		}
	}

	switch old {
	case nil: // On create
		// c.Spec.InfrastructureRef and c.Spec.ControlPlaneRef could not be set
		if c.Spec.InfrastructureRef != nil {
			allErrs = append(
				allErrs,
				field.Invalid(
					field.NewPath("spec", "infrastructureRef"),
					c.Spec.InfrastructureRef,
					"cannot be set when a Topology is defined",
				),
			)
		}
		if c.Spec.ControlPlaneRef != nil {
			allErrs = append(
				allErrs,
				field.Invalid(
					field.NewPath("spec", "controlPlaneRef"),
					c.Spec.ControlPlaneRef,
					"cannot be set when a Topology is defined",
				),
			)
		}
	default: // On update
		// Class could not be mutated.
		if c.Spec.Topology.Class != old.Spec.Topology.Class {
			allErrs = append(
				allErrs,
				field.Invalid(
					field.NewPath("spec", "topology", "class"),
					c.Spec.Topology.Class,
					"class cannot be changed",
				),
			)
		}

		// Version could only be increased.
		inVersion, err := semver.ParseTolerant(c.Spec.Topology.Version)
		if err != nil {
			allErrs = append(
				allErrs,
				field.Invalid(
					field.NewPath("spec", "topology", "version"),
					c.Spec.Topology.Version,
					"is not a valid version",
				),
			)
		}
		oldVersion, err := semver.ParseTolerant(old.Spec.Topology.Version)
		if err != nil {
			// NOTE: this should never happen. Nevertheless, handling this for extra caution.
			allErrs = append(
				allErrs,
				field.Invalid(
					field.NewPath("spec", "topology", "version"),
					c.Spec.Topology.Class,
					"cannot be compared with the old version",
				),
			)
		}
		if inVersion.NE(semver.Version{}) && oldVersion.NE(semver.Version{}) && version.Compare(inVersion, oldVersion, version.WithBuildTags()) == -1 {
			allErrs = append(
				allErrs,
				field.Invalid(
					field.NewPath("spec", "topology", "version"),
					c.Spec.Topology.Version,
					"cannot be decreased",
				),
			)
		}
	}

	return allErrs
}
