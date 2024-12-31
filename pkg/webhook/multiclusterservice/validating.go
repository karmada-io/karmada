/*
Copyright 2023 The Karmada Authors.

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

package multiclusterservice

import (
	"context"
	"net/http"
	"reflect"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	clustervalidation "github.com/karmada-io/karmada/pkg/apis/cluster/validation"
	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

// ValidatingAdmission validates MultiClusterService object when creating/updating.
type ValidatingAdmission struct {
	Decoder admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	mcs := &networkingv1alpha1.MultiClusterService{}
	err := v.Decoder.Decode(req, mcs)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.Infof("Validating MultiClusterService(%s/%s) for request: %s", mcs.Namespace, mcs.Name, req.Operation)

	if req.Operation == admissionv1.Update {
		oldMcs := &networkingv1alpha1.MultiClusterService{}
		err = v.Decoder.DecodeRaw(req.OldObject, oldMcs)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if errs := v.validateMCSUpdate(oldMcs, mcs); len(errs) != 0 {
			klog.Errorf("Validating MultiClusterServiceUpdate failed: %v", errs)
			return admission.Denied(errs.ToAggregate().Error())
		}
	} else {
		if errs := v.validateMCS(mcs); len(errs) != 0 {
			klog.Errorf("Validating MultiClusterService failed: %v", errs)
			return admission.Denied(errs.ToAggregate().Error())
		}
	}
	return admission.Allowed("")
}

func (v *ValidatingAdmission) validateMCSUpdate(oldMcs, newMcs *networkingv1alpha1.MultiClusterService) field.ErrorList {
	allErrs := apimachineryvalidation.ValidateObjectMetaUpdate(&newMcs.ObjectMeta, &oldMcs.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, v.validateMCSTypesUpdate(oldMcs, newMcs)...)
	allErrs = append(allErrs, v.validateMCS(newMcs)...)
	allErrs = append(allErrs, lifted.ValidateLoadBalancerStatus(&newMcs.Status.LoadBalancer, field.NewPath("status", "loadBalancer"))...)
	return allErrs
}

func (v *ValidatingAdmission) validateMCSTypesUpdate(oldMcs, newMcs *networkingv1alpha1.MultiClusterService) field.ErrorList {
	allErrs := field.ErrorList{}
	if !reflect.DeepEqual(oldMcs.Spec.Types, newMcs.Spec.Types) {
		typePath := field.NewPath("spec").Child("types")
		allErrs = append(allErrs, field.Invalid(typePath, oldMcs.Spec.Types, "MultiClusterService types are immutable"))
	}
	return allErrs
}

func (v *ValidatingAdmission) validateMCS(mcs *networkingv1alpha1.MultiClusterService) field.ErrorList {
	allErrs := apimachineryvalidation.ValidateObjectMeta(&mcs.ObjectMeta, true,
		apimachineryvalidation.NameIsDNS1035Label, field.NewPath("metadata"))
	allErrs = append(allErrs, v.validateMultiClusterServiceSpec(mcs)...)
	return allErrs
}

// validateMultiClusterServiceSpec validates MultiClusterService spec.
func (v *ValidatingAdmission) validateMultiClusterServiceSpec(mcs *networkingv1alpha1.MultiClusterService) field.ErrorList {
	allErrs := field.ErrorList{}
	specPath := field.NewPath("spec")
	allPortNames := sets.New[string]()

	portsPath := specPath.Child("ports")
	for i := range mcs.Spec.Ports {
		portPath := portsPath.Index(i)
		port := mcs.Spec.Ports[i]
		allErrs = append(allErrs, v.validateExposurePort(&port, allPortNames, portPath)...)
	}

	typesSet := sets.Set[string]{}
	typesPath := specPath.Child("types")
	for i := range mcs.Spec.Types {
		typePath := typesPath.Index(i)
		exposureType := mcs.Spec.Types[i]
		typesSet.Insert(string(exposureType))
		allErrs = append(allErrs, v.validateExposureType(&exposureType, typePath)...)
	}
	if len(typesSet) > 1 {
		allErrs = append(allErrs, field.Invalid(typesPath, mcs.Spec.Types, "MultiClusterService types should not contain more than one type"))
	}

	clusterNamesPath := specPath.Child("range").Child("providerClusters")
	for i := range mcs.Spec.ProviderClusters {
		clusterNamePath := clusterNamesPath.Index(i)
		clusterName := mcs.Spec.ProviderClusters[i].Name
		if errMegs := clustervalidation.ValidateClusterName(clusterName); len(errMegs) > 0 {
			allErrs = append(allErrs, field.Invalid(clusterNamePath, clusterName, strings.Join(errMegs, ",")))
		}
	}

	clusterNamesPath = specPath.Child("range").Child("consumerClusters")
	for i := range mcs.Spec.ConsumerClusters {
		clusterNamePath := clusterNamesPath.Index(i)
		clusterName := mcs.Spec.ConsumerClusters[i].Name
		if errMegs := clustervalidation.ValidateClusterName(clusterName); len(errMegs) > 0 {
			allErrs = append(allErrs, field.Invalid(clusterNamePath, clusterName, strings.Join(errMegs, ",")))
		}
	}
	return allErrs
}

// validateExposurePort validates MultiClusterService ExposurePort.
func (v *ValidatingAdmission) validateExposurePort(ep *networkingv1alpha1.ExposurePort, allNames sets.Set[string], fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(ep.Name) != 0 {
		allErrs = append(allErrs, lifted.ValidateDNS1123Label(ep.Name, fldPath.Child("name"))...)
		if allNames.Has(ep.Name) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Child("name"), ep.Name))
		} else {
			allNames.Insert(ep.Name)
		}
	}
	for _, msg := range validation.IsValidPortNum(int(ep.Port)) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("port"), ep.Port, msg))
	}
	return allErrs
}

// validateExposureType validates MultiClusterService ExposureType.
func (v *ValidatingAdmission) validateExposureType(et *networkingv1alpha1.ExposureType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if *et != networkingv1alpha1.ExposureTypeCrossCluster && *et != networkingv1alpha1.ExposureTypeLoadBalancer {
		msg := "ExposureType Error"
		allErrs = append(allErrs, field.Invalid(fldPath, *et, msg))
	}
	return allErrs
}
