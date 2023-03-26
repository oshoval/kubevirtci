package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	fsys "kubevirt.io/kubevirtci/cluster-provision/gocli/cmd/filesystem"
)

const (
	// Git status
	FILE_ADDED   = "A"
	FILE_DELETED = "D"
	FILE_RENAMED = "R"

	TARGET_NONE = "none"
)

type parameters struct {
	tag       string
	rulesFile string
	debug     bool
}

type OutputSplitter struct{}

var fileSystem fsys.FileSystem = fsys.RealFileSystem{}

func SetFileSystem(fs fsys.FileSystem) {
	if fs == nil {
		fileSystem = fsys.RealFileSystem{}
	} else {
		fileSystem = fs
	}
}

// NewProvisionManagerCommand determines which providers should be rebuilt
func NewProvisionManagerCommand() *cobra.Command {
	provision := &cobra.Command{
		Use:   "pman",
		Short: "provision manager determines which providers should be rebuilt",
		RunE:  provisionManager,
		Args:  cobra.ExactArgs(0),
	}
	provision.Flags().String("tag", "", "kubevirtci tag to compare to, default: fetch latest")
	provision.Flags().String("rules", "hack/pman/rules.txt", "rules file, default: hack/pman/rules.txt")
	provision.Flags().Bool("debug", false, "run in debug mode, default: false")

	return provision
}

func provisionManager(cmd *cobra.Command, arguments []string) error {
	params, err := parseArguments(cmd)
	if err != nil {
		return err
	}

	configLogger(params.debug)

	// Sleep to let logrus flush its buffer, in order to avoid race between logrus and printing of the return value
	defer func() {
		time.Sleep(200 * time.Millisecond)
	}()

	if len(params.tag) == 0 {
		params.tag, err = getKubevirtciTag()
		if err != nil {
			return err
		}
	}

	printSection("Parameters")
	logrus.Debug("Tag: ", params.tag)

	targets, err := getTargets("cluster-provision/k8s/*")
	if err != nil {
		return err
	}

	rulesDB, err := buildRulesDBfromFile(params.rulesFile, targets)
	if err != nil {
		return err
	}

	targetToRebuild, err := processGitNameStatusChanges(rulesDB, targets, params.tag)
	if err != nil {
		return err
	}

	j, err := json.Marshal(targetToRebuild)
	if err != nil {
		return err
	}

	printSection("Result")
	fmt.Println(string(j))

	return nil
}

func parseArguments(cmd *cobra.Command) (parameters, error) {
	params := parameters{}
	var err error

	params.debug, err = cmd.Flags().GetBool("debug")
	if err != nil {
		return parameters{}, err
	}

	params.rulesFile, err = cmd.Flags().GetString("rules")
	if err != nil {
		return parameters{}, err
	}

	params.tag, err = cmd.Flags().GetString("tag")
	if err != nil {
		return parameters{}, err
	}

	return params, nil
}

func configLogger(debug bool) {
	logrus.SetOutput(&OutputSplitter{})

	logrus.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.SetFormatter(&logrus.TextFormatter{DisableLevelTruncation: true, ForceColors: true, DisableTimestamp: true})
	}
}

func (splitter *OutputSplitter) Write(p []byte) (n int, err error) {
	if bytes.Contains(p, []byte("level=error")) || bytes.Contains(p, []byte("level=fatal")) {
		return os.Stderr.Write(p)
	}
	return os.Stdout.Write(p)
}

func getTargets(path string) ([]string, error) {
	directories, err := globDirectories("cluster-provision/k8s/*")
	if err != nil {
		return nil, err
	}

	targets := []string{}
	for _, dir := range directories {
		targets = append(targets, filepath.Base(dir))
	}

	logrus.Debug("Targets: ", targets)
	return targets, nil
}

func globDirectories(path string) ([]string, error) {
	files, err := fileSystem.Glob(path)
	if err != nil {
		return nil, err
	}

	var directories []string
	for _, candid := range files {
		f, err := os.Stat(candid)
		if err != nil {
			return nil, err
		}
		if f.IsDir() {
			directories = append(directories, candid)
		}
	}

	return directories, nil
}

func processGitNameStatusChanges(rulesDB map[string][]string, targets []string, tag string) (map[string]bool, error) {
	cmdOutput, err := runCommand("git", []string{"diff", "--name-status", tag})
	if err != nil {
		return nil, err
	}

	return processChanges(rulesDB, targets, cmdOutput)
}

