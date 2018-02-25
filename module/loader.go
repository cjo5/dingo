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

var emptyPath = requirePath{"", ""}

type requirePath struct {
	canonical string // Used to determine if two paths refer to the same file
	actual    string // The actual (cleaned) path in the code
}

type fileDependency struct {
	dep  *ir.FileDependency
	path requirePath
	file *file
}

type file struct {
	file       *ir.File
	path       requirePath
	requiredBy *file
	deps       []*fileDependency
}

type loader struct {
	errors      *common.ErrorList
	cwd         string
	loadedFiles []*file
}

// Load module and includes.
//
func Load(path string) (*ir.Module, error) {
	if !strings.HasSuffix(path, fileExtension) {
		return nil, fmt.Errorf("%s does not have file extension %s", path, fileExtension)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	normPath, err := normalizePath(cwd, "", path)
	if err != nil {
		return nil, err
	}

	loader := newLoader()
	loader.cwd = cwd

	mod := loader.loadModule(normPath)
	if mod == nil || loader.errors.IsError() {
		return nil, loader.errors
	}

	return mod, loader.errors
}

func newLoader() *loader {
	l := &loader{}
	l.errors = &common.ErrorList{}
	return l
}

func (l *loader) loadModule(path requirePath) *ir.Module {
	rootFile, rootDecls, err := parser.ParseFile(path.actual)
	if err != nil {
		l.errors.AddGeneric3(path.actual, token.NoPosition, err)
		return nil
	}

	mod := &ir.Module{}
	mod.FQN = ""
	if rootFile.Ctx.ModName != nil {
		mod.FQN = ir.ExprToModuleFQN(rootFile.Ctx.ModName)
	}
	mod.Path = path.actual
	mod.Files = append(mod.Files, rootFile)
	mod.Decls = append(mod.Decls, rootDecls...)

	l.loadedFiles = append(l.loadedFiles, &file{file: rootFile, path: path})

	var allDepDecls []ir.TopDecl
	for i := 0; i < len(l.loadedFiles); i++ {
		srcFile := l.loadedFiles[i]

		if !l.createDependencyList(srcFile) {
			return nil
		}

		for _, dep := range srcFile.deps {
			if dep.file != nil {
				continue
			}

			depFile, depDecls, err := parser.ParseFile(dep.path.actual)
			if err != nil {
				l.errors.AddGeneric3(dep.path.actual, token.NoPosition, err)
				continue
			}

			mod.Files = append(mod.Files, depFile)
			allDepDecls = append(allDepDecls, depDecls...)

			loadedFile := &file{file: depFile, path: dep.path, requiredBy: srcFile}
			dep.file = loadedFile
			l.loadedFiles = append(l.loadedFiles, loadedFile)
		}
	}

	mod.Decls = append(mod.Decls, allDepDecls...)

	return mod
}

func normalizePath(cwd string, rel string, path string) (requirePath, error) {
	normPath := emptyPath
	if filepath.IsAbs(path) {
		normPath.actual = filepath.Clean(path)
		normPath.canonical = normPath.actual
	} else {
		normPath.actual = filepath.Join(rel, path)
		if !strings.HasPrefix(normPath.actual, ".") {
			normPath.actual = "./" + normPath.actual
		}
		normPath.canonical = filepath.Join(cwd, rel, path)
	}

	stat, err := os.Stat(normPath.actual)
	if err == nil {
		if stat.IsDir() {
			return emptyPath, fmt.Errorf("%s is a directory", normPath.actual)
		}
		return normPath, nil
	} else if !os.IsNotExist(err) {
		return emptyPath, err
	}

	// Failed to find file
	return emptyPath, fmt.Errorf("failed to find file %s", normPath.actual)
}

func (l *loader) createDependencyList(loadedFile *file) bool {
	parentDir := filepath.Dir(loadedFile.path.actual)

	for _, dep := range loadedFile.file.FileDeps {
		unquoted, err := strconv.Unquote(dep.Literal.Literal)
		if err != nil {
			l.errors.AddGeneric3(loadedFile.path.actual, dep.Literal.Pos, err)
			break
		}

		if len(unquoted) == 0 {
			l.errors.AddTrace(loadedFile.path.actual, dep.Literal.Pos, dep.Literal.Pos, l.getRequiredByTrace(loadedFile), "invalid path")
			continue
		}

		normPath, err := normalizePath(l.cwd, parentDir, unquoted)
		if err != nil {
			l.errors.AddGeneric3(loadedFile.path.actual, dep.Literal.Pos, err)
			continue
		}

		foundFile := l.findLoadedFile(normPath.canonical)
		loadedFile.deps = append(loadedFile.deps, &fileDependency{file: foundFile, dep: dep, path: normPath})
	}

	if l.errors.IsError() {
		return false
	}

	return true
}

func (l *loader) findLoadedFile(path string) *file {
	for _, file := range l.loadedFiles {
		if file.path.canonical == path {
			return file
		}
	}
	return nil
}

func (l *loader) getRequiredByTrace(loadedFile *file) common.Trace {
	var trace []string
	for file := loadedFile; file != nil; {
		trace = append(trace, file.path.actual)
		file = file.requiredBy
	}
	return common.NewTrace("required by:", trace)
}
