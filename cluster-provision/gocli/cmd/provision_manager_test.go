package cmd

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Provision Manager functionality", func() {
	It("processChanges", func() {
		rulesDB := make(map[string][]string)
		rulesDB["file1"] = []string{"t1"}

		targetToRebuild, err := processChanges(rulesDB, []string{"t1", "t2"}, "A\tfile1")
		Expect(err).ToNot(HaveOccurred())
		Expect(targetToRebuild).To(Equal(map[string]bool{"t1": true, "t2": false}))
	})
})
