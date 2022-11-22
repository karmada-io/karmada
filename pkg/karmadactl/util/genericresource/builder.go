package genericresource

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/resource"
)

const defaultHTTPGetAttempts int = 3

var defaultNewFunc = func() interface{} {
	return map[string]interface{}{}
}

var errMissingResource = fmt.Errorf(`you must provide one or more resources`)

// Builder provides convenience functions for taking arguments and parameters
// from the command line and converting them to a list of resources to iterate
// over using the Visitor interface.
type Builder struct {
	errs       []error
	paths      []Visitor
	stdinInUse bool
	mapper     *mapper
	schema     resource.ContentValidator
}

// NewBuilder returns a Builder.
func NewBuilder() *Builder {
	return &Builder{
		mapper: &mapper{
			newFunc: defaultNewFunc,
		},
	}
}

// Schema set the schema to validate data in files.
func (b *Builder) Schema(schema resource.ContentValidator) *Builder {
	b.schema = schema
	return b
}

// Constructor tells wanted type of object.
func (b *Builder) Constructor(newFunc func() interface{}) *Builder {
	b.mapper.newFunc = newFunc
	return b
}

// Filename groups input in two categories: URLs and files (files, directories, STDIN)
func (b *Builder) Filename(recursive bool, filenames ...string) *Builder {
	for _, s := range filenames {
		switch {
		case s == "-":
			b.Stdin()
		case strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://"):
			u, err := url.Parse(s)
			if err != nil {
				b.errs = append(b.errs, fmt.Errorf("the URL passed to filename %q is not valid: %v", s, err))
				continue
			}
			b.URL(defaultHTTPGetAttempts, u)
		default:
			matches, err := expandIfFilePattern(s)
			if err != nil {
				b.errs = append(b.errs, err)
				continue
			}
			b.Path(recursive, matches...)
		}
	}
	return b
}

// Stdin will read objects from the standard input.
func (b *Builder) Stdin() *Builder {
	if b.stdinInUse {
		b.errs = append(b.errs, resource.StdinMultiUseError)
	}
	b.stdinInUse = true
	b.paths = append(b.paths, FileVisitorForSTDIN(b.mapper, b.schema))
	return b
}

// URL accepts a number of URLs directly.
func (b *Builder) URL(httpAttemptCount int, urls ...*url.URL) *Builder {
	for _, u := range urls {
		b.paths = append(b.paths, NewURLVisitor(b.mapper, httpAttemptCount, u, b.schema))
	}
	return b
}

// Path accepts a set of paths that may be files, directories (all can contain
// one or more resources). Creates a FileVisitor for each file and then each
// FileVisitor is streaming the content to a StreamVisitor.
func (b *Builder) Path(recursive bool, paths ...string) *Builder {
	for _, p := range paths {
		_, err := os.Stat(p)
		if os.IsNotExist(err) {
			b.errs = append(b.errs, fmt.Errorf("the path %q does not exist", p))
			continue
		}
		if err != nil {
			b.errs = append(b.errs, fmt.Errorf("the path %q cannot be accessed: %v", p, err))
			continue
		}

		visitors, err := ExpandPathsToFileVisitors(b.mapper, p, recursive, resource.FileExtensions, b.schema)
		if err != nil {
			b.errs = append(b.errs, fmt.Errorf("error reading %q: %v", p, err))
		}

		b.paths = append(b.paths, visitors...)
	}
	if len(b.paths) == 0 && len(b.errs) == 0 {
		b.errs = append(b.errs, fmt.Errorf("error reading %v: recognized file extensions are %v", paths, resource.FileExtensions))
	}
	return b
}

// Do returns a Result object with a Visitor for the resources identified by the Builder. Note that stream
// inputs are consumed by the first execution - use Infos() or Objects() on the Result to capture a list
// for further iteration.
func (b *Builder) Do() *Result {
	r := b.visitorResult()
	return r
}

func (b *Builder) visitorResult() *Result {
	if len(b.errs) > 0 {
		return &Result{err: utilerrors.NewAggregate(b.errs)}
	}

	// visit items specified by paths
	if len(b.paths) != 0 {
		return b.visitByPaths()
	}
	return &Result{err: errMissingResource}
}

func (b *Builder) visitByPaths() *Result {
	result := &Result{}

	result.visitor = VisitorList(b.paths)
	return result
}

// expandIfFilePattern returns all the filenames that match the input pattern
// or the filename if it is a specific filename and not a pattern.
// If the input is a pattern and it yields no result it will result in an error.
func expandIfFilePattern(pattern string) ([]string, error) {
	if _, err := os.Stat(pattern); os.IsNotExist(err) {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) == 0 {
			return nil, fmt.Errorf("the path %q does not exist", pattern)
		}
		if err == filepath.ErrBadPattern {
			return nil, fmt.Errorf("pattern %q is not valid: %v", pattern, err)
		}
		return matches, err
	}
	return []string{pattern}, nil
}
