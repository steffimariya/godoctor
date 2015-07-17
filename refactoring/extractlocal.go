// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file will extract the local variables from
// statements and put them into a variable to replae
// them in the statements.

package refactoring

import (
	"go/ast"
	"regexp"
	// "fmt"
	"github.com/godoctor/godoctor/text"
	"github.com/godoctor/godoctor/internal/golang.org/x/tools/astutil"
)

type ExtractLocal struct {
	RefactoringBase
	varName string
}

func (r *ExtractLocal) Description() *Description {
	return &Description{
		Name:      "Extract Local Variable Refactoring",
		Synopsis:  "Extract a selection to a new variable",
		Usage:     "<new_name>",
		Multifile: false,
		Params: []Parameter{
			// args[0] which is the string that will replace the selected text
			Parameter{
				Label:        "newVar name: ",
				Prompt:       "Please select name for the new Variable.",
				DefaultValue: "",
			}},
		Hidden: false,
	}
}

// this run function will run the program
func (r *ExtractLocal) Run(config *Config) *Result {
	r.RefactoringBase.Run(config)
	r.Log.ChangeInitialErrorsToWarnings()
	if r.Log.ContainsErrors() {
		return &r.Result
	}
	// get first variable
	// check if there was a selected node from refactoring
	if r.SelectedNode == nil {
		r.Log.Error("Please select an expression to extract.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return &r.Result
	}
	r.sChoice(config)
	r.selectionCheck()
	// get third variable that will be the
	// new variable to replace the expression
	r.varName = config.Args[0].(string)
	r.singleExtract(r.SelectedNode)
	r.FormatFileInEditor()
	r.UpdateLog(config, false)
	return &r.Result
}

// sChoice gets input for second variable
func (r *ExtractLocal) sChoice(config *Config) *Result {
	r.varName = config.Args[0].(string)
	if r.varName == "" {
		r.Log.Error("You must enter a name for the new variable.")
		return &r.Result
	}
	return &r.Result
}

// selectionCheck checks if the selection is empty or not
func (r *ExtractLocal) selectionCheck() *Result {
	if r.SelectedNode == nil {
		r.Log.Error("Please select an identifier to extract.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return &r.Result
	}
	return &r.Result
}


// singleExtract locates the position of the selection and will
// replace it with a new variable of the user's chosing
func (r *ExtractLocal) singleExtract(sel ast.Node) {
	var switchList, typeSwitchFound []ast.Stmt
	var lhs []ast.Expr
	var switchNode, ifInitAssign, condition, assign ast.Node
	found:= false
	ast.Inspect(r.File, func(n ast.Node) bool {
		switch selectedNode := n.(type) {
		case ast.Stmt:
			switch node := selectedNode.(type){
			case *ast.SwitchStmt:
				switchNode = node
				switchList = node.Body.List
			case *ast.IfStmt:
				if assignNode, ok := node.Init.(*ast.AssignStmt); ok { // check if the if statement has a assignment statement in its init list
					ifInitAssign = assignNode
				}
			case *ast.ForStmt:
				if r.forCheck() {
					found = true
					r.Log.Error("You can't extract from a for loop's conditions (any part in the for statement).")
				}
			case *ast.TypeSwitchStmt:
				if len(node.Body.List) != 0 {
					typeSwitchFound = node.Body.List
				}
				if node.Init != nil {
					condition = node.Init
				} else if node.Assign != nil {
					assign = node.Assign
				}
			case *ast.AssignStmt:
				lhs = node.Lhs
			case *ast.BlockStmt:
				if _, ok := r.SelectedNode.(*ast.BlockStmt);ok{
					r.Log.Error("You can't select a block for extract local. Please use the extract refactoring when selecting blocks or functions.")
					found = true
				}
			case *ast.BranchStmt:
				if _ , ok := r.SelectedNode.(*ast.BranchStmt); ok {
					r.Log.Error("Sorry, you can't extract a goto, break, continue, or fallthrough statement")
					found = true
				}
			case *ast.LabeledStmt:
				if _, ok := r.SelectedNode.(*ast.LabeledStmt); ok {
					r.Log.Error("You can't create a variable for a Label.")
					found = true
				}
			}
		case ast.Expr:
			if r.multiAssignVarCheck(lhs) { // check if selected node is from part a _, _ := _, _ stmt
				r.Log.Error("You can't extract from a multi-assign statement (ie: a, b := 0, 0).")
				found = true
			} else if r.isPreDeclaredIdent() { // check if the word extracted is a reserved word
				r.Log.Error("Sorry, you can't pull out a predetermined identifier like string or reflect "+
				"and make a variable of that type (reflect.String can't be made newVar.String since it isn't type reflect)")
				found = true
			} else if r.varStmtCheck() { // Checks if the extracted statement is a variable declaration i.e, var i int
				r.Log.Error("Extracting from a var stmt will alter the definition and should be avoided.")
				found = true
			} else if r.lhsAssignVarCheck() { // checks if its from the lhs of assign stmt
				r.Log.Error("You can't extract a variable from the lhs of an assignment statement.")				
				found = true
			} else if r.callCheck(selectedNode) { // checks if it's in call expr
				r.Log.Error("You can't extract this part of a 'call expr' (ie:  fmt.Println('____') can't extract the fmt or Println, or fmt.Println).")
				found = true
			} else if r.funcInputCheck() { // check if selected node is from the function definition
				r.Log.Error("You can't extract from the function parameters/results/method input at the function definition or the whole "+
				"function itself (for full function extraction use the extract refactoring).")
				found = true
			} else if r.selectorCheck(selectedNode) { // check for when a selector type
				r.Log.Error("You can't extract the type from a selector expr (ie: case reflect.Float32:  can't extract Float32).")
				found = true
			} else if r.caseSelectorCheck(switchList) { // checks if its from the switch type (ie switch node, checks if its node)
				r.Log.Error("Sorry, you can't extract the switch key or the case selector.")
				found = true
			} else if r.checkNil(selectedNode) { // checks if it's nil
				r.Log.Error("You can't extract nil since nil isn't a type.")				
				found = true
			} else if r.keyValueCheck() { // check to see if from a key:value
				r.Log.Error("You can't extract the whole key/value from a key value expression (ie: key: value can't be newVar := key: value).")
				found = true
			} else if r.checkComposite() {
				r.Log.Error("You can't select a block for extract local. Please use the extract refactoring when selecting blocks or functions.")
				found = true
			} else if r.checkAssignIdents() {
				newVar := r.createVar(r.SelectedNode)
				found = r.createParent(newVar)
			} else if selectedNode == r.SelectedNode {
				if r.switchCheck(switchList, selectedNode) { // check for switch stmts
					newVar := r.createVar(selectedNode)
					found = r.addForSwitch(switchNode, selectedNode, newVar)
				} else if r.typeSwitchCheck(typeSwitchFound, condition, assign){ // check if selected node is from part of the type switch
					r.Log.Error("You can't extract a type variable from a type switch statement or it's case statements.")
					found = true
				} else if r.ifMultLeftCheck(ifInitAssign, selectedNode) { // check for for loops or if mutli assign
					found = true
					r.Log.Error("You can't extract from an if statement with an assign stmt in it")
				} else {
					found = r.createParent(r.createVar(selectedNode))
				}
			}
		default:
		}
		// used to end the inspect func once an extraction is done
		if found == false {
			return true
		}
		return false

	})
}


// multiAssignVarCheck checks if it's a multi assign stmt ie  _, _ := _, _
// and returns true if the selected node is on either side
func (r *ExtractLocal) multiAssignVarCheck(lhs []ast.Expr) bool {
	// if len(lhs) > 1 && r.posLine(lhs[0]) == r.posLine(r.SelectedNode) {
	if (len(lhs)>1) && r.posLine(lhs[0]) == r.posLine(r.SelectedNode){
		r.Log.Error("You can't extract from a multi-assign statement (ie: a, b := 0, 0).")
	}
	return false
}


func (r *ExtractLocal) posOff(node ast.Node) int {
	return r.Program.Fset.Position(node.Pos()).Offset
}
func (r *ExtractLocal) endOff(node ast.Node) int {
	return r.Program.Fset.Position(node.End()).Offset
}
func (r *ExtractLocal) endLine(node ast.Node) int {
	return r.Program.Fset.Position(node.End()).Line
}
func (r *ExtractLocal) posLine(node ast.Node) int {
	return r.Program.Fset.Position(node.Pos()).Line
}

// switchCheck switch case check for switch version of extraction
func (r *ExtractLocal) switchCheck(switchList []ast.Stmt, selectedNode ast.Node) bool {
	for index, _ := range switchList {
		line1 := r.Program.Fset.Position(switchList[index].Pos()).Line
		line2 := r.Program.Fset.Position(selectedNode.Pos()).Line
		if line1 == line2 {
			return true
		}
	}



	return false
}

// forCheck for loop init/cond/post test
func (r *ExtractLocal) forCheck() bool { // check if the immediate parent is a for loop, if so, then return true - fails the for loop extraction
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, index2 := range enclosing {
		if forparent, ok := index2.(*ast.ForStmt); ok {
			if r.posLine(forparent) == r.posLine(r.SelectedNode) {
				return true
			}
		}
	}
	return false
}

// ifMultLeftCheck if stmt check for _, _ conditions on the left
func (r *ExtractLocal) ifMultLeftCheck(ifInitAssign ast.Node, selectedNode ast.Node) bool {
		if ifInitAssign != nil {
		if r.Program.Fset.Position(ifInitAssign.Pos()).Line == r.Program.Fset.Position(r.SelectedNode.Pos()).Line {
			return true
		}
	}
	return false
}

// typeSwitchCheck sees if the switch is a type switch and if so returns true to produce an error
func (r *ExtractLocal) typeSwitchCheck(typeSwitchFound []ast.Stmt, condition ast.Node, assign ast.Node) bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	if len(typeSwitchFound) != 0 {
		for _, index := range typeSwitchFound {
			if caseClauses, ok := index.(*ast.CaseClause); ok {
				for _, index2 := range caseClauses.List {
					for _, index3 := range enclosing {
						if index3 == index2 {
						r.Log.Error("You can't extract a type variable from a type switch statement or it's case statements.")
						}
					}
				}
			}
		}
	}
	if condition == r.SelectedNode {
		return true
	} else if assign != nil {
		if aStmt, ok := assign.(*ast.AssignStmt); ok {
			if aStmt.Lhs != nil && aStmt.Lhs[0] == r.SelectedNode {
				return true
			} else if aStmt.Rhs != nil && aStmt.Rhs[0] == r.SelectedNode {
				return true
			}
		}
	}
	return false
}

// funcInputCheck checks to see if selected node is a function parameter or result
// in the definition and returns true if it is so as to produce an error
func (r *ExtractLocal) funcInputCheck() bool {
	path, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	if _,ok := r.SelectedNode.(*ast.FieldList);ok{
		return true
	}else if _,ok := path[1].(*ast.Field);ok{
		return true
	}else if _,ok := path[0].(*ast.Field);ok{
		return true
	}
	return false
}


// keyValueCheck checks if is a key value expr or a child of
// a key value expr, so anything to do maps, or keys : values
func (r *ExtractLocal) keyValueCheck() bool {
	if _, ok := r.SelectedNode.(*ast.KeyValueExpr); ok {
		return true
	}
	return false
}

// varStmtCheck check if it's a declartion stmt, and return true
// if it is ie var apple int or var handler http.Handler
func (r *ExtractLocal) varStmtCheck() bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, index := range enclosing {
		if _, ok := index.(*ast.DeclStmt); ok {
			return true
		}
	}
	return false
}

