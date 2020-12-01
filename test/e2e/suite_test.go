package e2e

import (
	"testing"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

const (
	// TestSuiteSetupTimeOut defines the time after which the suite setup times out.
	TestSuiteSetupTimeOut = 300 * time.Second
	// TestSuiteTeardownTimeOut defines the time after which the suite tear down times out.
	TestSuiteTeardownTimeOut = 300 * time.Second
)

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "E2E Suite")
}

var _ = ginkgo.BeforeSuite(func() {
	// suite set up, such as get karmada environment ready.
}, TestSuiteSetupTimeOut.Seconds())

var _ = ginkgo.AfterSuite(func() {
	// suite tear down, such as cleanup karmada environment.
}, TestSuiteTeardownTimeOut.Seconds())
