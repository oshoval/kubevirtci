package cmd

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/afero"
)

var fsMock afero.Fs

type MockFileSystem struct {
	fs afero.Fs
}

func (fs MockFileSystem) Open(name string) (afero.File, error) {
	return fs.fs.Open(name)
}

func (fs MockFileSystem) Glob(pattern string) ([]string, error) {
	return afero.Glob(fs.fs, pattern)
}

var _ = BeforeSuite(func() {
	fsMock = afero.NewMemMapFs()

	dirs := []string{
		"cluster-provision/k8s",
		"cluster-provision/k8s/target1",
		"cluster-provision/k8s/target2",
	}

	for _, dir := range dirs {
		err := fsMock.MkdirAll(dir, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
	}

	SetFileSystem(MockFileSystem{fsMock})
})

var _ = AfterSuite(func() {
	SetFileSystem(nil)
})

var _ = Describe("Provision Manager functionality", func() {
	Describe("processChanges", func() {
		var targets []string
		BeforeEach(func() {
			var err error
			targets, err = getTargets("cluster-provision/k8s/*")
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when rules exists in rulesDB", func() {
			It("returns expected target when matching file is Added", func() {
				rulesDB := make(map[string][]string)
				rulesDB["file1"] = []string{"target1"}

				targetToRebuild, err := processChanges(rulesDB, targets, "A\tfile1")
				Expect(err).ToNot(HaveOccurred())
				Expect(targetToRebuild).To(Equal(map[string]bool{"target1": true, "target2": false}))
			})

			It("ignore added markdown files", func() {
				rulesDB := make(map[string][]string)
				rulesDB["file1"] = []string{"target1"}

				targetToRebuild, err := processChanges(rulesDB, targets, "A\tREADME.md")
				Expect(err).ToNot(HaveOccurred())
				Expect(targetToRebuild).To(Equal(map[string]bool{"target1": false, "target2": false}))
			})

			It("fails when added file doesn't have a matching rule", func() {
				rulesDB := make(map[string][]string)
				rulesDB["file2"] = []string{"target2"}

				_, err := processChanges(rulesDB, targets, "A\tfile_should_fail")
				Expect(err).ToNot(Equal("Errors detected: files dont have a matching rule"))
			})

			It("returns expected target when matching file is Deleted", func() {
				rulesDB := make(map[string][]string)
				rulesDB["file2"] = []string{"target2"}

				targetToRebuild, err := processChanges(rulesDB, targets, "D\tfile2")
				Expect(err).ToNot(HaveOccurred())
				Expect(targetToRebuild).To(Equal(map[string]bool{"target1": false, "target2": true}))
			})

			It("returns expected target when matching file is Renamed", func() {
				rulesDB := make(map[string][]string)
				rulesDB["file1"] = []string{"target1"}
				rulesDB["file2"] = []string{"target2"}

				targetToRebuild, err := processChanges(rulesDB, targets, "R70\tfile2\tfile1")
				Expect(err).ToNot(HaveOccurred())
				Expect(targetToRebuild).To(Equal(map[string]bool{"target1": true, "target2": false}))
			})
		})
	})

	Describe("buildRulesDB", func() {
		var targets []string
		BeforeEach(func() {
			var err error
			targets, err = getTargets("cluster-provision/k8s/*")
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns an error for invalid syntax", func() {
			input := strings.NewReader("invalid-syntax")
			_, err := buildRulesDB(input, targets)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for invalid targets", func() {
			input := strings.NewReader("cluster-provision/k8s/* - invalid-target")
			_, err := buildRulesDB(input, targets)
			Expect(err).To(HaveOccurred())
		})

		DescribeTable("buildRulesDBfromFile",
			func(contentStr []string, expected map[string][]string) {
				err := createFileWithContent("rules.txt", contentStr)
				Expect(err).ToNot(HaveOccurred())

				rulesDB, err := buildRulesDBfromFile("rules.txt", targets)
				Expect(err).ToNot(HaveOccurred())

				Expect(rulesDB).To(Equal(expected))
			},
			Entry("should create a regex rule",
				[]string{"cluster-provision/k8s/target* - regex"},
				map[string][]string{
					"cluster-provision/k8s/target1/*": []string{"target1"},
					"cluster-provision/k8s/target2/*": []string{"target2"},
				},
			),
			Entry("should create a regex_none rule",
				[]string{"cluster-provision/k8s/target* - regex_none"},
				map[string][]string{
					"cluster-provision/k8s/target1/*": []string{"none"},
					"cluster-provision/k8s/target2/*": []string{"none"},
				},
			),
			Entry("should create an 'all' rule",
				[]string{"cluster-provision/k8s/* - all"},
				map[string][]string{
					"cluster-provision/k8s/*": []string{"target1", "target2"},
				},
			),
			Entry("should create a 'none' rule",
				[]string{"cluster-provision/k8s/* - none"},
				map[string][]string{
					"cluster-provision/k8s/*": []string{"none"},
				},
			),
			Entry("should create an exclude rule",
				[]string{"cluster-provision/k8s/* - !target1"},
				map[string][]string{
					"cluster-provision/k8s/*": []string{"target2"},
				},
			),
			Entry("should create a specific target rule",
				[]string{"cluster-provision/k8s/* - target1"},
				map[string][]string{
					"cluster-provision/k8s/*": []string{"target1"},
				},
			),
		)
	})

	Describe("matcher", func() {
		rulesDB := map[string][]string{
			"path/to/file":      {"rule1", "rule2"},
			"path/to/directory": {"rule2", "rule3"},
			"path/to/dir/*":     {"rule4"},
		}

		Context("when file path exists in rulesDB", func() {
			It("returns matching rules for exact file name", func() {
				matches, err := matcher(rulesDB, "path/to/file", FILE_ADDED)
				Expect(err).To(BeNil())
				Expect(matches).To(Equal([]string{"rule1", "rule2"}))
			})

			It("returns matching rules for file name in a non recursive directory", func() {
				matches, err := matcher(rulesDB, "path/to/directory/file", FILE_ADDED)
				Expect(err).To(BeNil())
				Expect(matches).To(Equal([]string{"rule2", "rule3"}))
			})

			It("returns matching rules for file name in the recursive directory", func() {
				matches, err := matcher(rulesDB, "path/to/dir/file", FILE_ADDED)
				Expect(err).To(BeNil())
				Expect(matches).To(Equal([]string{"rule4"}))
			})

			It("returns matching rules for file name in the parent directory", func() {
				matches, err := matcher(rulesDB, "path/to/dir/subdir/file", FILE_ADDED)
				Expect(err).To(BeNil())
				Expect(matches).To(Equal([]string{"rule4"}))
			})
		})

		Context("when file path does not exist in rulesDB", func() {
			It("returns an error when status is not FILE_DELETED", func() {
				_, err := matcher(rulesDB, "path/to/nonexistentfile", FILE_ADDED)
				Expect(err).NotTo(BeNil())
			})

			It("returns non error when status is FILE_DELETED", func() {
				matches, err := matcher(rulesDB, "path/to/nonexistentfile", FILE_DELETED)
				Expect(err).To(BeNil())
				Expect(matches).To(BeNil())
			})
		})
	})
})

func createFileWithContent(filePath string, contentStr []string) error {
	content := []byte("")

	for _, str := range contentStr {
		content = append(content, []byte(str)...)
		content = append(content, []byte("\n")...)
	}

	return afero.WriteFile(fsMock, filePath, content, 0644)
}