// lhsAssignVarCheck checks if r.SelectedNode is on the lhs of an
// assign statement
func (r *ExtractLocal) lhsAssignVarCheck() bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, index := range enclosing {
		if assign, ok := index.(*ast.AssignStmt); ok {
			for _, index2 := range assign.Lhs {
				// if _, ok := index2.(*ast.SelectorExpr); ok {
				// 	return true
				// }
				if index2 == r.SelectedNode {
					return true
				}
			}
		}
	}
	return false
}

// callCheck check for call expr and don't allow if its just
// a call expr itself, or if part of the callexpr (although
// inside the () of it should work)
func (r *ExtractLocal) callCheck(selectedNode ast.Node) bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, index := range enclosing {
		if call, ok := index.(*ast.CallExpr); ok {
			for _, index2 := range call.Args {
				if index2 == r.SelectedNode { // as in fmt.Println(a+b); a+b can be extracted into a new variable
					return false
				} else if _, ok := index2.(*ast.BinaryExpr); ok { // as in fmt.Println(a+b+c); a+b+c can be extracted into a new variable
					return false
				}
			}
			return true
		}
		// if _, ok := index.(*ast.CallExpr); ok {
		// 	return true
		// }
	}
	return false
}

// selectorCheck this will check the selector expr and see if the
// sel part (which is the type) is the r.SelectedNode
func (r *ExtractLocal) selectorCheck(selectedNode ast.Node) bool {
	if  selector,ok := selectedNode.(*ast.SelectorExpr);ok{
		if selector.Sel == r.SelectedNode {
			return true
		}
	}
	return false
}

