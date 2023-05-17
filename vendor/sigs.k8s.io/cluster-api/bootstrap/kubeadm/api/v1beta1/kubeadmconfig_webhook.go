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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"sigs.k8s.io/cluster-api/feature"
)

var (
	cannotUseWithIgnition                            = fmt.Sprintf("not supported when spec.format is set to %q", Ignition)
	conflictingFileSourceMsg                         = "only one of content or contentFrom may be specified for a single file"
	conflictingUserSourceMsg                         = "only one of passwd or passwdFrom may be specified for a single user"
	kubeadmBootstrapFormatIgnitionFeatureDisabledMsg = "can be set only if the KubeadmBootstrapFormatIgnition feature gate is enabled"
	missingSecretNameMsg                             = "secret file source must specify non-empty secret name"
	missingSecretKeyMsg                              = "secret file source must specify non-empty secret key"
	pathConflictMsg                                  = "path property must be unique among all files"
)

func (c *KubeadmConfig) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/mutate-bootstrap-cluster-x-k8s-io-v1beta1-kubeadmconfig,mutating=true,failurePolicy=fail,groups=bootstrap.cluster.x-k8s.io,resources=kubeadmconfigs,versions=v1beta1,name=default.kubeadmconfig.bootstrap.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &KubeadmConfig{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (c *KubeadmConfig) Default() {
	DefaultKubeadmConfigSpec(&c.Spec)
}

// DefaultKubeadmConfigSpec defaults a KubeadmConfigSpec.
func DefaultKubeadmConfigSpec(r *KubeadmConfigSpec) {
	if r.Format == "" {
		r.Format = CloudConfig
	}
	if r.InitConfiguration != nil && r.InitConfiguration.NodeRegistration.ImagePullPolicy == "" {
		r.InitConfiguration.NodeRegistration.ImagePullPolicy = "IfNotPresent"
	}
	if r.JoinConfiguration != nil && r.JoinConfiguration.NodeRegistration.ImagePullPolicy == "" {
		r.JoinConfiguration.NodeRegistration.ImagePullPolicy = "IfNotPresent"
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-bootstrap-cluster-x-k8s-io-v1beta1-kubeadmconfig,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=bootstrap.cluster.x-k8s.io,resources=kubeadmconfigs,versions=v1beta1,name=validation.kubeadmconfig.bootstrap.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Validator = &KubeadmConfig{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (c *KubeadmConfig) ValidateCreate() error {
	return c.Spec.validate(c.Name)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (c *KubeadmConfig) ValidateUpdate(_ runtime.Object) error {
	return c.Spec.validate(c.Name)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (c *KubeadmConfig) ValidateDelete() error {
	return nil
}

func (c *KubeadmConfigSpec) validate(name string) error {
	allErrs := c.Validate(field.NewPath("spec"))

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("KubeadmConfig").GroupKind(), name, allErrs)
}

// Validate ensures the KubeadmConfigSpec is valid.
func (c *KubeadmConfigSpec) Validate(pathPrefix *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, c.validateFiles(pathPrefix)...)
	allErrs = append(allErrs, c.validateUsers(pathPrefix)...)
	allErrs = append(allErrs, c.validateIgnition(pathPrefix)...)

	return allErrs
}

func (c *KubeadmConfigSpec) validateFiles(pathPrefix *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	knownPaths := map[string]struct{}{}

	for i := range c.Files {
		file := c.Files[i]
		if file.Content != "" && file.ContentFrom != nil {
			allErrs = append(
				allErrs,
				field.Invalid(
					pathPrefix.Child("files").Index(i),
					file,
					conflictingFileSourceMsg,
				),
			)
		}
		// n.b.: if we ever add types besides Secret as a ContentFrom
		// Source, we must add webhook validation here for one of the
		// sources being non-nil.
		if file.ContentFrom != nil {
			if file.ContentFrom.Secret.Name == "" {
				allErrs = append(
					allErrs,
					field.Required(
						pathPrefix.Child("files").Index(i).Child("contentFrom", "secret", "name"),
						missingSecretNameMsg,
					),
				)
			}
			if file.ContentFrom.Secret.Key == "" {
				allErrs = append(
					allErrs,
					field.Required(
						pathPrefix.Child("files").Index(i).Child("contentFrom", "secret", "key"),
						missingSecretKeyMsg,
					),
				)
			}
		}
		_, conflict := knownPaths[file.Path]
		if conflict {
			allErrs = append(
				allErrs,
				field.Invalid(
					pathPrefix.Child("files").Index(i).Child("path"),
					file,
					pathConflictMsg,
				),
			)
		}
		knownPaths[file.Path] = struct{}{}
	}

	return allErrs
}

func (c *KubeadmConfigSpec) validateUsers(pathPrefix *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for i := range c.Users {
		user := c.Users[i]
		if user.Passwd != nil && user.PasswdFrom != nil {
			allErrs = append(
				allErrs,
				field.Invalid(
					pathPrefix.Child("users").Index(i),
					user,
					conflictingUserSourceMsg,
				),
			)
		}
		// n.b.: if we ever add types besides Secret as a PasswdFrom
		// Source, we must add webhook validation here for one of the
		// sources being non-nil.
		if user.PasswdFrom != nil {
			if user.PasswdFrom.Secret.Name == "" {
				allErrs = append(
					allErrs,
					field.Required(
						pathPrefix.Child("users").Index(i).Child("passwdFrom", "secret", "name"),
						missingSecretNameMsg,
					),
				)
			}
			if user.PasswdFrom.Secret.Key == "" {
				allErrs = append(
					allErrs,
					field.Required(
						pathPrefix.Child("users").Index(i).Child("passwdFrom", "secret", "key"),
						missingSecretKeyMsg,
					),
				)
			}
		}
	}

	return allErrs
}

func (c *KubeadmConfigSpec) validateIgnition(pathPrefix *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if !feature.Gates.Enabled(feature.KubeadmBootstrapFormatIgnition) {
		if c.Format == Ignition {
			allErrs = append(allErrs, field.Forbidden(
				pathPrefix.Child("format"), kubeadmBootstrapFormatIgnitionFeatureDisabledMsg))
		}

		if c.Ignition != nil {
			allErrs = append(allErrs, field.Forbidden(
				pathPrefix.Child("ignition"), kubeadmBootstrapFormatIgnitionFeatureDisabledMsg))
		}

		return allErrs
	}

	if c.Format != Ignition {
		if c.Ignition != nil {
			allErrs = append(
				allErrs,
				field.Invalid(
					pathPrefix.Child("format"),
					c.Format,
					fmt.Sprintf("must be set to %q if spec.ignition is set", Ignition),
				),
			)
		}

		return allErrs
	}

	for i, user := range c.Users {
		if user.Inactive != nil && *user.Inactive {
			allErrs = append(
				allErrs,
				field.Forbidden(
					pathPrefix.Child("users").Index(i).Child("inactive"),
					cannotUseWithIgnition,
				),
			)
		}
	}

	if c.UseExperimentalRetryJoin {
		allErrs = append(
			allErrs,
			field.Forbidden(
				pathPrefix.Child("useExperimentalRetryJoin"),
				cannotUseWithIgnition,
			),
		)
	}

	for i, file := range c.Files {
		if file.Encoding == Gzip || file.Encoding == GzipBase64 {
			allErrs = append(
				allErrs,
				field.Forbidden(
					pathPrefix.Child("files").Index(i).Child("encoding"),
					cannotUseWithIgnition,
				),
			)
		}
	}

	if c.DiskSetup == nil {
		return allErrs
	}

	for i, partition := range c.DiskSetup.Partitions {
		if partition.TableType != nil && *partition.TableType != "gpt" {
			allErrs = append(
				allErrs,
				field.Invalid(
					pathPrefix.Child("diskSetup", "partitions").Index(i).Child("tableType"),
					*partition.TableType,
					fmt.Sprintf(
						"only partition type %q is supported when spec.format is set to %q",
						"gpt",
						Ignition,
					),
				),
			)
		}
	}

	for i, fs := range c.DiskSetup.Filesystems {
		if fs.ReplaceFS != nil {
			allErrs = append(
				allErrs,
				field.Forbidden(
					pathPrefix.Child("diskSetup", "filesystems").Index(i).Child("replaceFS"),
					cannotUseWithIgnition,
				),
			)
		}

		if fs.Partition != nil {
			allErrs = append(
				allErrs,
				field.Forbidden(
					pathPrefix.Child("diskSetup", "filesystems").Index(i).Child("partition"),
					cannotUseWithIgnition,
				),
			)
		}
	}

	return allErrs
}
