package module

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fmt"

	"github.com/jhnl/interpreter/common"
	"github.com/jhnl/interpreter/parser"
	"github.com/jhnl/interpreter/semantics"
	"github.com/jhnl/interpreter/token"
)

const langExtension = ".lang"
const packageFile = "package" + langExtension

type color int

const (
	white color = iota
	gray
	black
)

type moduleImport struct {
	imp  *semantics.Import
	path string
	mod  *loadedModule // Remove since imp.mod can be used instead
}

type loadedFile struct {
	file       *semantics.File
	importedBy *loadedFile
	imports    []*moduleImport
}

type loadedModule struct {
	mod   *semantics.Module
	files []*loadedFile
	color color
}

type loader struct {
	errors        *common.ErrorList
	currentFile   *loadedFile
	loadedModules []*loadedModule
}

// Load module and imports.
//
func Load(path string) (*semantics.Program, error) {
	if !strings.HasSuffix(path, langExtension) {
		return nil, fmt.Errorf("not a lang file")
	}

	loader := &loader{}
	loader.errors = &common.ErrorList{}

	path, err := normalizePath("", path, false)
	if err != nil {
		return nil, err
	}

	mainModule := loader.loadModule(path)
	if mainModule == nil {
		return nil, loader.errors
	}

	loader.loadedModules = append(loader.loadedModules, mainModule)

	for i := 0; i < len(loader.loadedModules); i++ {
		loadedMod := loader.loadedModules[i]
		for _, loadedFile := range loadedMod.files {
			loader.currentFile = loadedFile

			parentDir := filepath.Dir(loadedFile.file.Path)
			loadedFile.imports = loader.createModuleImports(loadedFile.file, parentDir)

			for _, currentImport := range loadedFile.imports {
				if currentImport.mod != nil {
					continue
				}

				mod := loader.loadModule(currentImport.path)
				if mod == nil {
					break
				}

				for _, importedFile := range mod.files {
					importedFile.importedBy = loader.currentFile
				}

				currentImport.mod = mod
				currentImport.imp.Mod = currentImport.mod.mod
				loader.loadedModules = append(loader.loadedModules, mod)
			}
		}
	}

	if loader.errors.IsFatal() {
		return nil, loader.errors
	}

	var modules []*semantics.Module
	if !loader.getModules(&modules) {
		// This shouldn't actually happen since cycles are checked when loading imports
		panic("cycle detected")
	}

	return &semantics.Program{Main: mainModule.mod, Modules: modules}, loader.errors
}

func (l *loader) loadModule(filename string) *loadedModule {
	mainFile, err := parser.ParseFile(filename)
	if err != nil {
		l.errors.AddGeneric(filename, token.NoPosition, err)
		return nil
	}

	if !mainFile.Decl.IsValid() {
		l.errors.AddTrace(filename, token.NoPosition, l.getImportedByTrace(), "not declared as a module")
		return nil
	}

	mainFile.Path = filename
	mod := &semantics.Module{}
	mod.Files = append(mod.Files, mainFile)
	loadedMod := &loadedModule{mod: mod}
	loadedMod.files = append(loadedMod.files, &loadedFile{file: mainFile})

	moduleDir := filepath.Dir(filename)
	moduleName := filepath.Base(filename)

	if moduleName != packageFile {
		moduleName = strings.Replace(moduleName, langExtension, "", -1)
		mod.Name = token.Synthetic(token.Ident, moduleName)
		mod.Path = filename
		return loadedMod
	}

	moduleName = filepath.Base(moduleDir)
	mod.Name = token.Synthetic(token.Ident, moduleName)
	mod.Path = moduleDir

	files, err := ioutil.ReadDir(moduleDir)
	if err != nil {
		l.errors.AddGeneric(filename, token.NoPosition, err)
		return nil
	}

	var packageFiles []string

	for _, f := range files {
		if filepath.Ext(f.Name()) != langExtension || f.Name() == packageFile {
			continue
		}
		packageFiles = append(packageFiles, filepath.Join(moduleDir, f.Name()))
	}

	for _, f := range packageFiles {
		file, err := parser.ParseFile(f)

		if file.Decl.IsValid() {
			l.errors.AddNonFatal(f, token.NoPosition, "ignoring file as it's not part of module package %s (declared as a separate module)", moduleName)
		} else {
			if err != nil {
				l.errors.AddGeneric(f, token.NoPosition, err)
			}
			file.Path = f
			mod.Files = append(mod.Files, file)
			loadedMod.files = append(loadedMod.files, &loadedFile{file: file})
		}
	}

	return loadedMod
}