func processChanges(rulesDB map[string][]string, targets []string, changes string) (map[string]bool, error) {
	targetToRebuild := make(map[string]bool)
	for _, target := range targets {
		targetToRebuild[target] = false
	}

	printSection("Changed files")

	errorFound := false
	files := strings.Split(changes, "\n")
	for _, nameStatus := range files {
		if nameStatus == "" {
			break
		}

		tokens := strings.Split(nameStatus, "\t")
		if len(tokens) < 2 {
			return nil, fmt.Errorf("wrong input syntax, should be <status>\\t<filename>")
		}

		status := tokens[0]
		fileName := tokens[1]

		if strings.HasPrefix(status, FILE_RENAMED) {
			if len(tokens) != 3 {
				return nil, fmt.Errorf("wrong input syntax, should be <status>\\t<old_filename>\\t<new_filename>")
			}
			fileName = tokens[2]
		}

		// Skip markdown files
		if strings.HasSuffix(fileName, ".md") {
			continue
		}

		match, err := matcher(rulesDB, fileName, status)
		if err != nil {
			errorFound = true
			logrus.Error(err)
			continue
		}

		if !errorFound {
			logrus.Debug(status + " : " + fileName + " - [" + strings.Join(match, " ") + "]")
		}

		for _, target := range match {
			if target != TARGET_NONE {
				targetToRebuild[target] = true
			}
		}
	}

	if errorFound {
		return nil, fmt.Errorf("Errors detected: files dont have a matching rule")
	}

	return targetToRebuild, nil
}

func matcher(rulesDB map[string][]string, fileName string, status string) ([]string, error) {
	match, ok := rulesDB[fileName]
	if ok {
		return match, nil
	}

	match, ok = rulesDB[filepath.Dir(fileName)]
	if ok {
		return match, nil
	}

	candid := fileName
	for candid != "." && candid != "/" {
		candid = filepath.Dir(candid)
		match, ok = rulesDB[candid+"/*"]
		if ok {
			return match, nil
		}
	}

	if status != FILE_DELETED {
		return nil, fmt.Errorf("Failed to find a rule for " + fileName)
	}

	return nil, nil
}

func getKubevirtciTag() (string, error) {
	const kubevirtciTagUrl = "https://storage.googleapis.com/kubevirt-prow/release/kubevirt/kubevirtci/latest?ignoreCache=1"
	output, err := runCommand("curl", []string{kubevirtciTagUrl})
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(output, "\n"), err
}

func runCommand(command string, args []string) (string, error) {
	var stdout, stderr bytes.Buffer

	cmd := exec.Command(command, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if strings.Contains(stderr.String(), "unknown revision or path not in the working tree") {
			logrus.Error("Tag not found, run git pull --tags")
		}
		return "", errors.Wrapf(err, "Failed to run git command: git %s\nStdout:\n%s\nStderr:\n%s",
			strings.Join(args, " "), cmd.Stdout, cmd.Stderr)
	}

	return stdout.String(), nil
}

func buildRulesDBfromFile(rulesFile string, targets []string) (map[string][]string, error) {
	inFile, err := fileSystem.Open(rulesFile)
	if err != nil {
		return nil, err
	}
	defer inFile.Close()

	return buildRulesDB(inFile, targets)
}

func buildRulesDB(input io.Reader, targets []string) (map[string][]string, error) {
	rulesDB := make(map[string][]string)

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tokens := strings.Fields(line)
		if len(tokens) != 3 || tokens[1] != "-" {
			return nil, fmt.Errorf("Invalid syntax (should be 'directory - target'), rule: " + line)
		}

		dir := tokens[0]
		target := tokens[2]

		switch {
		case target == "all":
			rulesDB[dir] = targets
		case target == TARGET_NONE:
			rulesDB[dir] = []string{TARGET_NONE}
		case strings.HasPrefix(target, "!"):
			target = target[1:]
			if !isTargetValid(target, targets) {
				return nil, fmt.Errorf("Invalid target, rule: " + line)
			}
			rulesDB[dir] = excludeTarget(target, targets)
		case target == "regex":
			directories, _ := globDirectories(dir)
			for _, dir := range directories {
				target = strings.ReplaceAll(filepath.Base(dir), "k8s-", "")
				if !isTargetValid(target, targets) {
					return nil, fmt.Errorf("Invalid target " + target + ", rule: " + line)
				}
				rulesDB[dir+"/*"] = []string{target}
			}
		case target == "regex_none":
			directories, _ := globDirectories(dir)
			for _, dir := range directories {
				rulesDB[dir+"/*"] = []string{TARGET_NONE}
			}
		default:
			if !isTargetValid(target, targets) {
				return nil, fmt.Errorf("Invalid target, rule: " + line)
			}
			rulesDB[dir] = []string{target}
		}
	}

	printRulesDB(rulesDB)

	return rulesDB, nil
}

func printRulesDB(rulesDB map[string][]string) {
	keys := make([]string, 0, len(rulesDB))
	for k := range rulesDB {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	printSection("Rules")

	for _, k := range keys {
		logrus.Debug(k + " : [" + strings.Join(rulesDB[k], " ") + "]")
	}
}

func isTargetValid(target string, targets []string) bool {
	for _, t := range targets {
		if t == target {
			return true
		}
	}
	return false
}

func excludeTarget(target string, targets []string) []string {
	newTargets := []string{}
	for _, t := range targets {
		if t != target {
			newTargets = append(newTargets, t)
		}
	}
	return newTargets
}

func printSection(title string) {
	title += ":"
	dashes := strings.Repeat("-", len(title))

	logrus.Debug(dashes)
	logrus.Debug(title)
	logrus.Debug(dashes)
}
