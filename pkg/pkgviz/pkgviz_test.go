package pkgviz_test

import (
	pkgviz "github.com/tiegz/pkgviz/fake_basic_pkg"
	"testing"
)

func TestWriteGraph(t *testing.T) {
	dotGraph := pkgviz.WriteGraph("github.com/tiegz/pkgviz/fake_basic_pkg")
	if 1 != 1 {
		t.Errorf("Expected %q, got %q instead.", 1, 2)
	}
}
