package genericresource

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/resource"
)

const (
	constSTDINstr = "STDIN"
)

// VisitorList implements Visit for the sub visitors it contains. The first error
// returned from a child Visitor will terminate iteration.
type VisitorList []Visitor

// Visit implements Visitor
func (l VisitorList) Visit(fn VisitorFunc) error {
	for i := range l {
		if err := l[i].Visit(fn); err != nil {
			return err
		}
	}
	return nil
}

// URLVisitor downloads the contents of a URL, and if successful, returns
// an info object representing the downloaded object.
type URLVisitor struct {
	URL *url.URL
	*StreamVisitor
	HTTPAttemptCount int
}

// NewURLVisitor returns a visitor to download from given url. It will max retry "httpAttemptCount" when failed.
func NewURLVisitor(mapper *mapper, httpAttemptCount int, u *url.URL, schema resource.ContentValidator) *URLVisitor {
	return &URLVisitor{
		URL:              u,
		StreamVisitor:    NewStreamVisitor(nil, mapper, u.String(), schema),
		HTTPAttemptCount: httpAttemptCount,
	}
}

// Visit down object from url.
func (v *URLVisitor) Visit(fn VisitorFunc) error {
	body, err := readHTTPWithRetries(httpgetImpl, time.Second, v.URL.String(), v.HTTPAttemptCount)
	if err != nil {
		return err
	}
	defer body.Close()
	v.StreamVisitor.Reader = body
	return v.StreamVisitor.Visit(fn)
}

func ignoreFile(path string, extensions []string) bool {
	if len(extensions) == 0 {
		return false
	}
	ext := filepath.Ext(path)
	for _, s := range extensions {
		if s == ext {
			return false
		}
	}
	return true
}

// FileVisitorForSTDIN return a special FileVisitor just for STDIN
func FileVisitorForSTDIN(mapper *mapper, schema resource.ContentValidator) Visitor {
	return &FileVisitor{
		Path:          constSTDINstr,
		StreamVisitor: NewStreamVisitor(nil, mapper, constSTDINstr, schema),
	}
}

// ExpandPathsToFileVisitors will return a slice of FileVisitors that will handle files from the provided path.
// After FileVisitors open the files, they will pass an io.Reader to a StreamVisitor to do the reading. (stdin
// is also taken care of). Paths argument also accepts a single file, and will return a single visitor
func ExpandPathsToFileVisitors(mapper *mapper, paths string, recursive bool, extensions []string, schema resource.ContentValidator) ([]Visitor, error) {
	var visitors []Visitor
	err := filepath.Walk(paths, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if fi.IsDir() {
			if path != paths && !recursive {
				return filepath.SkipDir
			}
			return nil
		}
		// Don't check extension if the filepath was passed explicitly
		if path != paths && ignoreFile(path, extensions) {
			return nil
		}

		visitor := &FileVisitor{
			Path:          path,
			StreamVisitor: NewStreamVisitor(nil, mapper, path, schema),
		}

		visitors = append(visitors, visitor)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return visitors, nil
}

// FileVisitor is wrapping around a StreamVisitor, to handle open/close files
type FileVisitor struct {
	Path string
	*StreamVisitor
}

// Visit in a FileVisitor is just taking care of opening/closing files
func (v *FileVisitor) Visit(fn VisitorFunc) error {
	var f *os.File
	if v.Path == constSTDINstr {
		f = os.Stdin
	} else {
		var err error
		f, err = os.Open(v.Path)
		if err != nil {
			return err
		}
		defer f.Close()
	}

	// TODO: Consider adding a flag to force to UTF16, apparently some
	// Windows tools don't write the BOM
	utf16bom := unicode.BOMOverride(unicode.UTF8.NewDecoder())
	v.StreamVisitor.Reader = transform.NewReader(f, utf16bom)

	return v.StreamVisitor.Visit(fn)
}

// StreamVisitor reads objects from an io.Reader and walks them. A stream visitor can only be
// visited once.
// Unmarshal stream to object by json.
type StreamVisitor struct {
	io.Reader
	*mapper

	Source string
	Schema resource.ContentValidator
}

// NewStreamVisitor is a helper function that is useful when we want to change the fields of the struct but keep calls the same.
func NewStreamVisitor(r io.Reader, mapper *mapper, source string, schema resource.ContentValidator) *StreamVisitor {
	return &StreamVisitor{
		Reader: r,
		mapper: mapper,
		Source: source,
		Schema: schema,
	}
}

// Visit implements Visitor over a stream. StreamVisitor is able to distinct multiple resources in one stream.
func (v *StreamVisitor) Visit(fn VisitorFunc) error {
	d := yaml.NewYAMLOrJSONDecoder(v.Reader, 4096)
	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("error parsing %s: %v", v.Source, err)
		}
		// TODO: This needs to be able to handle object in other encodings and schemas.
		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}
		if err := resource.ValidateSchema(ext.Raw, v.Schema); err != nil {
			return fmt.Errorf("error validating %q: %v", v.Source, err)
		}
		info, err := v.infoForData(ext.Raw, v.Source)
		if err != nil {
			if fnErr := fn(info, err); fnErr != nil {
				return fnErr
			}
			continue
		}
		if err = fn(info, nil); err != nil {
			return err
		}
	}
}

