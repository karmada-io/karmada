package helper

import (
	"context"

	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// CreateOrUpdateEndpointSlice creates a EndpointSlice object if not exist, or updates if it already exist.
func CreateOrUpdateEndpointSlice(client client.Client, endpointSlice *discoveryv1.EndpointSlice) error {
	runtimeObject := endpointSlice.DeepCopy()
	var operationResult controllerutil.OperationResult
	err := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		operationResult, err = controllerutil.CreateOrUpdate(context.TODO(), client, runtimeObject, func() error {
			runtimeObject.AddressType = endpointSlice.AddressType
			runtimeObject.Endpoints = endpointSlice.Endpoints
			runtimeObject.Labels = endpointSlice.Labels
			runtimeObject.Ports = endpointSlice.Ports
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		klog.Errorf("Failed to create/update EndpointSlice %s/%s. Error: %v", endpointSlice.GetNamespace(), endpointSlice.GetName(), err)
		return err
	}

	if operationResult == controllerutil.OperationResultCreated {
		klog.V(2).Infof("Create EndpointSlice %s/%s successfully.", endpointSlice.GetNamespace(), endpointSlice.GetName())
	} else if operationResult == controllerutil.OperationResultUpdated {
		klog.V(2).Infof("Update EndpointSlice %s/%s successfully.", endpointSlice.GetNamespace(), endpointSlice.GetName())
	} else {
		klog.V(2).Infof("EndpointSlice %s/%s is up to date.", endpointSlice.GetNamespace(), endpointSlice.GetName())
	}

	return nil
}

// GetEndpointSlices returns a EndpointSliceList by labels
func GetEndpointSlices(c client.Client, ls labels.Set) (*discoveryv1.EndpointSliceList, error) {
	endpointSlices := &discoveryv1.EndpointSliceList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(ls)}

	return endpointSlices, c.List(context.TODO(), endpointSlices, listOpt)
}

// DeleteEndpointSlice will delete all EndpointSlice objects by labels.
func DeleteEndpointSlice(c client.Client, selector labels.Set) error {
	endpointSliceList, err := GetEndpointSlices(c, selector)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("Failed to get endpointslices by label %v: %v", selector, err)
		return err
	}

	var errs []error
	for index, work := range endpointSliceList.Items {
		if err := c.Delete(context.TODO(), &endpointSliceList.Items[index]); err != nil {
			klog.Errorf("Failed to delete endpointslice(%s/%s): %v", work.Namespace, work.Name, err)
			errs = append(errs, err)
		}
	}

	return errors.NewAggregate(errs)
}
