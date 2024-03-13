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

package hpascaletargetmarker

import (
	"context"
	"fmt"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

type labelEventKind int

const (
	// addLabelEvent refer to adding util.RetainReplicasLabel to resource scaled by HPA
	addLabelEvent labelEventKind = iota
	// deleteLabelEvent refer to deleting util.RetainReplicasLabel from resource scaled by HPA
	deleteLabelEvent
)

type labelEvent struct {
	kind labelEventKind
	hpa  *autoscalingv2.HorizontalPodAutoscaler
}

func (r *HpaScaleTargetMarker) reconcileScaleRef(key util.QueueKey) (err error) {
	event, ok := key.(labelEvent)
	if !ok {
		klog.Errorf("Found invalid key when reconciling hpa scale ref: %+v", key)
		return nil
	}

	switch event.kind {
	case addLabelEvent:
		err = r.addHPALabelToScaleRef(context.TODO(), event.hpa)
	case deleteLabelEvent:
		err = r.deleteHPALabelFromScaleRef(context.TODO(), event.hpa)
	default:
		klog.Errorf("Found invalid key when reconciling hpa scale ref: %+v", key)
		return nil
	}

	if err != nil {
		klog.Errorf("reconcile scale ref failed: %+v", err)
	}
	return err
}

func (r *HpaScaleTargetMarker) addHPALabelToScaleRef(ctx context.Context, hpa *autoscalingv2.HorizontalPodAutoscaler) error {
	targetGVK := schema.FromAPIVersionAndKind(hpa.Spec.ScaleTargetRef.APIVersion, hpa.Spec.ScaleTargetRef.Kind)
	mapping, err := r.RESTMapper.RESTMapping(targetGVK.GroupKind(), targetGVK.Version)
	if err != nil {
		return fmt.Errorf("unable to recognize scale ref resource, %s/%v, err: %+v", hpa.Namespace, hpa.Spec.ScaleTargetRef, err)
	}

	scaleRef, err := r.DynamicClient.Resource(mapping.Resource).Namespace(hpa.Namespace).Get(ctx, hpa.Spec.ScaleTargetRef.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("scale ref resource is not found (%s/%v), skip processing", hpa.Namespace, hpa.Spec.ScaleTargetRef)
			return nil
		}
		return fmt.Errorf("failed to find scale ref resource (%s/%v), err: %+v", hpa.Namespace, hpa.Spec.ScaleTargetRef, err)
	}

	// use patch is better than update, when modification occur after get, patch can still success while update can not
	newScaleRef := scaleRef.DeepCopy()
	util.MergeLabel(newScaleRef, util.RetainReplicasLabel, util.RetainReplicasValue)
	patchBytes, err := helper.GenMergePatch(scaleRef, newScaleRef)
	if err != nil {
		return fmt.Errorf("failed to gen merge patch (%s/%v), err: %+v", hpa.Namespace, hpa.Spec.ScaleTargetRef, err)
	}
	if len(patchBytes) == 0 {
		klog.Infof("hpa labels already exist, skip adding (%s/%v)", hpa.Namespace, hpa.Spec.ScaleTargetRef)
		return nil
	}

	_, err = r.DynamicClient.Resource(mapping.Resource).Namespace(newScaleRef.GetNamespace()).
		Patch(ctx, newScaleRef.GetName(), types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("scale ref resource is not found (%s/%v), skip processing", hpa.Namespace, hpa.Spec.ScaleTargetRef)
			return nil
		}
		return fmt.Errorf("failed to patch scale ref resource (%s/%v), err: %+v", hpa.Namespace, hpa.Spec.ScaleTargetRef, err)
	}

	klog.Infof("add hpa labels to %s/%v success", hpa.Namespace, hpa.Spec.ScaleTargetRef)
	return nil
}

func (r *HpaScaleTargetMarker) deleteHPALabelFromScaleRef(ctx context.Context, hpa *autoscalingv2.HorizontalPodAutoscaler) error {
	targetGVK := schema.FromAPIVersionAndKind(hpa.Spec.ScaleTargetRef.APIVersion, hpa.Spec.ScaleTargetRef.Kind)
	mapping, err := r.RESTMapper.RESTMapping(targetGVK.GroupKind(), targetGVK.Version)
	if err != nil {
		return fmt.Errorf("unable to recognize scale ref resource, %s/%v, err: %+v", hpa.Namespace, hpa.Spec.ScaleTargetRef, err)
	}

	scaleRef, err := r.DynamicClient.Resource(mapping.Resource).Namespace(hpa.Namespace).Get(ctx, hpa.Spec.ScaleTargetRef.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("scale ref resource is not found (%s/%v), skip processing", hpa.Namespace, hpa.Spec.ScaleTargetRef)
			return nil
		}
		return fmt.Errorf("failed to find scale ref resource (%s/%v), err: %+v", hpa.Namespace, hpa.Spec.ScaleTargetRef, err)
	}

	// use patch is better than update, when modification occur after get, patch can still success while update can not
	newScaleRef := scaleRef.DeepCopy()
	util.RemoveLabels(newScaleRef, util.RetainReplicasLabel)
	patchBytes, err := helper.GenMergePatch(scaleRef, newScaleRef)
	if err != nil {
		return fmt.Errorf("failed to gen merge patch (%s/%v), err: %+v", hpa.Namespace, hpa.Spec.ScaleTargetRef, err)
	}
	if len(patchBytes) == 0 {
		klog.Infof("hpa labels not exist, skip deleting (%s/%v)", hpa.Namespace, hpa.Spec.ScaleTargetRef)
		return nil
	}

	_, err = r.DynamicClient.Resource(mapping.Resource).Namespace(newScaleRef.GetNamespace()).
		Patch(ctx, newScaleRef.GetName(), types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("scale ref resource is not found (%s/%v), skip processing", hpa.Namespace, hpa.Spec.ScaleTargetRef)
			return nil
		}
		return fmt.Errorf("failed to patch scale ref resource (%s/%v), err: %+v", hpa.Namespace, hpa.Spec.ScaleTargetRef, err)
	}

	klog.Infof("delete hpa labels from %s/%+v success", hpa.Namespace, hpa.Spec.ScaleTargetRef)
	return nil
}
