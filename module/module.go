package module

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fmt"

	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/parser"
	"github.com/jhnl/dingo/token"
)

const fileExtension = ".dg"

type include struct {
	inc  *ir.Include
	path string
	file *file
}

type file struct {
	file       *ir.File
	includedBy *file
	includes   []*include
}

func (f *file) path() string {
	return f.file.Path()
}

type loader struct {
	errors      *common.ErrorList
	loadedFiles []*file
}

// Load module and includes.
//
func Load(path string) (*ir.ModuleSet, error) {
	if !strings.HasSuffix(path, fileExtension) {
		return nil, fmt.Errorf("%s does not have file extension %s", path, fileExtension)
	}

	path, err := normalizePath("", path)
	if err != nil {
		return nil, err
	}

	loader := newLoader()

	mod := loader.loadModule(path)
	if mod == nil || loader.errors.IsFatal() {
		return nil, loader.errors
	}

	set := &ir.ModuleSet{}
	set.Modules = append(set.Modules, mod)

	return set, loader.errors
}

func newLoader() *loader {
	l := &loader{}
	l.errors = &common.ErrorList{}
	return l
}

func (l *loader) loadModule(filename string) *ir.Module {
	rootFile, rootDecls, err := parser.ParseFile(filename)
	if err != nil {
		l.errors.AddGeneric(filename, token.NoPosition, err)
		return nil
	}

	mod := &ir.Module{ID: 1}
	mod.Name = rootFile.Ctx.Module
	mod.Path = filename
	mod.Files = append(mod.Files, rootFile)
	mod.Decls = append(mod.Decls, rootDecls...)

	l.loadedFiles = append(l.loadedFiles, &file{file: rootFile})

	var allIncludeDecls []ir.TopDecl
	for i := 0; i < len(l.loadedFiles); i++ {
		srcFile := l.loadedFiles[i]

		if !l.createIncludeList(srcFile) {
			return nil
		}

		for _, inc := range srcFile.includes {
			if inc.file != nil {
				continue
			}

			includeFile, includeDecls, err := parser.ParseFile(inc.path)
			if err != nil {
				l.errors.AddGeneric(inc.path, token.NoPosition, err)
				continue
			}

			mod.Files = append(mod.Files, includeFile)
			allIncludeDecls = append(allIncludeDecls, includeDecls...)
			inc.inc.File = includeFile

			loadedFile := &file{file: includeFile, includedBy: srcFile}
			inc.file = loadedFile
			l.loadedFiles = append(l.loadedFiles, loadedFile)
		}

	}

	mod.Decls = append(mod.Decls, allIncludeDecls...)

	return mod
}

func normalizePath(rel string, path string) (string, error) {
	normPath := filepath.Join(rel, path)
	if !strings.HasPrefix(normPath, "/") {
		normPath = "./" + normPath
	}

	stat, err := os.Stat(normPath)
	if err == nil {
		if stat.IsDir() {
			return "", fmt.Errorf("%s is a directory", normPath)
		}
		return normPath, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	// Failed to find file
	return "", fmt.Errorf("failed to find file %s", normPath)
}

func (l *loader) createIncludeList(loadedFile *file) bool {
	parentDir := filepath.Dir(loadedFile.path())

	for _, inc := range loadedFile.file.Includes {
		unquoted, err := strconv.Unquote(inc.Literal.Literal)
		if err != nil {
			l.errors.AddGeneric(loadedFile.path(), inc.Literal.Pos, err)
			break
		}

		if len(unquoted) == 0 {
			l.errors.AddTrace(loadedFile.path(), inc.Literal.Pos, l.getIncludedByTrace(loadedFile), "invalid path")
			continue
		}

		includePath, err := normalizePath(parentDir, unquoted)
		if err != nil {
			l.errors.AddGeneric(loadedFile.path(), inc.Literal.Pos, err)
			continue
		}

		foundFile := l.findLoadedFile(includePath)

		if foundFile != nil {
			var traceLines []string
			if checkIncludeCycle(foundFile, loadedFile.path(), &traceLines) {
				trace := common.NewTrace(fmt.Sprintf("%s includes:", loadedFile.path()), nil)
				for i := len(traceLines) - 1; i >= 0; i-- {
					trace.Lines = append(trace.Lines, traceLines[i])
				}
				l.errors.AddTrace(loadedFile.path(), inc.Literal.Pos, trace, "include cycle detected")
				continue
			}
			inc.File = foundFile.file
		}

		loadedFile.includes = append(loadedFile.includes, &include{file: foundFile, inc: inc, path: includePath})
	}

	if l.errors.IsFatal() {
		return false
	}

	return true
}

func (l *loader) findLoadedFile(path string) *file {
	for _, file := range l.loadedFiles {
		if file.path() == path {
			return file
		}
	}
	return nil
}

func (l *loader) getIncludedByTrace(loadedFile *file) common.Trace {
	var trace []string
	for file := loadedFile; file != nil; {
		trace = append(trace, file.path())
		file = file.includedBy
	}
	return common.NewTrace("included by:", trace)
}

func checkIncludeCycle(loadedFile *file, includePath string, trace *[]string) bool {
	if loadedFile.path() == includePath {
		*trace = append(*trace, loadedFile.path())
		return true
	}

	for _, inc := range loadedFile.includes {
		if inc.file == nil {
			continue
		}
		if checkIncludeCycle(inc.file, includePath, trace) {
			*trace = append(*trace, loadedFile.path())
			return true
		}
	}
	return false
}
