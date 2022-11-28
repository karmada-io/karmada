package genericresource

import (
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// Result contains helper methods for dealing with the outcome of a Builder.
type Result struct {
	err     error
	visitor Visitor

	// populated by a call to Infos
	info []*Info

	ignoreErrors []utilerrors.Matcher
}

// Err returns one or more errors (via a util.ErrorList) that occurred prior
// to visiting the elements in the visitor. To see all errors including those
// that occur during visitation, invoke Infos().
func (r *Result) Err() error {
	return r.err
}

// Objects returns the list of objects of all found resources.
func (r *Result) Objects() ([]interface{}, error) {
	infos, err := r.Infos()
	if err != nil {
		return nil, err
	}

	objects := make([]interface{}, len(infos))
	for i, info := range infos {
		objects[i] = info.Object
	}
	return objects, err
}

// Infos returns an array of all of the resource infos retrieved via traversal.
// Will attempt to traverse the entire set of visitors only once, and will return
// a cached list on subsequent calls.
func (r *Result) Infos() ([]*Info, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.info != nil {
		return r.info, nil
	}

	var infos []*Info
	err := r.visitor.Visit(func(info *Info, err error) error {
		if err != nil {
			return err
		}
		infos = append(infos, info)
		return nil
	})
	err = utilerrors.FilterOut(err, r.ignoreErrors...)

	r.info, r.err = infos, err
	return infos, err
}

// Visit implements the Visitor interface on the items described in the Builder.
// Note that some visitor sources are not traversable more than once, or may
// return different results.  If you wish to operate on the same set of resources
// multiple times, use the Infos() method.
func (r *Result) Visit(fn VisitorFunc) error {
	if r.err != nil {
		return r.err
	}
	err := r.visitor.Visit(fn)
	return utilerrors.FilterOut(err, r.ignoreErrors...)
}
