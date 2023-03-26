package cmd

import (
	"bytes"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func globDirectoriesMock(path string) ([]string, error) {
	tokens := strings.Split(path, "/")
	tokensWithoutLast := tokens[:len(tokens)-1]
	dirName := strings.Join(tokensWithoutLast, "/")

	return []string{dirName + "/target1", dirName + "/target2"}, nil
}

var _ = Describe("Provision Manager functionality", func() {
	BeforeEach(func() {
		globFunction = globDirectoriesMock
	})

	AfterEach(func() {
		globFunction = globDirectories
	})

	Context("processChanges", func() {
		var targets []string
		BeforeEach(func() {
			var err error
			targets, err = getTargets("cluster-provision/cluster/*")
			Expect(err).ToNot(HaveOccurred())
		})

		It("When file is Added", func() {
			rulesDB := make(map[string][]string)
			rulesDB["file1"] = []string{"target1"}

			targetToRebuild, err := processChanges(rulesDB, targets, "A\tfile1")
			Expect(err).ToNot(HaveOccurred())
			Expect(targetToRebuild).To(Equal(map[string]bool{"target1": true, "target2": false}))
		})

		It("When markdown file is added", func() {
			rulesDB := make(map[string][]string)
			rulesDB["file1"] = []string{"target1"}

			targetToRebuild, err := processChanges(rulesDB, targets, "A\tREADME.md")
			Expect(err).ToNot(HaveOccurred())
			Expect(targetToRebuild).To(Equal(map[string]bool{"target1": false, "target2": false}))
		})

		It("When added file doesn't have a matching rule", func() {
			rulesDB := make(map[string][]string)
			rulesDB["file2"] = []string{"target2"}

			_, err := processChanges(rulesDB, targets, "A\tfile1")
			Expect(err).ToNot(Equal("Errors detected: files dont have a matching rule"))
		})

		It("When file is Deleted", func() {
			rulesDB := make(map[string][]string)
			rulesDB["file2"] = []string{"target2"}

			targetToRebuild, err := processChanges(rulesDB, targets, "D\tfile2")
			Expect(err).ToNot(HaveOccurred())
			Expect(targetToRebuild).To(Equal(map[string]bool{"target1": false, "target2": true}))
		})

		It("When file is Renamed", func() {
			rulesDB := make(map[string][]string)
			rulesDB["file1"] = []string{"target1"}

			targetToRebuild, err := processChanges(rulesDB, targets, "R70\told_file1\tfile1")
			Expect(err).ToNot(HaveOccurred())
			Expect(targetToRebuild).To(Equal(map[string]bool{"target1": true, "target2": false}))
		})
	})

	Context("buildRulesDB", func() {
		var targets []string
		BeforeEach(func() {
			var err error
			targets, err = getTargets("cluster-provision/cluster/*")
			Expect(err).ToNot(HaveOccurred())
		})

		It("buildRulesDB", func() {
			rules := []string{
				"cluster-up/none/* - regex_none",
				"cluster-up/cluster/* - regex",
			}
			ibuf := bytes.NewBufferString(strings.Join(rules, "\n"))
			rulesDB, err := buildRulesDB(ibuf, targets)
			Expect(err).ToNot(HaveOccurred())
			Expect(rulesDB).To(Equal(map[string][]string{
				"cluster-up/none/target1/*":    []string{"none"},
				"cluster-up/none/target2/*":    []string{"none"},
				"cluster-up/cluster/target1/*": []string{"target1"},
				"cluster-up/cluster/target2/*": []string{"target2"},
			}))
		})
	})
})