type mapper struct {
	newFunc func() interface{}
}

func (m *mapper) infoForData(data []byte, source string) (*Info, error) {
	info := &Info{
		Source: source,
		Data:   data,
		Object: m.newFunc(),
	}

	err := json.Unmarshal(data, info.Object)
	if err != nil {
		return nil, err
	}

	return info, nil
}

// httpget Defines function to retrieve a url and return the results.  Exists for unit test stubbing.
type httpget func(url string) (int, string, io.ReadCloser, error)

// httpgetImpl Implements a function to retrieve a url and return the results.
func httpgetImpl(url string) (int, string, io.ReadCloser, error) {
	// nolint:gosec
	resp, err := http.Get(url)
	if err != nil {
		return 0, "", nil, err
	}
	return resp.StatusCode, resp.Status, resp.Body, nil
}

// readHTTPWithRetries tries to http.Get the v.URL retries times before giving up.
func readHTTPWithRetries(get httpget, duration time.Duration, u string, attempts int) (io.ReadCloser, error) {
	var err error
	if attempts <= 0 {
		return nil, fmt.Errorf("http attempts must be greater than 0, was %d", attempts)
	}
	for i := 0; i < attempts; i++ {
		var (
			statusCode int
			status     string
			body       io.ReadCloser
		)
		if i > 0 {
			time.Sleep(duration)
		}

		// Try to get the URL
		statusCode, status, body, err = get(u)

		// Retry Errors
		if err != nil {
			continue
		}

		if statusCode == http.StatusOK {
			return body, nil
		}
		body.Close()
		// Error - Set the error condition from the StatusCode
		err = fmt.Errorf("unable to read URL %q, server reported %s, status code=%d", u, status, statusCode)

		if statusCode >= 500 && statusCode < 600 {
			// Retry 500's
			continue
		} else {
			// Don't retry other StatusCodes
			break
		}
	}
	return nil, err
}

// Info contains temporary info to execute a REST call, or show the results
// of an already completed REST call.
type Info struct {
	// Optional, Source is the filename or URL to template file (.json or .yaml),
	// or stdin to use to handle the resource
	Source string
	// Optional, this is the most recent value returned by the server if available. It will
	// typically be in unstructured or internal forms, depending on how the Builder was
	// defined. If retrieved from the server, the Builder expects the mapping client to
	// decide the final form. Use the AsVersioned, AsUnstructured, and AsInternal helpers
	// to alter the object versions.
	// If Subresource is specified, this will be the object for the subresource.
	Data []byte

	Object interface{}
}
