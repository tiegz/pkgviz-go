

// command line:
// go build pkgviz.go && ./pkgviz time | tee > foo.dot && dot -Tpng foo.dot -o foo.png && open foo.png

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
)

type GoListResult struct {
	Dir        string
	ImportPath string
	GoFiles    []string
	Imports    []string
}

func listGoFilesInPackage(pkg string) GoListResult {
	var listCmdOut []byte
	var err error
	if listCmdOut, err = exec.Command("go", "list", "-json", pkg).Output(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Printf("Debug: %v\n", listCmdOut)
		os.Exit(1)
	}

	var data GoListResult
	if err := json.Unmarshal(listCmdOut, &data); err != nil {
		panic(err)
	}

	return data
}

func recursivelyFetchPackageFiles(importedPkg string, indentLevel int) {
	pkgLabel, _ := labelizeName("", importedPkg)
	fmt.Printf("  subgraph pkg%v { \n", pkgLabel)
	listData := listGoFilesInPackage(importedPkg)

	fset := token.NewFileSet()
	var files []*ast.File
	for _, file := range listData.GoFiles {
		filepath := path.Join(listData.Dir, file)
		f, err := parser.ParseFile(fset, filepath, nil, 0)
		if err != nil {
			log.Fatal(err)
		}
		files = append(files, f)
	}
	checkFiles(importedPkg, fset, files, indentLevel+1)
	fmt.Printf("  }\n")

	for _, pkg := range listData.Imports {
		if strings.HasPrefix(pkg, listData.ImportPath) {
			recursivelyFetchPackageFiles(pkg, indentLevel)
		}
	}
}

func checkFiles(importedPkg string, fset *token.FileSet, files []*ast.File, indentLevel int) {
	// Type-check the package. Setup the maps that Check will fill.
	info := types.Info{
		Defs:   make(map[*ast.Ident]types.Object),
		// Types:  make(map[ast.Expr]types.TypeAndValue),
		// Uses:   make(map[*ast.Ident]types.Object),
		// Scopes: make(map[ast.Node]*types.Scope),
	}

	var conf types.Config = types.Config{
		Importer:                 importer.For("source", nil), // importer.Default(),
		DisableUnusedImportCheck: true,
		FakeImportC:              true,
		Error: func(err error) {
			fmt.Printf("There was an Importer err: %v\n", err)
		},
	}

	_, err := conf.Check("", fset, files, &info) // TODO: what is the first arg for?
	if err != nil {
		log.Fatal(err)
	}

	for id, obj := range info.Defs {
		if _, ok := obj.(*types.TypeName); ok {
			// fmt.Printf("Type debug for %v:\n  Of type: %T\n  Id: %v\n  Name: %v\n  String: %v\n  Type: %v\n  Underlying Type: %v\n  Pos: %v\n  Pkg: %v\n",
			// 	id, typeName, typeName.Id(), typeName.Name(), typeName.String(), typeName.Type(), typeName.Type().Underlying(), lineCol, typeName.Pkg(),
			// )

			// Print out all the named types
			posn := fset.Position(id.Pos())
			_ = printNamedType(obj, posn, importedPkg, indentLevel)
		}
	}
}

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		log.Fatalln("error: no package name given")
		return
	}

	fmt.Printf("digraph graphname {\n")
	fmt.Printf("  graph [label=< <br/><b>%s</b> >, labelloc=b, fontsize=10];\n", args[0])
	recursivelyFetchPackageFiles(args[0], 1)
	fmt.Println("}")
}

func labelizeName(pkgName, typeName string) (string, bool) {
	isPointer := strings.Contains(typeName, "*")
	label := strings.Replace(typeName, "*", "", -1) // remove pointers, handle them separately by returning bool
	label = strings.Replace(label, "/", "SLASH", -1)
	label = strings.Replace(label, "[]", "ARY", -1)
	label = strings.Replace(label, "{}", "BRACES", -1)
	label = strings.Replace(label, ",", "COMMA", -1)
	label = strings.Replace(label, "(", "LPARENS", -1)
	label = strings.Replace(label, ")", "RPARENS", -1)
	label = strings.Replace(label, " ", "", -1)
	// If the type is from another package, don't prepend this package's name to it
	if strings.Contains(label, ".") {
		// TODO: handle cases when it's in another package
		label = strings.Replace(label, ".", "_DOT_", -1)
	} else {
		label = strings.Join([]string{pkgName, label}, "_")
	}
	return strings.ToUpper(label), isPointer
}

func printNamedType(obj types.Object, posn token.Position, importedPkg string, indentLevel int) string {
	switch namedTypeType := obj.Type().Underlying().(type) {
	case *types.Basic:
		return printBasic(obj, namedTypeType, importedPkg, indentLevel)
	case *types.Struct:
		return printStruct(obj, namedTypeType, importedPkg, posn, indentLevel)
	case *types.Interface:
		return printInterface(obj, namedTypeType, importedPkg, posn, indentLevel)
	case *types.Pointer:
		return printPointer(obj, namedTypeType, importedPkg, posn, indentLevel)
	case *types.Signature:
		return printSignature(obj, namedTypeType, importedPkg, indentLevel)
	case *types.Chan:
		return printChan(obj, namedTypeType, importedPkg, indentLevel)
	case *types.Slice:
		return printSlice(obj, namedTypeType, importedPkg, indentLevel)
	case *types.Map:
		return printMap(obj, namedTypeType, importedPkg, indentLevel)
	default:
		fmt.Printf(
			"    // Unkonwn: %v <%T> - %v <%T>\n",
			obj, obj,
			namedTypeType, namedTypeType,
		)
		return "UNKNOWN"
	}
}

