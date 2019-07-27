package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cjo5/dingo/internal/backend"
	"github.com/cjo5/dingo/internal/frontend"

	"github.com/cjo5/dingo/internal/semantics"

	"github.com/cjo5/dingo/internal/common"

	"github.com/cjo5/dingo/internal/ir"
	"github.com/cjo5/dingo/internal/token"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	var manifest string
	var explicitTests string

	flag.StringVar(&manifest, "manifest", "", "Test manifest")
	flag.StringVar(&explicitTests, "test", "", "Explicit tests -- remaining arguments are interpreted as modules")
	flag.Parse()

	var groups []*testGroup
	tester := &testRunner{cwd: cwd}

	if len(manifest) > 0 {
		groups = readTestManifest(manifest)
		tester.baseDir = filepath.Dir(manifest)
	} else {
		var tests []string
		var modules []string
		if len(explicitTests) > 0 {
			tests = strings.Split(explicitTests, " ")
		} else {
			tests = flag.Args()
		}
		groups = createTestGroups(tests)
		tester.defaultModules = modules
	}

	tester.total = countTests(groups)
	tester.runTestGroups(groups)
	fmt.Printf("\n%d/%d test(s) %s (%d %s, %d %s, and %d %s)\n",
		tester.success, tester.total, statusSuccess,
		tester.fail, statusFail,
		tester.invalid, statusInvalid,
		tester.skip, statusSkip,
	)
}

type testRunner struct {
	cwd            string
	baseDir        string
	defaultModules []string

	// stats
	total   int
	success int
	skip    int
	fail    int
	invalid int
}

type testGroup struct {
	Disable bool
	Dir     string
	Modules []string
	Tests   []string
}

type testResult struct {
	status status
	reason []string
}

func (r *testResult) addReason(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	r.reason = append(r.reason, msg)
}

type testOutput struct {
	pos  token.Position
	text string
}

type testOutputPattern struct {
	pos   token.Position
	text  string
	parts []patternPart
}

func (t *testOutputPattern) addPart(text string, regex *regexp.Regexp) {
	part := patternPart{
		text:  text,
		regex: regex,
	}
	t.parts = append(t.parts, part)
}

type patternPart struct {
	text  string
	regex *regexp.Regexp
}

type status int

const (
	statusSuccess status = iota
	statusFail
	statusSkip
	statusInvalid
)

func (t status) String() string {
	switch t {
	case statusSuccess:
		return common.BoldGreen("passed")
	case statusFail:
		return common.BoldRed("failed")
	case statusSkip:
		return common.BoldPurple("disabled")
	case statusInvalid:
		return common.BoldRed("invalid")
	default:
		return "-"
	}
}

func abort(err error) {
	fmt.Println("error:", err)
	os.Exit(1)
}

func readTestManifest(manifest string) []*testGroup {
	bytes, err := ioutil.ReadFile(manifest)
	if err != nil {
		abort(err)
	}
	var groups []*testGroup
	err = json.Unmarshal(bytes, &groups)
	if err != nil {
		abort(err)
	}
	return groups
}

func createTestGroups(testFiles []string) []*testGroup {
	var groups []*testGroup
	for _, testFile := range testFiles {
		test := &testGroup{}
		test.Tests = append(test.Tests, testFile)
		test.Dir = ""
		groups = append(groups, test)
	}
	return groups
}

func countTests(groups []*testGroup) int {
	count := 0
	for _, group := range groups {
		count += len(group.Tests)
	}
	return count
}

func toTestName(testDir string, testFile string) string {
	ext := filepath.Ext(testFile)
	baseName := filepath.Base(testFile)
	baseName = baseName[:len(baseName)-len(ext)]
	testName := filepath.Join(testDir, baseName)
	return testName
}

func toTestLine(name string, index int, count int) string {
	countStr := fmt.Sprintf("%d", count)
	indexStr := fmt.Sprintf("%d", index)
	line := fmt.Sprintf("%s%s/%s %s", strings.Repeat(" ", len(countStr)-len(indexStr)), indexStr, countStr, name)
	return line
}

