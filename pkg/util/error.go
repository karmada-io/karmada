package util

import utilerrors "k8s.io/apimachinery/pkg/util/errors"

// AggregateErrors will receive all errors from the channel and stuff all non-nil errors
// into the returned Aggregate.
func AggregateErrors(ch <-chan error) error {
	var errs []error
	for {
		drained := false
		select {
		case err := <-ch:
			errs = append(errs, err)
		default:
			drained = true
		}
		if drained {
			break
		}
	}
	return utilerrors.NewAggregate(errs)
}
