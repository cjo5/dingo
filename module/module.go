package module

import (
	"io/ioutil"
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
const packageFile = "package" + fileExtension

type moduleImport struct {
	imp  *ir.Import
	path string
	mod  *loadedModule
}

type loadedFile struct {
	file       *ir.File
	importedBy *loadedFile
	imports    []*moduleImport
}

type loadedModule struct {
	mod   *ir.Module
	files []*loadedFile
}

type loader struct {
	errors        *common.ErrorList
	moduleID      int
	currentFile   *loadedFile
	loadedModules []*loadedModule
}

// Load module and imports.
//
func Load(path string) (*ir.ModuleSet, error) {
	if !strings.HasSuffix(path, fileExtension) {
		return nil, fmt.Errorf("%s is not a dingo file", path)
	}

	loader := &loader{moduleID: 1} // ID 0 is reserved
	loader.errors = &common.ErrorList{}

	path, err := normalizePath("", path, false)
	if err != nil {
		return nil, err
	}

	mainModule := loader.loadModule(path)
	if mainModule == nil {
		return nil, loader.errors
	}

	mainModule.mod.Flags |= ir.ModFlagMain
	loader.loadedModules = append(loader.loadedModules, mainModule)

	for i := 0; i < len(loader.loadedModules); i++ {
		loadedMod := loader.loadedModules[i]
		for _, loadedFile := range loadedMod.files {
			loader.currentFile = loadedFile

			parentDir := filepath.Dir(loadedFile.file.Ctx.Path)
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

	set := &ir.ModuleSet{}
	for _, loadedMod := range loader.loadedModules {
		set.Modules = append(set.Modules, loadedMod.mod)
	}

	return set, loader.errors
}

func (l *loader) loadModule(filename string) *loadedModule {
	mainFile, mainDecls, err := parser.ParseFile(filename)
	if err != nil {
		l.errors.AddGeneric(filename, token.NoPosition, err)
		return nil
	}

	if !mainFile.Ctx.Decl.IsValid() {
		l.errors.AddTrace(filename, token.NoPosition, l.getImportedByTrace(), "not declared as a module")
		return nil
	}

	mod := &ir.Module{ID: l.moduleID}
	l.moduleID++
	mod.Files = append(mod.Files, mainFile)
	mod.Decls = append(mod.Decls, mainDecls...)
	loadedMod := &loadedModule{mod: mod}
	loadedMod.files = append(loadedMod.files, &loadedFile{file: mainFile})

	moduleDir := filepath.Dir(filename)
	moduleName := filepath.Base(filename)

	if moduleName != packageFile {
		moduleName = strings.Replace(moduleName, fileExtension, "", -1)
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
		if filepath.Ext(f.Name()) != fileExtension || f.Name() == packageFile {
			continue
		}
		packageFiles = append(packageFiles, filepath.Join(moduleDir, f.Name()))
	}

	for _, f := range packageFiles {
		file, decls, err := parser.ParseFile(f)

		if file.Ctx.Decl.IsValid() {
			l.errors.AddNonFatal(f, token.NoPosition, "ignoring file as it's not part of module package %s (declared as a separate module)", moduleName)
		} else {
			if err != nil {
				l.errors.AddGeneric(f, token.NoPosition, err)
			}
			file.Ctx.Path = f
			mod.Files = append(mod.Files, file)
			mod.Decls = append(mod.Decls, decls...)
			loadedMod.files = append(loadedMod.files, &loadedFile{file: file})
		}
	}

	return loadedMod
}

func (l *loader) createModuleImports(file *ir.File, dir string) []*moduleImport {
	var imports []*moduleImport

	for _, imp := range file.Imports {
		unquoted, err := strconv.Unquote(imp.Literal.Literal)
		if err != nil {
			l.errors.AddGeneric(file.Ctx.Path, imp.Literal.Pos, err)
			break
		}

		if len(unquoted) == 0 {
			l.errors.AddTrace(file.Ctx.Path, imp.Literal.Pos, l.getImportedByTrace(), "invalid path")
			continue
		} else if unquoted[0] == '/' {
			l.errors.AddTrace(file.Ctx.Path, imp.Literal.Pos, l.getImportedByTrace(), "import path cannot be absolute")
			continue
		}

		norm, err := normalizePath(dir, unquoted, true)
		if err != nil {
			l.errors.AddGeneric(file.Ctx.Path, imp.Literal.Pos, err)
		}

		foundMod, foundFile := l.findLoadedModule(norm)
		if foundFile != nil && !foundFile.file.Ctx.Decl.IsValid() {
			l.errors.AddTrace(file.Ctx.Path, imp.Literal.Pos, l.getImportedByTrace(), "import %s does not designate a valid module", imp.Literal.Literal)
			continue
		}

		if foundMod != nil {
			var traceLines []string
			if checkImportCycle(foundMod, file.Ctx.Path, &traceLines) {
				trace := common.NewTrace(fmt.Sprintf("%s imports:", file.Ctx.Path), nil)
				for i := len(traceLines) - 1; i >= 0; i-- {
					trace.Lines = append(trace.Lines, traceLines[i])
				}
				l.errors.AddTrace(file.Ctx.Path, imp.Literal.Pos, trace, "import cycle detected")
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
			if loadedFile.file.Ctx.Path == importPath {
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
		trace = append(trace, file.file.Ctx.Path)
		file = file.importedBy
	}
	return common.NewTrace("imported by:", trace)
}

func normalizePath(rel string, path string, appendExtension bool) (string, error) {
	/*absPath, err := filepath.Abs(filepath.Join(rel, path))
	if err != nil {
		return "", err
	}*/
	absPath := filepath.Join(rel, path)
	if !strings.HasPrefix(absPath, "/") {
		absPath = "./" + absPath
	}

	absFilePath := absPath
	absPackagePath := absPath

	if appendExtension {
		absFilePath += fileExtension
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
	return "", fmt.Errorf("failed to find file or package module %s", absPath)
}

func checkImportCycle(loadedMod *loadedModule, importPath string, trace *[]string) bool {
	if loadedMod.mod.Path == importPath {
		*trace = append(*trace, loadedMod.mod.Path)
		return true
	}
	for _, loadedFile := range loadedMod.files {
		if loadedFile.file.Ctx.Path == importPath {
			*trace = append(*trace, loadedFile.file.Ctx.Path)
			return true
		}
		for _, imp := range loadedFile.imports {
			if imp.mod == nil {
				continue
			}
			if checkImportCycle(imp.mod, importPath, trace) {
				*trace = append(*trace, loadedFile.file.Ctx.Path)
				return true
			}
		}
	}
	return false
}
