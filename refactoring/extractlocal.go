// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file will extract the local variables from
// statements and put them into a variable to replae
// them in the statements.

package refactoring

import (
	"go/ast"
	"regexp"
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
	r.validateVarName(config)
	r.varName = config.Args[0].(string)
	r.checkSelection()
	r.FormatFileInEditor()
	r.UpdateLog(config, false)
	return &r.Result
}

// sChoice gets input for second variable
func (r *ExtractLocal) validateVarName(config *Config) *Result {
	if config.Args[0].(string) == "" {
		r.Log.Error("You must enter a name for the new variable.")
		return &r.Result
	}
	return &r.Result
}

func (r *ExtractLocal) checkSelection(){
switch sel := r.SelectedNode.(type){
case ast.Stmt:
	r.handleStmt(sel)
case ast.Expr:
	r.handleExpr(sel)
default: // *ast.Field and *ast.Fieldlist
	r.Log.Error("You can't extract from the function parameters/results/method input at the function definition or the whole "+
				"function itself (for full function extraction use the extract refactoring).")
}
}

func (r *ExtractLocal) handleExpr(sel ast.Node){
	switch sel.(type){
	case *ast.Ident:
		r.checkForFields()
		r.isPreDeclaredIdent()
		r.checkNil()
		r.varStmtCheck()
		r.lhsAssignVarCheck()
		r.checkIdentParent()
		r.checkAssignIdents()
		r.commonCheck()
	case *ast.BinaryExpr:
		r.ifMultLeftCheck()
		r.commonCheck()
	case *ast.SelectorExpr:
		r.lhsAssignVarCheck()
		if sel.(*ast.SelectorExpr).Sel == r.SelectedNode{
			r.Log.Error("You can't extract the type from a selector expr (ie: case reflect.Float32:  can't extract Float32).")
		}
		r.commonCheck()
	case *ast.UnaryExpr:
		r.commonCheck()
	case *ast.IndexExpr:
		r.commonCheck()
	case *ast.BasicLit:
		r.commonCheck()
		r.ifMultLeftCheck()
	case *ast.ParenExpr:
		r.Log.Error("You can't extract this part of a 'call expr' (ie:  fmt.Println('____') can't extract the fmt or Println, or fmt.Println).")
	case *ast.CallExpr:
		r.Log.Error("You can't extract this part of a 'call expr' (ie:  fmt.Println('____') can't extract the fmt or Println, or fmt.Println).")
	case *ast.KeyValueExpr:
		r.Log.Error("You can't extract the whole key/value from a key value expression (ie: key: value can't be newVar := key: value).")
	case *ast.CompositeLit:
		r.Log.Error("You can't select a block for extract local. Please use the extract refactoring when selecting blocks or functions.")
	case *ast.StarExpr:
		if r.checkStarExpr(){
			r.Log.Error("Cannot extract Star Expression statements!")
		}else{
			r.createParent(r.createVar(r.SelectedNode))
		}	
	case *ast.TypeAssertExpr:
		r.Log.Error("Expression followed by a type assertion cannot be extracted")
	}
}

func (r *ExtractLocal) commonCheck(){
	r.forCheck()
	r.callCheck()
	if r.switchCheckCall(){
		return
	}
	r.createParent(r.createVar(r.SelectedNode))
}
// forCheck for loop init/cond/post test
func (r *ExtractLocal) forCheck(){ // check if the immediate parent is a for loop, if so, then return true - fails the for loop extraction
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, index2 := range enclosing {
		if forparent, ok := index2.(*ast.ForStmt); ok {
				if r.Program.Fset.Position(forparent.Pos()).Line == r.Program.Fset.Position(r.SelectedNode.Pos()).Line { 
				r.Log.Error("You can't extract from a for loop's conditions (any part in the for statement).")

			}
		}
	}
}


func (r *ExtractLocal) checkStarExpr() bool {
	enclosing ,_:=astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
		for _,a:= range enclosing{
			if _,ok := a.(*ast.AssignStmt);ok{
				return false
			}
		}
		return true
}

