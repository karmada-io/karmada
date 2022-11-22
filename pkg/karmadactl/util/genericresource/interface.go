package genericresource

// Visitor lets clients walk a list of resources.
type Visitor interface {
	Visit(VisitorFunc) error
}

// VisitorFunc implements the Visitor interface for a matching function.
// If there was a problem walking a list of resources, the incoming error
// will describe the problem and the function can decide how to handle that error.
// A nil returned indicates to accept an error to continue loops even when errors happen.
// This is useful for ignoring certain kinds of errors or aggregating errors in some way.
type VisitorFunc func(*Info, error) error
