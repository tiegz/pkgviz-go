package pkgviz_test

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/tiegz/pkgviz-go/pkg/pkgviz"
)

func TestPlaceholder(t *testing.T) {
}

// TODO finish this one the package is public. Local dev is too tricky.
// Also, type-checker output may be non-deterministic?
// func TestWriteGraphWithBasicTypes(t *testing.T) {
// 	assertGraph(
// 		t,
// 		"../fake_pkg",
// 		"../../pkg/fake_pkg/fake_pkg.dot",
// 	)
// }

func assertGraph(t *testing.T, pkgPath, pkgExpectationPath string) {
	actual := pkgviz.WriteGraph(pkgPath)
	expected := getFixtureFile(pkgExpectationPath)

	if strings.TrimSpace(actual) != strings.TrimSpace(expected) {
		t.Errorf("Expected %s, got %s instead.", expected, actual)
	}
}

func getFixtureFile(filepath string) string {
	dat, err := ioutil.ReadFile(filepath)
	if err != nil {
		panic(err)
	}
	return string(dat)
}
