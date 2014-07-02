// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package engine_test

import (
	"testing"

	"golang-refactoring.org/go-doctor/engine"
	"golang-refactoring.org/go-doctor/filesystem"
	"golang-refactoring.org/go-doctor/refactoring"
	"golang-refactoring.org/go-doctor/text"
)

type customRefactoring struct{}

func (*customRefactoring) Description() *refactoring.Description {
	return &refactoring.Description{
		Name:    "Test",
		Params:  nil,
		Quality: refactoring.Development,
	}
}

func (*customRefactoring) Run(config *refactoring.Config) *refactoring.Result {
	return &refactoring.Result{
		Log:       refactoring.NewLog(),
		Edits:     map[string]*text.EditSet{},
		FSChanges: []filesystem.Change{},
	}
}

func TestEngine(t *testing.T) {
	first := ""
	for shortName, refac := range engine.AllRefactorings() {
		if first == "" {
			first = shortName
		}
		if engine.GetRefactoring(shortName) != refac {
			t.Fatalf("GetRefactoring return incorrect")
		}
	}

	err := engine.AddRefactoring(first, &customRefactoring{})
	if err == nil {
		t.Fatalf("Should have forbidden adding with existing name")
	}

	err = engine.AddRefactoring("zz_new", &customRefactoring{})
	if err != nil {
		t.Fatalf("The name zz_new should be unique and OK to add (?!)")
	}
}