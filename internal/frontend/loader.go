package frontend

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fmt"

	"github.com/jhnl/dingo/internal/common"
	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

const fileExtension = ".dg"

type dgFile struct {
	srcPos            token.Position // Position in parent where file was included
	path              token.Position // The actual (cleaned) path in the code
	canonicalPath     token.Position // Used to determine if two paths refer to the same file
	parsedFile        *ir.File
	parent            *dgFile
	children          []*dgFile
	parentModuleIndex int
}

type loader struct {
	errors *common.ErrorList
	cwd    string
}

// Load files and includes.
func Load(filenames []string) (ir.FileMatrix, error) {
	l := newLoader()
	cwd, err := os.Getwd()
	if err != nil {
		l.errors.AddGeneric1(err)
		return nil, l.errors
	}
	l.cwd = cwd
	var matrix ir.FileMatrix
	for _, filename := range filenames {
		list := l.loadFileList(filename)
		if list != nil {
			matrix = append(matrix, list)
		}
	}
	return matrix, l.errors
}

func newLoader() *loader {
	l := &loader{}
	l.errors = &common.ErrorList{}
	return l
}

func (l *loader) loadFileList(filename string) ir.FileList {
	if !strings.HasSuffix(filename, fileExtension) {
		l.errors.AddGeneric1(fmt.Errorf("%s does not have file extension %s", filename, fileExtension))
		return nil
	}

	root, err := dgFileFromPath(token.NoPosition, l.cwd, "", filename)
	if err != nil {
		l.errors.AddGeneric1(err)
		return nil
	}

	root.parsedFile, err = parseFile(root.path.Filename)

	if err != nil {
		l.errors.AddGeneric2(root.path, err)
		return nil
	}

	root.parsedFile.ParentIndex1 = 0
	root.parsedFile.ParentIndex2 = 0

	if !l.createIncludeList(root) {
		return nil
	}

	var fileList ir.FileList
	var loadedFiles []*dgFile
	loadedFiles = append(loadedFiles, root)

	for fileID := 0; fileID < len(loadedFiles); fileID++ {
		file := loadedFiles[fileID]
		for _, child := range file.children {
			child.parsedFile, err = parseFile(child.path.Filename)
			if err != nil {
				l.errors.AddGeneric2(child.path, err)
				continue
			}
			child.parsedFile.ParentIndex1 = fileID
			child.parsedFile.ParentIndex2 = child.parentModuleIndex
			if !l.createIncludeList(child) {
				continue
			}
			loadedFiles = append(loadedFiles, child)
		}
		fileList = append(fileList, file.parsedFile)
	}

	return fileList
}

func dgFileFromPath(src token.Position, cwd string, dir string, filename string) (*dgFile, error) {
	path := filename
	canonicalPath := filename
	if !filepath.IsAbs(filename) {
		path = filepath.Join(dir, filename)
		canonicalPath = filepath.Join(cwd, path)
		if !strings.HasPrefix(path, ".") {
			path = "./" + path
		}
	}
	path = filepath.Clean(path)
	canonicalPath = filepath.Clean(canonicalPath)
	if stat, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			// Failed to find file
			return nil, fmt.Errorf("failed to find file '%s'", path)
		}
		return nil, err
	} else if stat.IsDir() {
		return nil, fmt.Errorf("'%s' is a directory", path)
	}
	file := &dgFile{
		srcPos:        src,
		path:          token.NewPosition1(path),
		canonicalPath: token.NewPosition1(canonicalPath),
	}
	return file, nil
}

func (l *loader) createIncludeList(parent *dgFile) bool {
	parentDir := filepath.Dir(parent.path.Filename)
	ok := true

	for modIndex, mod := range parent.parsedFile.Modules {
		for _, includeLit := range mod.Includes {
			unquoted, err := strconv.Unquote(includeLit.Value)
			if err != nil {
				panic(fmt.Sprintf("%s at %s", err, includeLit.Pos()))
			}

			if len(unquoted) == 0 {
				trace := getIncludedByTrace(parent)
				if len(trace) > 1 {
					l.errors.AddContext(includeLit.Pos(), formatIncludeTrace(trace), "invalid path")
				} else {
					l.errors.Add(includeLit.Pos(), "invalid path")
				}
				ok = false
				break
			}

			child, err := dgFileFromPath(includeLit.Pos(), l.cwd, parentDir, unquoted)
			if err != nil {
				l.errors.AddGeneric2(includeLit.Pos(), err)
				ok = false
				break
			}

			if trace, ok := checkIncludeCycle(parent, child); ok {
				l.errors.AddContext(includeLit.Pos(), formatIncludeTrace(trace), "cycle detected")
				ok = false
				break
			}

			child.parent = parent
			child.parentModuleIndex = modIndex
			parent.children = append(parent.children, child)
		}
	}

	return ok
}

func checkIncludeCycle(parent *dgFile, child *dgFile) ([]*dgFile, bool) {
	var trace []*dgFile
	file := parent
	for file != nil {
		trace = append(trace, file)
		if file.canonicalPath.Filename == child.canonicalPath.Filename {
			return trace, true
		}
		file = file.parent
	}
	return trace, false
}

func getIncludedByTrace(file *dgFile) []*dgFile {
	var trace []*dgFile
	for file != nil {
		trace = append(trace, file)
		file = file.parent
	}
	return trace
}

func formatIncludeTrace(trace []*dgFile) []string {
	var res []string
	next := 1
	if next == len(trace) {
		next = 0
	}
	res = append(res, fmt.Sprintf("  >> [%d] ^ included by [%d]", 0, next))
	for i := 1; i < len(trace); i++ {
		next := i + 1
		if next == len(trace) {
			next = 0
		}
		line := fmt.Sprintf("  >> [%d] %s included by [%d]", i, trace[i-1].srcPos, next)
		res = append(res, line)
	}
	return res
}