func printBasic(obj types.Object, b *types.Basic, importedPkg string, indentLevel int) string {
	typeString := obj.Type().String()
	typeId, _ := labelizeName("main", typeString)

	fmt.Printf("%s%v [shape=record, label=\"%v (%s)\"];\n", strings.Repeat("  ", indentLevel), typeId, typeString, b)

	return typeId
}

func printChan(obj types.Object, c *types.Chan, importedPkg string, indentLevel int) string {
	chanType := c.Elem()
	cPkgName, _ := obj.Pkg().Name(), obj.Name()
	typeName := strings.Join([]string{"chan", chanType.String()}, "_")
	typeId, _ := labelizeName(cPkgName, typeName)

	fmt.Printf("%s%v [shape=record, label=\"chan %s\", color=\"gray\"]\n", strings.Repeat("  ", indentLevel), typeId, chanType.String())

	return typeId
}

func printSlice(obj types.Object, s *types.Slice, importedPkg string, indentLevel int) string {
	sliceType := s.Elem()
	sPkgName, _ := obj.Pkg().Name(), obj.Name()
	typeName := strings.Join([]string{"chan", sliceType.String()}, "_")
	typeId, _ := labelizeName(sPkgName, typeName)

	fmt.Printf(
		"%s%v [shape=record, label=\"%s\", color=\"gray\"]\n",
		strings.Repeat("  ", indentLevel),
		typeId,
		s,
	)

	return typeId
}


func printMap(obj types.Object, m *types.Map, importedPkg string, indentLevel int) string {
	mapType := m.Elem()
	mPkgName, _ := obj.Pkg().Name(), obj.Name()
	typeName := strings.Join([]string{"chan", mapType.String()}, "_")
	typeId, _ := labelizeName(mPkgName, typeName)

	// TODO: break down the map more and point each level to its type?
	fmt.Printf(
		"%s%v [shape=record, label=\"%s\", color=\"gray\"]\n",
		strings.Repeat("  ", indentLevel),
		typeId,
		m,
	)

	return typeId
}

func printSignature(obj types.Object, s *types.Signature, importedPkg string, indentLevel int) string {
	typeString := obj.Type().String()
	typeId, _ := labelizeName("main", typeString)

	fmt.Printf(
		"%s%v [shape=record, label=\"%s\", color=\"gray\"]\n",
		strings.Repeat("  ", indentLevel),
		typeId,
		// TODO: how can we escape in the label instead of removing {}?
		strings.Replace(strings.Replace(typeString, "{", "", -1), "}", "", -1),
	)

	return typeId
}

func printPointer(obj types.Object, p *types.Pointer, importedPkg string, posn token.Position, indentLevel int) string {
	pointerType := p.Elem()
	pPkgName, _ := obj.Pkg().Name(), obj.Name()
	typeId, _ := labelizeName(pPkgName, pointerType.String())

	// TODO finish? make sure it looks like a pointer
	// fmt.Printf("%s%v [shape=record, label=\"pointer\", color=\"gray\"]\n", strings.Repeat("  ", indentLevel), typeId)

	return typeId
}

func printStruct(obj types.Object, ss *types.Struct, importedPkg string, posn token.Position, indentLevel int) string {
	sPkgName, sName := obj.Pkg().Name(), obj.Name()
	typeId, _ := labelizeName(sPkgName, sName)
	pathAry := strings.Split(posn.String(), importedPkg)
	fileAndPosn := pathAry[len(pathAry)-1]

	fmt.Printf("%s%v [shape=record, label=\"{{%v|struct}|%v}\", color=%v]\n", strings.Repeat("  ", indentLevel), typeId, sName, fileAndPosn, "orange") // FOONODE [shape=record, label="{{FOONODE|5}|11}"];
	for i := 0; i < ss.NumFields(); i++ {
		f := ss.Field(i)
		toTypeId := printNamedType(f, posn, importedPkg, indentLevel)
		fmt.Printf(
			"%s%v -> %v [label=\"%v\"];\n",
			strings.Repeat("  ", indentLevel),
			typeId,
			toTypeId,
			f.Name(),
		)
	}
	return typeId
}

func printInterface(obj types.Object, i *types.Interface, importedPkg string, posn token.Position, indentLevel int) string {
	iPkgName, iName := obj.Pkg().Name(), obj.Type().String()

	typeId, _ := labelizeName(iPkgName, iName)
	pathAry := strings.Split(posn.String(), importedPkg)
	fileAndPosn := pathAry[len(pathAry)-1]

	fmt.Printf(
		"%s%v [shape=record, label=\"{{interface}|%v}\", color=%v]\n",
		strings.Repeat("  ", indentLevel),
		typeId,
		fileAndPosn,
		"red",
	)

	return typeId
}
