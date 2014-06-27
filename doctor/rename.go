// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doctor

// This file defines a refactoring to rename variables, functions, methods, structs,interfaces and packages
// (TODO: It cannot yet rename packages.)

import (
	"go/ast"
	"regexp"
	//"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"strings"

	"code.google.com/p/go.tools/go/types"
)

// A renameRefactoring is used to rename identifiers in Go programs.
type renameRefactoring struct {
	refactoringBase
	newName   string
	signature *types.Signature
}

func (r *renameRefactoring) Description() *Description {
	return &Description{
		Name: "Rename",
		Params: []Parameter{Parameter{
			Label:        "New Name:",
			Prompt:       "What to rename this identifier to.",
			DefaultValue: "",
		}},
		Quality: Development,
	}
}

func (r *renameRefactoring) Run(config *Config) *Result {
	if r.refactoringBase.Run(config); r.Log.ContainsErrors() {
		return &r.Result
	}

	if !validateArgs(config, r.Description(), r.Log) {
		return &r.Result
	}

	r.newName = config.Args[0].(string)
	if !r.isIdentifierValid(r.newName) {
		r.Log.Log(FATAL_ERROR, "The new name "+r.newName+" is not a valid Go identifier")
		return &r.Result
	}

	if r.selectedNode == nil {
		r.Log.Log(FATAL_ERROR, "Please select an identifier to rename.")
		return &r.Result
	}

	if r.newName == "" {
		r.Log.Log(FATAL_ERROR, "newName cannot be empty")
		return &r.Result
	}

	switch ident := r.selectedNode.(type) {
	case *ast.Ident:
		if ast.IsExported(ident.Name) && !ast.IsExported(r.newName) {
			r.Log.Log(FATAL_ERROR, "newName cannot be non Exportable if selected identifier name is Exportable")
			return &r.Result
		}
		r.rename(ident)

	default:
		r.Log.Log(FATAL_ERROR, "Please select an identifier to rename.")
	}
	return &r.Result
}

func (r *renameRefactoring) isIdentifierValid(newName string) bool {
	matched, err := regexp.MatchString("^[A-Za-z_][0-9A-Za-z_]*$", newName)
	if matched && err == nil {
		keyword, err := regexp.MatchString("^(break|case|chan|const|continue|default|defer|else|fallthrough|for|func|go|goto|if|import|interface|map|package|range|return|select|struct|switch|type|var)$", newName)
		return !keyword && err == nil
	}
	return false
}

func (r *renameRefactoring) rename(ident *ast.Ident) {
	if !r.IdentifierExists(ident) {
		search := &SearchEngine{r.program}
		searchResult, err := search.FindOccurrences(ident)
		if err != nil {
			r.Log.Log(FATAL_ERROR, err.Error())
			return
		}

		r.addOccurrences(searchResult)
		if search.isPackageName(ident) {
			r.addFileSystemChanges(searchResult, ident)
		}
		//TODO: r.checkForErrors()
		return
	}

}

//IdentifierExists checks if there already exists an Identifier with the newName,with in the scope of the oldname.
func (r *renameRefactoring) IdentifierExists(ident *ast.Ident) bool {

	obj := r.pkgInfo(r.fileContaining(ident)).ObjectOf(ident)
	search := &SearchEngine{r.program}

	if obj == nil && !search.isPackageName(ident) {

		r.Log.Log(FATAL_ERROR, "unable to find declaration of selected identifier")
		return true
	}

	if search.isPackageName(ident) {
		return false
	}
	identscope := obj.Parent()

	if isMethod(obj) {
		objfound, _, pointerindirections := types.LookupFieldOrMethod(methodReceiver(obj).Type(), obj.Pkg(), r.newName)
		if isMethod(objfound) && pointerindirections {
			r.Log.Log(FATAL_ERROR, "newname already exists in scope,please select other value for the newname")
			return true
		} else {
			return false
		}
	}

	if identscope.LookupParent(r.newName) != nil {

		r.Log.Log(FATAL_ERROR, "newname already exists in scope,please select other value for the newname")
		return true
	}

	return false
}

//addOccurrences adds all the Occurences to the editset
func (r *renameRefactoring) addOccurrences(allOccurrences map[string][]OffsetLength) {
	for filename, occurrences := range allOccurrences {
		for _, occurrence := range occurrences {
			if r.Edits[filename] == nil {
				r.Edits[filename] = NewEditSet()
			}
			r.Edits[filename].Add(occurrence, r.newName)

		}
	}
}

func (r *SearchEngine) isPackageName(ident *ast.Ident) bool {

	if r.pkgInfo(r.fileContaining(ident)).Pkg.Name() == ident.Name {
		return true
	}

	return false
}

func (r *renameRefactoring) addFileSystemChanges(allOccurrences map[string][]OffsetLength, ident *ast.Ident) {
	for filename, _ := range allOccurrences {

		if filepath.Base(filepath.Dir(filename)) == ident.Name && allFilesinDirectoryhaveSamePkg(filepath.Dir(filename), ident) {
			chg := &FSRename{filepath.Dir(filename), r.newName}
			r.FSChanges = append(r.FSChanges,
				chg)

		}
	}
}

func allFilesinDirectoryhaveSamePkg(directorypath string, ident *ast.Ident) bool {

	var renamefile bool = false
	fileInfos, _ := ioutil.ReadDir(directorypath)

	for _, file := range fileInfos {
		if strings.HasSuffix(file.Name(), ".go") {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, filepath.Join(directorypath, file.Name()), nil, 0)
			if err != nil {
				panic(err)
			}
			if f.Name.Name == ident.Name {
				renamefile = true
			}
		}
	}

	return renamefile
}
