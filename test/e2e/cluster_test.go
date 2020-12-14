package e2e

import (
	"github.com/onsi/ginkgo"
)

var _ = ginkgo.Describe("[cluster-lifecycle] [cluster-join] cluster lifecycle functionality testing", func() {
	ginkgo.BeforeEach(func() {
		// TODO(RainbowMango): create a new member cluster which will be used by following tests.
	})

	ginkgo.AfterEach(func() {
		// TODO(RainbowMango): remove member clusters that created for this test
	})

	ginkgo.Context("normal cluster join and unjoin functionality", func() {
		ginkgo.It("new cluster could be joined to control plane", func() {
			ginkgo.By("join new member cluster", func() {
				// TODO(RainbowMango): add implementations here
			})
			ginkgo.By("check member cluster status", func() {
				// TODO(RainbowMango): add implementations here
			})
			ginkgo.By("unjoin member cluster")
		})
	})

	ginkgo.Context("abnormal cluster join and unjoin functionality", func() {
		// TODO(RainbowMango): add implementations here
	})
})