func (l *loader) createModuleImports(file *semantics.File, dir string) []*moduleImport {
	var imports []*moduleImport

	for _, imp := range file.Imports {
		unquoted, err := strconv.Unquote(imp.Literal.Literal)
		if err != nil {
			l.errors.AddGeneric(file.Path, imp.Literal.Pos, err)
			break
		}

		if len(unquoted) == 0 {
			l.errors.AddTrace(file.Path, imp.Literal.Pos, l.getImportedByTrace(), "invalid path")
			continue
		} else if unquoted[0] == '/' {
			l.errors.AddTrace(file.Path, imp.Literal.Pos, l.getImportedByTrace(), "import path cannot be absolute")
			continue
		}

		norm, err := normalizePath(dir, unquoted, true)
		if err != nil {
			l.errors.AddGeneric(file.Path, imp.Literal.Pos, err)
		}

		foundMod, foundFile := l.findLoadedModule(norm)
		if foundFile != nil && !foundFile.file.Decl.IsValid() {
			l.errors.AddTrace(file.Path, imp.Literal.Pos, l.getImportedByTrace(), "import %s does not designate a valid module", imp.Literal.Literal)
			continue
		}

		if foundMod != nil {
			var traceLines []string
			if checkImportCycle(foundMod, file.Path, &traceLines) {
				trace := common.NewTrace(fmt.Sprintf("%s imports:", imp.Literal.Literal), nil)
				for i := len(traceLines) - 2; i >= 0; i-- {
					trace.Lines = append(trace.Lines, traceLines[i])
				}
				l.errors.AddTrace(file.Path, imp.Literal.Pos, trace, "import cycle detected")
				continue
			}
			imp.Mod = foundMod.mod
		}

		imports = append(imports, &moduleImport{mod: foundMod, imp: imp, path: norm})
	}

	if l.errors.IsFatal() {
		return nil
	}

	return imports
}

func (l *loader) findLoadedModule(importPath string) (*loadedModule, *loadedFile) {
	for _, loadedMod := range l.loadedModules {
		for _, loadedFile := range loadedMod.files {
			if loadedFile.file.Path == importPath {
				return loadedMod, loadedFile
			}
		}
		if loadedMod.mod.Path == importPath {
			return loadedMod, nil
		}
	}
	return nil, nil
}

func (l *loader) getImportedByTrace() common.Trace {
	var trace []string
	for file := l.currentFile; file != nil; {
		trace = append(trace, file.file.Path)
		file = file.importedBy
	}
	return common.NewTrace("imported by:", trace)
}

func (l *loader) getModules(modules *[]*semantics.Module) bool {
	for _, mod := range l.loadedModules {
		mod.color = white
	}
	for _, mod := range l.loadedModules {
		if mod.color == white {
			if !dfs(mod, modules) {
				return false
			}
		}
	}
	return true
}

// Returns false if a cycle is detected.
func dfs(loadedModule *loadedModule, modules *[]*semantics.Module) bool {
	loadedModule.color = gray
	for _, loadedFile := range loadedModule.files {
		for _, imp := range loadedFile.imports {
			if imp.mod.color == gray {
				return false
			}
			if imp.mod.color == white {
				if !dfs(imp.mod, modules) {
					return false
				}
			}
		}
	}
	loadedModule.color = black
	*modules = append(*modules, loadedModule.mod)
	return true
}

func normalizePath(rel string, path string, appendExtension bool) (string, error) {
	absPath, err := filepath.Abs(filepath.Join(rel, path))
	if err != nil {
		return "", err
	}

	absFilePath := absPath
	absPackagePath := absPath

	if appendExtension {
		absFilePath += langExtension
		absPackagePath = filepath.Join(absPath, packageFile)
	}

	filePathStat, filePathErr := os.Stat(absFilePath)
	if filePathErr == nil && !filePathStat.IsDir() {
		// It's a file
		return absFilePath, nil
	} else if !os.IsNotExist(filePathErr) {
		return "", filePathErr
	}

	if appendExtension {
		packagePathStat, packagePathErr := os.Stat(absPackagePath)
		if packagePathErr == nil && !packagePathStat.IsDir() {
			// It's a package
			return absPackagePath, nil
		} else if !os.IsNotExist(packagePathErr) {
			return "", packagePathErr
		}
	}

	// Failed to find file or package
	return "", fmt.Errorf("%s is not a valid file or package module", absPath)
}

func checkImportCycle(loadedMod *loadedModule, importPath string, trace *[]string) bool {
	if loadedMod.mod.Path == importPath {
		*trace = append(*trace, loadedMod.mod.Path)
		return true
	}
	for _, loadedFile := range loadedMod.files {
		if loadedFile.file.Path == importPath {
			*trace = append(*trace, loadedFile.file.Path)
			return true
		}
		for _, imp := range loadedFile.imports {
			if imp.mod == nil {
				continue
			}
			if checkImportCycle(imp.mod, importPath, trace) {
				*trace = append(*trace, loadedFile.file.Path)
				return true
			}
		}
	}
	return false
}
