package pkgviz_test

import (
	"io/ioutil"
	"strings"
	"testing"

	pkgviz "github.com/tiegz/pkgviz/pkg/pkgviz"
)

func TestWriteGraphWithBasicTypes(t *testing.T) {
	assertGraph(
		t,
		"../../pkg/fake_pkg/fake_basic_pkg",
		"../../pkg/fake_pkg/fake_basic_pkg/fake_basic_pkg.dot",
	)
}

func TestWriteGraphWithStructTypes(t *testing.T) {
	assertGraph(
		t,
		"../../pkg/fake_pkg/fake_struct_pkg",
		"../../pkg/fake_pkg/fake_struct_pkg/fake_struct_pkg.dot",
	)
}

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
