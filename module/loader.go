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

var emptyPath = includePath{token.NoPosition, token.NoPosition}

type includePath struct {
	canonical token.Position // Used to determine if two paths refer to the same file
	actual    token.Position // The actual (cleaned) path in the code
}

type fileDependency struct {
	dep  *ir.FileDependency
	path includePath
	file *file
}

type file struct {
	file       *ir.File
	path       includePath
	requiredBy *file
	deps       []*fileDependency
}

type loader struct {
	errors      *common.ErrorList
	cwd         string
	loadedFiles []*file
}

// Load modules.
func Load(filenames []string) (*ir.ModuleSet, error) {
	loader := newLoader()
	set := ir.NewModuleSet()

	for _, filename := range filenames {
		mod := loader.load(filename)
		if mod != nil {
			if existing, ok := set.Modules[mod.FQN]; ok {
				loader.errors.Add(token.NoPosition, "name conflict for module '%s': %s and %s",
					mod.FQN, existing.Path.Filename, mod.Path.Filename)
			} else {
				set.Modules[mod.FQN] = mod
			}
		}
	}

	if set == nil || loader.errors.IsError() {
		return nil, loader.errors
	}

	return set, nil
}

func newLoader() *loader {
	l := &loader{}
	l.errors = &common.ErrorList{}
	return l
}

func (l *loader) load(filename string) *ir.Module {
	if !strings.HasSuffix(filename, fileExtension) {
		err := fmt.Errorf("%s does not have file extension %s", filename, fileExtension)
		l.errors.AddGeneric1(err)
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		l.errors.AddGeneric1(err)
		return nil
	}

	normPath, err := normalizePath(cwd, "", filename)
	if err != nil {
		l.errors.AddGeneric1(err)
		return nil
	}

	l.cwd = cwd
	l.loadedFiles = nil
	return l.loadModule(normPath)
}

func (l *loader) loadModule(path includePath) *ir.Module {
	rootFile, rootDecls, err := parser.ParseFile(path.actual.Filename)
	if err != nil {
		l.errors.AddGeneric3(path.actual, err)
		return nil
	}

	if rootFile.ModName == nil {
		rootFile.ModName = ir.NewIdent2(token.Synthetic(token.Ident), "main")
	}

	mod := &ir.Module{}
	mod.FQN = ir.ExprToModuleFQN(rootFile.ModName)
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

			depFile, depDecls, err := parser.ParseFile(dep.path.actual.Filename)
			if err != nil {
				l.errors.AddGeneric3(dep.path.actual, err)
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

func normalizePath(cwd string, rel string, path string) (includePath, error) {
	actual := ""
	canonical := ""
	if filepath.IsAbs(path) {
		actual = filepath.Clean(path)
		canonical = actual
	} else {
		actual = filepath.Join(rel, path)
		if !strings.HasPrefix(actual, ".") {
			actual = "./" + actual
		}
		canonical = filepath.Join(cwd, rel, path)
	}

	stat, err := os.Stat(actual)
	if err == nil {
		if stat.IsDir() {
			return emptyPath, fmt.Errorf("'%s' is a directory", actual)
		}

		normPath := includePath{}
		normPath.actual = token.NewPosition1(actual)
		normPath.canonical = token.NewPosition1(canonical)

		return normPath, nil
	} else if !os.IsNotExist(err) {
		return emptyPath, err
	}

	// Failed to find file
	return emptyPath, fmt.Errorf("failed to find file '%s'", actual)
}

func (l *loader) createDependencyList(loadedFile *file) bool {
	parentDir := filepath.Dir(loadedFile.path.actual.Filename)

	for _, dep := range loadedFile.file.FileDeps {
		unquoted, err := strconv.Unquote(dep.Literal.Value)
		if err != nil {
			l.errors.AddGeneric3(dep.Literal.Tok.Pos, err)
			break
		}

		if len(unquoted) == 0 {
			l.errors.AddTrace(dep.Literal.Tok.Pos, l.getRequiredByTrace(loadedFile), "invalid path")
			continue
		}

		normPath, err := normalizePath(l.cwd, parentDir, unquoted)
		if err != nil {
			l.errors.AddGeneric3(dep.Literal.Tok.Pos, err)
			continue
		}

		foundFile := l.findLoadedFile(normPath.canonical.Filename)
		loadedFile.deps = append(loadedFile.deps, &fileDependency{file: foundFile, dep: dep, path: normPath})
	}

	if l.errors.IsError() {
		return false
	}

	return true
}

func (l *loader) findLoadedFile(path string) *file {
	for _, file := range l.loadedFiles {
		if file.path.canonical.Filename == path {
			return file
		}
	}
	return nil
}

func (l *loader) getRequiredByTrace(loadedFile *file) common.Trace {
	var trace []string
	for file := loadedFile; file != nil; {
		trace = append(trace, file.path.actual.Filename)
		file = file.requiredBy
	}
	return common.NewTrace("required by:", trace)
}