// ifMultLeftCheck if stmt check for _, _ conditions on the left
func (r *ExtractLocal) ifMultLeftCheck() {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, a:= range enclosing{
		if _,ok:=a.(*ast.IfStmt);ok{
			if assignNode, ok := a.(*ast.IfStmt).Init.(*ast.AssignStmt); ok {
				if r.Program.Fset.Position(assignNode.Pos()).Line == r.Program.Fset.Position(r.SelectedNode.Pos()).Line {
					r.Log.Error("You can't extract from an if statement with an assign stmt in it")
				}
			}
			
		}
	}
}

func (r *ExtractLocal) typeSwitchCheckCall(){
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _,a:= range enclosing{
		if _,ok := a.(*ast.TypeSwitchStmt);ok{
			if r.typeSwitchCheck(a.(*ast.TypeSwitchStmt).Body.List,a.(*ast.TypeSwitchStmt).Init ,a.(*ast.TypeSwitchStmt).Assign ){
				r.Log.Error("You can't extract a type variable from a type switch statement or it's case statements.")
			}
		}
	}
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

// switchCheck switch case check for switch version of extraction
func (r *ExtractLocal) switchCheckCall()  bool {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _,a:= range enclosing{
		if _,ok := a.(*ast.SwitchStmt);ok{
			if r.switchCheck(a.(*ast.SwitchStmt).Body.List){
				newVar := r.createVar(r.SelectedNode)
				r.addForSwitch(a, newVar)
				return true
			}
		}

	}
	return false
}

// switchCheck switch case check for switch version of extraction
func (r *ExtractLocal) switchCheck(switchList []ast.Stmt) bool {
	for index, _ := range switchList {
		line1 := r.Program.Fset.Position(switchList[index].Pos()).Line
		line2 := r.Program.Fset.Position(r.SelectedNode.Pos()).Line
		if line1 == line2 {
			return true
		}
	}
	return false
}


// checkAssignIdents checks to see if an ident obj is inside an assign, which would be
// allowed to be extracted
func (r *ExtractLocal) checkAssignIdents(){
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, index := range enclosing {
		if assign, ok := index.(*ast.AssignStmt); ok {
			for _, index := range assign.Rhs {
				if selector, ok := index.(*ast.SelectorExpr); ok {
					if selector.Sel == r.SelectedNode || selector.X == r.SelectedNode{
						r.Log.Error("In a Selector Expression, the selector.Sel part or the selector.X part cannot be extracted")
					}
				}
			}
		}
	}
}


// if the identifier's immedeate parent is a *ast.SelectorExpr then throw error
func (r *ExtractLocal) checkIdentParent() {
	path, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	if _,ok:=path[1].(*ast.SelectorExpr);ok{
		if path[1].(*ast.SelectorExpr).X == r.SelectedNode{
			return
		}else if path[1].(*ast.SelectorExpr).Sel == r.SelectedNode{
			r.Log.Error("You can't extract the type from a selector expr (ie: case reflect.Float32:  can't extract Float32).")
		}
		
	}
}

// callCheck check for call expr and don't allow if its just a call expr itself, or if part of the callexpr (although
// inside the () of it should work)
func (r *ExtractLocal) callCheck() {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, index := range enclosing {
		if call, ok := index.(*ast.CallExpr); ok {
			for _, index2 := range call.Args {
				if index2 == r.SelectedNode { // as in fmt.Println(a+b); a+b can be extracted into a new variable
					return
				} else if _, ok := index2.(*ast.BinaryExpr); ok { // as in fmt.Println(a+b+c); a+b+c can be extracted into a new variable
					return
				}
			}
			r.Log.Error("You can't extract this part of a 'call expr' (ie:  fmt.Println('____') can't extract the fmt or Println, or fmt.Println).")
		}
	}
}

// lhsAssignVarCheck checks if r.SelectedNode is on the lhs of an
// assign statement
func (r *ExtractLocal) lhsAssignVarCheck(){
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, index := range enclosing {
		if assign, ok := index.(*ast.AssignStmt); ok {
			for _, index2 := range assign.Lhs {
				if index2 == r.SelectedNode {
					r.Log.Error("You can't extract a variable from the lhs of an assignment statement.")				
				}
			}
		}
	}
}

// varStmtCheck check if it's a declartion stmt, and return true
// if it is ie var apple int or var handler http.Handler
func (r *ExtractLocal) varStmtCheck() {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, index := range enclosing {
		if _, ok := index.(*ast.DeclStmt); ok {
			r.Log.Error("Extracting from a var stmt will alter the definition and should be avoided.")
		}
	}
}

// checkNil checks if nil is trying to be extracted
func (r *ExtractLocal) checkNil() {
	if r.SelectedNode.(*ast.Ident).Name == "nil"{
		r.Log.Error("You can't extract nil since nil isn't a type.")	
	}
}

// function needed to detect if a reserved word is trying to be extracted (a predeclared Ident)
func (r *ExtractLocal) isPreDeclaredIdent() {
	result, _ := regexp.MatchString(
		"^(bool|byte|complex64|complex128|error|float32|float64|int|int8|int16|int32|int64|rune|string|uint|uint8|uint16|uint32|uint64|uintptr|global|reflect)$", 
							string(r.FileContents[int(r.Program.Fset.Position(r.SelectedNode.Pos()).Offset):int(r.Program.Fset.Position(r.SelectedNode.End()).Offset)]))
	if result{
		r.Log.Error("Sorry, you can't pull out a predetermined identifier like string or reflect "+
				"and make a variable of that type (reflect.String can't be made newVar.String since it isn't type reflect)")
	}
}

func (r *ExtractLocal) checkForFields(){
	path ,_ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
	for _, node := range path{
		if _, ok := node.(*ast.Field); ok {
			r.Log.Error("You can't extract from the function parameters/results/method input at the function definition or the whole "+
				"function itself (for full function extraction use the extract refactoring).")
		}	
	}
}

func (r *ExtractLocal) handleStmt(sel ast.Node){
	switch sel.(type){
	case *ast.BranchStmt:
		r.Log.Error("Sorry, you can't extract a goto, break, continue, or fallthrough statement")
	case *ast.LabeledStmt:
		r.Log.Error("You can't create a variable for a Label.")
	case *ast.CaseClause:
		r.Log.Error("Sorry, you can't extract the switch key or the case selector.")
	case *ast.IncDecStmt:
		r.Log.Error("Sorry, you cannot extract an increment/ decrement statement.")
	case *ast.BlockStmt:
		r.Log.Error("You can't select a block for extract local. Please use the extract refactoring when selecting blocks or functions.")
	case *ast.RangeStmt:
		r.Log.Error("You cannot extract a part from a for-range statement")
	case *ast.IfStmt:
		r.Log.Error("You cannot extract a part of 'if' statement")
	case *ast.AssignStmt:
		r.Log.Error("You cannot extract an Assignment statement")
	default :
		r.Log.Error("You cannot extract any statement")
	}
}

// addForSwitch switch extract function
func (r *ExtractLocal) addForSwitch(switchNode ast.Node, newVar string) bool {
	r.Edits[r.Filename].Add(&text.Extent{r.Program.Fset.Position(switchNode.Pos()).Offset, 0}, newVar)
	r.Edits[r.Filename].Add(&text.Extent{r.Program.Fset.Position(r.SelectedNode.Pos()).Offset, 
											r.Program.Fset.Position(r.SelectedNode.End()).Offset - r.Program.Fset.Position(r.SelectedNode.Pos()).Offset }, r.varName)
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
func (r *ExtractLocal) createParent(newVar string) {
	enclosing, _ := astutil.PathEnclosingInterval(r.File, r.SelectedNode.Pos(), r.SelectedNode.End())
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
		r.addBeforeParent(indexOfParent, newVar)
	}
}

// addBeforeParent extract that puts the new var above the parent
func (r *ExtractLocal) addBeforeParent(parentNode ast.Node, newVar string){
	r.Edits[r.Filename].Add(&text.Extent{r.Program.Fset.Position(parentNode.Pos()).Offset, 0}, newVar)
	r.Edits[r.Filename].Add(&text.Extent{r.Program.Fset.Position(r.SelectedNode.Pos()).Offset,
										 r.Program.Fset.Position(r.SelectedNode.End()).Offset - r.Program.Fset.Position(r.SelectedNode.Pos()).Offset}, r.varName)
}