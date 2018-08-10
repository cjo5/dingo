package frontend

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fmt"

	"github.com/cjo5/dingo/internal/common"
	"github.com/cjo5/dingo/internal/ir"
	"github.com/cjo5/dingo/internal/token"
)

const fileExtension = ".dg"

type dgFile struct {
	srcPos            token.Position // Position in parent where file was included
	path              token.Position // The actual (cleaned) path in the code
	absPath           token.Position // Used to determine if two paths refer to the same file
	parsedFile        *ir.File
	parent            *dgFile
	children          []*dgFile
	parentModuleIndex int
}

// Load files and includes.
func Load(ctx *common.BuildContext, filenames []string) (ir.FileMatrix, bool) {
	var matrix ir.FileMatrix
	ctx.SetCheckpoint()
	for _, filename := range filenames {
		list := loadFileList(ctx, filename)
		if list != nil {
			matrix = append(matrix, list)
		}
	}
	return matrix, !ctx.IsErrorSinceCheckpoint()
}

func loadFileList(ctx *common.BuildContext, filename string) ir.FileList {
	if !strings.HasSuffix(filename, fileExtension) {
		ctx.Errors.AddGeneric1(fmt.Errorf("%s does not have file extension %s", filename, fileExtension))
		return nil
	}

	root, err := dgFileFromPath(token.NoPosition, ctx.Cwd, "", filename)
	if err != nil {
		ctx.Errors.AddGeneric1(err)
		return nil
	}

	root.parsedFile = loadFile(ctx, root.path)
	if root.parsedFile == nil {
		return nil
	}

	root.parsedFile.ParentIndex1 = 0
	root.parsedFile.ParentIndex2 = 0

	if !createIncludeList(ctx, root) {
		return nil
	}

	var fileList ir.FileList
	var loadedFiles []*dgFile
	loadedFiles = append(loadedFiles, root)

	for fileID := 0; fileID < len(loadedFiles); fileID++ {
		file := loadedFiles[fileID]
		for _, child := range file.children {
			child.parsedFile = loadFile(ctx, child.path)
			if child.parsedFile == nil {
				continue
			}
			child.parsedFile.ParentIndex1 = fileID
			child.parsedFile.ParentIndex2 = child.parentModuleIndex
			if !createIncludeList(ctx, child) {
				continue
			}
			loadedFiles = append(loadedFiles, child)
		}
		fileList = append(fileList, file.parsedFile)
	}

	return fileList
}

func loadFile(ctx *common.BuildContext, path token.Position) *ir.File {
	cachedFile := ctx.LookupFile(path.Filename)
	if cachedFile == nil {
		buf, err := ioutil.ReadFile(path.Filename)
		if err != nil {
			ctx.Errors.AddGeneric2(path, err)
			return nil
		}
		cachedFile = ctx.NewFile(path.Filename, buf)
	}
	parsedFile, err := parseFile(path.Filename, cachedFile.Src)
	if err != nil {
		ctx.Errors.AddGeneric2(path, err)
		return nil
	}
	return parsedFile
}

func dgFileFromPath(src token.Position, cwd string, dir string, filename string) (*dgFile, error) {
	path := filename
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
		if !strings.HasPrefix(path, ".") {
			path = "./" + path
		}
	}
	path = filepath.Clean(path)
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
		srcPos:  src,
		path:    token.NewPosition1(path),
		absPath: token.NewPosition1(token.Abs(cwd, path)),
	}
	return file, nil
}

func createIncludeList(ctx *common.BuildContext, parent *dgFile) bool {
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
					ctx.Errors.AddContext(includeLit.Pos(), formatIncludeTrace(trace), "invalid path")
				} else {
					ctx.Errors.Add(includeLit.Pos(), "invalid path")
				}
				ok = false
				break
			}

			child, err := dgFileFromPath(includeLit.Pos(), ctx.Cwd, parentDir, unquoted)
			if err != nil {
				ctx.Errors.AddGeneric2(includeLit.Pos(), err)
				ok = false
				break
			}

			if trace, ok := checkIncludeCycle(parent, child); ok {
				ctx.Errors.AddContext(includeLit.Pos(), formatIncludeTrace(trace), "cycle detected")
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
		if file.absPath.Filename == child.absPath.Filename {
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
