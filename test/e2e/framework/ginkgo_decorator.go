package framework

import "github.com/onsi/ginkgo/v2"

// SerialDescribe is wrapper function for ginkgo.Describe with ginkgo.Serial decorator.
func SerialDescribe(text string, args ...interface{}) bool {
	return ginkgo.Describe(text, ginkgo.Serial, args)
}

// SerialContext is wrapper function for ginkgo.Context with ginkgo.Serial decorator.
func SerialContext(text string, args ...interface{}) bool {
	return ginkgo.Context(text, ginkgo.Serial, args)
}

// SerialWhen is wrapper function for ginkgo.When with ginkgo.Serial decorator.
func SerialWhen(text string, args ...interface{}) bool {
	return ginkgo.When(text, ginkgo.Serial, args)
}