// checkNil checks if nil is trying to be extracted
func (r *ExtractLocal) checkNil(selectedNode ast.Node) bool {

	if name, ok := selectedNode.(*ast.Ident); ok {
		if name == r.SelectedNode {
			if name.Name == "nil" {
				return true
			}
		}
	}
	return false
}

// checkComposite checks if r.SelectedNode is a composite lit
// which should be handled by extract, not extract local
func (r *ExtractLocal) checkComposite() bool {
	if _, ok := r.SelectedNode.(*ast.CompositeLit); ok {
		return true
	}
	return false
}

// checkAssignIdents checks to see if an ident obj is inside an assign, which would be
// allowed to be extracted
func (r *ExtractLocal) checkAssignIdents() bool {
if ident, ok := r.SelectedNode.(*ast.Ident); ok {
		enclosing, _ := astutil.PathEnclosingInterval(r.File, ident.Pos(), ident.End())
		for _, index := range enclosing {
			if assign, ok := index.(*ast.AssignStmt); ok {
				for _, index := range assign.Rhs {
					if selector, ok := index.(*ast.SelectorExpr); ok {
						if selector.Sel == ident || selector.X == ident {
							return false
						}
					}
				}
				return true
			}
		}
	}

	return false
}

// caseSelectorCheck check case clauses to see if the case part is trying to be extracted
func (r *ExtractLocal) caseSelectorCheck(switchList []ast.Stmt) bool {
	for _, index := range switchList {
		if cases, ok := index.(*ast.CaseClause); ok {
			if cases == r.SelectedNode {
				return true
			}
		}
	}
	return false
}

