/*
Copyright 2020 The Karmada Authors.

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

package integration

import (
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	// TestSuiteSetupTimeOut defines the time after which the suite setup times out.
	TestSuiteSetupTimeOut = 300 * time.Second
	// TestSuiteTeardownTimeOut defines the time after which the suite tear down times out.
	TestSuiteTeardownTimeOut = 300 * time.Second
)

func TestIntegration(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Integration Suite")
}

var _ = ginkgo.BeforeSuite(func() {
	// suite set up, such as get karmada environment ready.
})

var _ = ginkgo.AfterSuite(func() {
	// suite tear down, such as cleanup karmada environment.
})