func (t *testRunner) runTestGroups(groups []*testGroup) {
	testIndex := 1
	for groupIndex, group := range groups {
		testDir := group.Dir
		status := statusSuccess

		if group.Disable {
			status = statusSkip
		} else if len(group.Tests) == 0 {
			status = statusInvalid
		}

		if status != statusSuccess {
			if len(group.Tests) > 0 {
				for _, testFile := range group.Tests {
					line := toTestLine(toTestName(testDir, testFile), testIndex, t.total)
					fmt.Printf("test %s ... %s\n", line, status)
					t.updateStats(status)
					testIndex++
				}
			} else {
				line := toTestLine(testDir, groupIndex, len(groups))
				fmt.Printf("group %s ... %s\n", line, status)
				t.updateStats(status)
			}
			continue
		}

		for _, testFile := range group.Tests {
			testName := toTestName(testDir, testFile)
			line := toTestLine(testName, testIndex, t.total)
			fmt.Printf("test %s ... ", line)

			result := t.runTest(testName, testDir, testFile, group.Modules)
			t.updateStats(result.status)
			testIndex++

			fmt.Printf("%s\n", result.status)
			for _, txt := range result.reason {
				fmt.Printf("  >> %s\n", txt)
			}
		}
	}
}

func (t *testRunner) updateStats(res status) {
	switch res {
	case statusSuccess:
		t.success++
	case statusFail:
		t.fail++
	case statusSkip:
		t.skip++
	case statusInvalid:
		t.invalid++
	}
}

func (t *testRunner) runTest(testName string, testDir string, testFile string, testModules []string) *testResult {
	var filenames []string
	filenames = append(filenames, filepath.Join(t.baseDir, testDir, testFile))
	for _, mod := range testModules {
		filename := filepath.Join(t.baseDir, testDir, mod)
		filenames = append(filenames, filename)
	}
	for _, defaultMod := range t.defaultModules {
		filename := filepath.Join(t.baseDir, testDir, defaultMod)
		filenames = append(filenames, filename)
	}

	ctx := common.NewBuildContext(t.cwd)
	ctx.Exe = filepath.Join(os.TempDir(), strings.Replace(testName, "/", "_", -1))

	var expectedCompilerOutput []*testOutputPattern
	var expectedExeOutput []*testOutputPattern

	result := &testResult{status: statusSuccess}
	fileMatrix, _ := frontend.Load(ctx, filenames)

	if fileMatrix != nil {
		expectedCompilerOutput, expectedExeOutput = parseTestDescription(fileMatrix[0][0].Comments, result)
		if result.status != statusSuccess {
			return result
		}
	}

	if !ctx.Errors.IsError() {
		target := backend.NewLLVMTarget()
		if declMatrix, ok := semantics.Check(ctx, target, fileMatrix); ok {
			backend.BuildLLVM(ctx, target, declMatrix)
		}
	}

	var compilerOutput []*testOutput

	ctx.Errors.Sort()
	addCompilerOutput(ctx.Errors.Warnings, &compilerOutput)
	addCompilerOutput(ctx.Errors.Errors, &compilerOutput)
	compareOutput(expectedCompilerOutput, compilerOutput, result)

	var exeOutput []*testOutput

	if !ctx.Errors.IsError() {
		cmd := exec.Command(ctx.Exe)
		bytes, err := cmd.CombinedOutput()
		if err != nil {
			result.addReason("internal error: %s", err)
		} else {
			addExeOutput(bytes, &exeOutput)
		}
	}

	compareOutput(expectedExeOutput, exeOutput, result)

	if len(result.reason) > 0 {
		result.status = statusFail
	}

	os.Remove(ctx.Exe)

	return result
}

func addError(newError error, errors *common.ErrorList) bool {
	if newError == nil {
		return false
	}
	errors.AddGeneric1(newError)
	return errors.IsError()
}

func addCompilerOutput(errors []*common.Error, output *[]*testOutput) {
	for _, err := range errors {
		pos := err.Pos
		msg := fmt.Sprintf("%s(%d): %s", err.ID, pos.Line, err.Msg)
		*output = append(*output, &testOutput{pos: pos, text: msg})
		for _, line := range err.Context {
			line = strings.TrimSpace(line)
			*output = append(*output, &testOutput{pos: pos, text: line})
		}
	}
}

func addExeOutput(bytes []byte, output *[]*testOutput) {
	lines := strings.Split(string(bytes), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if len(line) == 0 && (i+1) == len(lines) {
			break
		}
		pos := token.Position{Line: i + 1, Column: 1}
		*output = append(*output, &testOutput{pos: pos, text: line})
	}
}

