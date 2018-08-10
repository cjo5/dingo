package common

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"

	"github.com/cjo5/dingo/internal/token"
)

// BuildContext contains config options and state for the current build.
type BuildContext struct {
	Cwd             string
	FileMap         map[string]*token.File
	Errors          *ErrorList
	ErrorCheckpoint int
	Verbose         bool
	LLVMIR          bool
	Exe             string
}

func NewBuildContext(cwd string) *BuildContext {
	return &BuildContext{
		Cwd:     cwd,
		FileMap: make(map[string]*token.File),
		Errors:  &ErrorList{},
	}
}

func (ctx *BuildContext) LookupFile(filename string) *token.File {
	filename = token.Abs(ctx.Cwd, filename)
	if found, ok := ctx.FileMap[filename]; ok {
		return found
	}
	return nil
}

func (ctx *BuildContext) NewFile(filename string, src []byte) *token.File {
	file := &token.File{Filename: filename, Src: src}
	key := token.Abs(ctx.Cwd, filename)
	ctx.FileMap[key] = file
	return file
}

func (ctx *BuildContext) SetCheckpoint() {
	ctx.ErrorCheckpoint = len(ctx.Errors.Errors)
}

func (ctx *BuildContext) IsErrorSinceCheckpoint() bool {
	return len(ctx.Errors.Errors) > ctx.ErrorCheckpoint
}

func (ctx *BuildContext) IsError() bool {
	return ctx.Errors.IsError()
}

func (ctx *BuildContext) FormatErrors() {
	ctx.Errors.Sort()
	ctx.SetErrorLocations()
}

func (ctx *BuildContext) SetErrorLocations() {
	cache := make(fileLinesCache)
	ctx.setErrorLocations2(cache, ctx.Errors.Warnings)
	ctx.setErrorLocations2(cache, ctx.Errors.Errors)
}

type fileLinesCache map[string][]string

func (ctx *BuildContext) fileLines(cache fileLinesCache, filename string) []string {
	key := token.Abs(ctx.Cwd, filename)
	if found, ok := cache[key]; ok {
		return found
	}
	file := ctx.FileMap[key]
	reader := bytes.NewReader(file.Src)
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	cache[key] = lines
	return lines
}

var notWSRegex *regexp.Regexp

func init() {
	var err error
	notWSRegex, err = regexp.Compile("\\S")
	if err != nil {
		panic(err)
	}
}

func (ctx *BuildContext) setErrorLocations2(cache fileLinesCache, errors []*Error) {
	var lines []string
	filename := ""
	for _, e := range errors {
		if len(e.Context) > 0 || !e.Pos.IsValid() {
			continue
		}
		if e.Pos.Filename != filename {
			lines = nil
			filename = e.Pos.Filename
			if len(filename) > 0 {
				lines = ctx.fileLines(cache, filename)
			}
		}
		linePos := e.Pos.Line - 1
		if linePos < 0 || linePos >= len(lines) {
			continue
		}
		line := lines[linePos]
		lineLen := len(line)
		if lineLen > 200 {
			line = line[:200]
			lineLen = len(line)
			line += "..."
		}
		columnPos := e.Pos.Column - 1
		if columnPos < 0 || columnPos >= lineLen {
			continue
		}
		mark := notWSRegex.ReplaceAllString(line[:columnPos], " ")
		markCount := (e.EndPos.Column - 1) - columnPos
		if e.EndPos.Line == e.Pos.Line && markCount > 1 && markCount < lineLen {
			mark += BoldGreen(strings.Repeat("~", markCount))
		} else {
			mark += BoldGreen("^")
		}
		e.Context = append(e.Context, line)
		e.Context = append(e.Context, mark)
	}
}