// function needed to detect if a reserved word is trying to be extracted (a predeclared Ident)
func (r *ExtractLocal) isPreDeclaredIdent() bool {
	result, _ := regexp.MatchString(
		"^(bool|byte|complex64|complex128|error|float32|float64|int|int8|int16|int32|int64|rune|string|uint|uint8|uint16|uint32|uint64|uintptr|global|reflect)$", 
							string(r.FileContents[int(r.Program.Fset.Position(r.SelectedNode.Pos()).Offset):int(r.Program.Fset.Position(r.SelectedNode.End()).Offset)]))
	return result
}

// addForSwitch switch extract function
func (r *ExtractLocal) addForSwitch(switchNode ast.Node, selectedNode ast.Node, newVar string) bool {
	r.Edits[r.Filename].Add(&text.Extent{r.Program.Fset.Position(switchNode.Pos()).Offset, 0}, newVar)
	r.Edits[r.Filename].Add(&text.Extent{r.Program.Fset.Position(selectedNode.Pos()).Offset, 
											r.Program.Fset.Position(selectedNode.End()).Offset - r.Program.Fset.Position(selectedNode.Pos()).Offset}, r.varName)
	return true
}

// createVar create the new var to go into the coding
func (r *ExtractLocal) createVar(selectedNode ast.Node) string {
	return  r.varName + " := " + string(r.FileContents[int(r.Program.Fset.Position(selectedNode.Pos()).Offset):
															int(r.Program.Fset.Position(selectedNode.End()).Offset)]) + "\n"
}

// createParent finds the parent of the selected node, and gives it to the
// function that inputs the newVar into the file at the parent spot and
// at the selected node spot
func (r *ExtractLocal) createParent(newVar string) bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	found := false
	var indexOfParent ast.Node
	if len(enclosing) != 0 {
		for index, _ := range enclosing {
			if enclosing[index] != nil {
				if _, ok := enclosing[index].(*ast.LabeledStmt); ok {
					indexOfParent = enclosing[index-1]
					break
				} else if _, ok := enclosing[index].(*ast.CaseClause); ok {
					indexOfParent = enclosing[index-1]
					break
				} else if _, ok := enclosing[index].(*ast.BlockStmt); ok {
					if enclosing[index-1] != nil {
						indexOfParent = enclosing[index-1]
						break
					}
				}
			}
		}
	}
	if indexOfParent != nil {
		found = r.addBeforeParent(indexOfParent, newVar)
	}
	return found
}

// addBeforeParent extract that puts the new var above the parent
func (r *ExtractLocal) addBeforeParent(parentNode ast.Node, newVar string) bool {
	r.Edits[r.Filename].Add(&text.Extent{r.Program.Fset.Position(parentNode.Pos()).Offset, 0}, newVar)
	r.Edits[r.Filename].Add(&text.Extent{r.Program.Fset.Position(r.SelectedNode.Pos()).Offset,
										 r.Program.Fset.Position(r.SelectedNode.End()).Offset - r.Program.Fset.Position(r.SelectedNode.Pos()).Offset}, r.varName)
	return true
}