func compareOutput(expectedOutput []*testOutputPattern, actualOutput []*testOutput, result *testResult) {
	expectedIndex := 0
	actualIndex := 0

	for ; expectedIndex < len(expectedOutput) &&
		actualIndex < len(actualOutput); expectedIndex, actualIndex = expectedIndex+1, actualIndex+1 {
		expected := expectedOutput[expectedIndex]
		actual := actualOutput[actualIndex]
		offset := 0
		partCount := 0

		for _, part := range expected.parts {
			partText := part.text
			if part.regex != nil {
				found := part.regex.FindString(actual.text[offset:])
				offset += len(found)
			} else {
				partLen := len(partText)
				remaining := (len(actual.text) - offset) - partLen
				if remaining < 0 || actual.text[offset:offset+partLen] != partText {
					break
				}
				offset += partLen
			}
			partCount++
		}

		if offset != len(actual.text) || partCount != len(expected.parts) {
			result.addReason("%s(%s): '%s'", common.BoldGreen("expected"), expected.pos, expected.text)
			result.addReason("     %s(%s): '%s'", common.BoldRed("got"), actual.pos, actual.text)
		}
	}

	if actualIndex < len(actualOutput) {
		result.addReason("%s:", common.BoldRed("got"))
		for i := actualIndex; i < len(actualOutput); i++ {
			result.addReason("[%d] (%s): '%s'", i+1, actualOutput[i].pos, actualOutput[i].text)
		}
	}

	if expectedIndex < len(expectedOutput) {
		result.addReason("%s:", common.BoldGreen("expected"))
		for i := expectedIndex; i < len(expectedOutput); i++ {
			result.addReason("[%d] (%s): '%s'", i+1, expectedOutput[i].pos, expectedOutput[i].text)
		}
	}
}

func match(lit *string, prefix string) bool {
	if strings.HasPrefix(*lit, prefix) {
		(*lit) = (*lit)[len(prefix):]
		return true
	}
	return false
}

func parseTestDescription(comments []*ir.Comment, result *testResult) (compiler []*testOutputPattern, exe []*testOutputPattern) {
	for _, comment := range comments {
		// Only check single-line comments
		if comment.Tok.Is(token.Comment) {
			raw := strings.TrimSpace(comment.Literal[2:])
			lit := raw
			pattern := &testOutputPattern{pos: comment.Pos, text: raw}

			if match(&lit, "expect") {
				ok := false
				isCompilerOutput := false
				isLineNum := false
				lineNum := comment.Pos.Line

				if match(&lit, "-error") {
					isLineNum = true
					isCompilerOutput = true
					pattern.addPart(common.ErrorMsg.String(), nil)
				} else if match(&lit, "-dgc") {
					isCompilerOutput = true
				}

				if match(&lit, ":") {
					if isLineNum {
						pattern.addPart(fmt.Sprintf("(%d): ", lineNum), nil)
					}
					if err := addPatternParts(lit, comment.Pos, pattern); err != nil {
						result.status = statusInvalid
						result.addReason(err.Error())
						continue
					}
					ok = true
				}

				if ok {
					if isCompilerOutput {
						compiler = append(compiler, pattern)
					} else {
						exe = append(exe, pattern)
					}
				} else {
					result.status = statusInvalid
					result.addReason("bad test description at '%s'", comment.Pos)
				}
			}
		}
	}

	return compiler, exe
}

func addPatternParts(line string, pos token.Position, pattern *testOutputPattern) error {
	line = strings.TrimSpace(line)
	offset := 0
	partOffset := 0
	partLen := 0
	isRegex := false
	for offset < len(line) {
		tag := ""
		if isRegex {
			tag = "</re>"
		} else {
			tag = "<re>"
		}
		hasTag := false
		if strings.HasPrefix(line[offset:], tag) {
			hasTag = true
			offset += len(tag)
		} else {
			offset++
			partLen++
		}
		if hasTag || offset == len(line) {
			partText := line[partOffset : partOffset+partLen]
			var regex *regexp.Regexp
			if isRegex {
				var err error
				partText = strings.TrimSpace(partText)
				regex, err = regexp.Compile(partText)
				if err != nil {
					return fmt.Errorf("bad regex: %s: %s: %s", pos, partText, err)
				}
			}
			pattern.addPart(partText, regex)
			isRegex = !isRegex
			partOffset = offset
			partLen = 0
		}
	}
	return nil
}
