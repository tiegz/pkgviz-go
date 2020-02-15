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
	"reflect"
	"strings"
)

type GoListResult struct {
	Dir        string
	ImportPath string
	GoFiles    []string
	Imports    []string
}

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		log.Fatalln("error: no package name given")
		return
	}

	fmt.Printf("digraph V {\n")
	fmt.Printf("  graph [label=< <br/><b>%s</b> >, labelloc=b, fontsize=10 fontname=Arial];\n", args[0])
	fmt.Printf("  node [fontname=Arial];\n")
	fmt.Printf("  edge [fontname=Arial];\n")
	recursivelyFetchPackageFiles(args[0], 1)
	fmt.Println("}")
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
	pkgLabel := labelizeName("", importedPkg)
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
	printNamedTypesFromTypeChecker(importedPkg, fset, files, indentLevel+1)
	fmt.Printf("  }\n")

	for _, pkg := range listData.Imports {
		if strings.HasPrefix(pkg, listData.ImportPath) {
			recursivelyFetchPackageFiles(pkg, indentLevel)
		}
	}
}

func printNamedTypesFromTypeChecker(importedPkg string, fset *token.FileSet, files []*ast.File, indentLevel int) {
	// Type-check the package. Setup the maps that Check will fill.
	info := types.Info{
		Defs: make(map[*ast.Ident]types.Object),
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

	// Print out all the Named types
	for id, obj := range info.Defs {
		if _, ok := obj.(*types.TypeName); ok {
			printType(obj, fset.Position(id.Pos()), importedPkg, indentLevel)
		}
	}

	// Print out all the links among Named types
	for id, obj := range info.Defs {
		if _, ok := obj.(*types.TypeName); ok {
			printTypeLinks(obj, fset.Position(id.Pos()), importedPkg, indentLevel)
		}
	}
}

// Turn a type string into a graphviz-friendly label, e.g. `func(interface{}, uintptr)` -> funclparensinterfacebracescommauintptrrparens
func labelizeName(pkgName, typeName string) string {
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
	return strings.ToLower(label)
}

func printTypeLinks(obj types.Object, posn token.Position, importedPkg string, indentLevel int) {
	switch namedTypeType := obj.Type().Underlying().(type) {
	case *types.Struct:
		printStructLinks(obj, namedTypeType, importedPkg, posn, indentLevel)
	default:
		// no-op
	}
}

func printType(obj types.Object, posn token.Position, importedPkg string, indentLevel int) {
	// Only print named types
	if reflect.TypeOf(obj.Type()).String() != "*types.Named" {
		return
	}

	switch namedTypeType := obj.Type().Underlying().(type) {
	case *types.Basic:
		printBasic(obj, namedTypeType, importedPkg, indentLevel)
	case *types.Interface:
		printInterface(obj, namedTypeType, importedPkg, posn, indentLevel)
	case *types.Pointer:
		printPointer(obj, namedTypeType, importedPkg, posn, indentLevel)
	case *types.Signature:
		printSignature(obj, namedTypeType, importedPkg, indentLevel)
	case *types.Chan:
		printChan(obj, namedTypeType, importedPkg, indentLevel)
	case *types.Slice:
		printSlice(obj, namedTypeType, importedPkg, indentLevel)
	case *types.Map:
		printMap(obj, namedTypeType, importedPkg, indentLevel)
	case *types.Struct:
		printStruct(obj, namedTypeType, importedPkg, posn, indentLevel)
	default:
		fmt.Printf(
			"    // Unknown: %v <%T> - %v <%T>\n",
			obj, obj,
			namedTypeType, namedTypeType,
		)
	}
}

func printBasic(obj types.Object, b *types.Basic, importedPkg string, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name())
	typeString := obj.Type().String()

	fmt.Printf(
		`%s%v [shape=plaintext label=< `+
			`<table border='2' cellborder='0' cellspacing='0' style='rounded' color='#4BAAD3'>`+
			`<tr><td bgcolor='#e0ebf5' align='center'>&nbsp;&nbsp;%v&nbsp;&nbsp;</td></tr>`+
			`<tr><td align='center'>&nbsp;&nbsp;%s&nbsp;&nbsp;</td></tr>`+
			`</table> >];
		`,
		strings.Repeat("  ", indentLevel),
		typeId,
		typeString,
		b,
	)
}

func printChan(obj types.Object, c *types.Chan, importedPkg string, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name())
	chanType := c.Elem()

	fmt.Printf("%s%v [shape=record, label=\"chan %s\", color=\"gray\"];\n", strings.Repeat("  ", indentLevel), typeId, chanType.String())
}

func printSlice(obj types.Object, s *types.Slice, importedPkg string, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name())

	fmt.Printf(
		"%s%v [shape=record, label=\"%s\", color=\"gray\"];\n",
		strings.Repeat("  ", indentLevel),
		typeId,
		s,
	)
}

func printMap(obj types.Object, m *types.Map, importedPkg string, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name())

	// TODO: break down the map more and point each level to its type?
	fmt.Printf(
		"%s%v [shape=record, label=\"%s\", color=\"gray\"]\n",
		strings.Repeat("  ", indentLevel),
		typeId,
		m,
	)
}

func printSignature(obj types.Object, s *types.Signature, importedPkg string, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name())
	typeString := obj.Type().String()

	fmt.Printf(
		"%s%v [shape=record, label=\"%s\", color=\"gray\"]\n",
		strings.Repeat("  ", indentLevel),
		typeId,
		// TODO: how can we escape in the label instead of removing {}?
		strings.Replace(strings.Replace(typeString, "{", "", -1), "}", "", -1),
	)
}

func printPointer(obj types.Object, p *types.Pointer, importedPkg string, posn token.Position, indentLevel int) {
	// TODO finish? make sure it looks like a pointer
	// fmt.Printf("%s%v [shape=record, label=\"pointer\", color=\"gray\"]\n", strings.Repeat("  ", indentLevel), typeId)
}

func printStruct(obj types.Object, ss *types.Struct, importedPkg string, posn token.Position, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name())
	sName := obj.Name()

	pathAry := strings.Split(posn.String(), importedPkg)
	var fileAndPosn string
	// TODO: make file location/column an optional flag
	if false {
		fileAndPosn = pathAry[len(pathAry)-1]
	} else {
		fileAndPosn = ""
	}

	structLabel := fmt.Sprintf(
		`%s%v [shape=plaintext label=<
			<table border='2' cellborder='0' cellspacing='0' style='rounded' color='#4BAAD3'>
				<tr><td bgcolor='#e0ebf5' align='center' colspan='2'>&nbsp;&nbsp;%s %s&nbsp;&nbsp;</td></tr>
		`,
		strings.Repeat("  ", indentLevel),
		typeId,
		sName,
		fileAndPosn,
	)
	for i := 0; i < ss.NumFields(); i++ {
		f := ss.Field(i)
		printType(f, posn, importedPkg, indentLevel)
		fTypeId := getTypeId(f.Type(), f.Pkg().Name())
		structLabel = fmt.Sprintf(
			"%s%s<tr><td port='port_%s' align='left'>&nbsp;&nbsp;%s&nbsp;&nbsp;</td><td align='left'><font color='#7f8183'>&nbsp;&nbsp;%s&nbsp;&nbsp;</font></td></tr>\n",
			structLabel,
			strings.Repeat("  ", indentLevel),
			fTypeId,
			f.Name(),
			escapeHtml(f.Type().String()),
		)
	}
	structLabel = fmt.Sprintf(`
		%s</table>
		>];%s`,
		structLabel,
		"\n",
	)
	fmt.Print(structLabel)
}

func printStructLinks(obj types.Object, ss *types.Struct, importedPkg string, posn token.Position, indentLevel int) {
	structTypeId := getTypeId(obj.Type(), obj.Pkg().Name())

	// TODO: move this into the printTypeLinks() func
	for i := 0; i < ss.NumFields(); i++ {
		f := ss.Field(i)
		fieldId := getTypeId(f.Type(), f.Pkg().Name())
		fTypeType := reflect.TypeOf(f.Type()).String()

		toTypeId := fieldId
		// Link to underlying type instead of slice-of-underlying type
		if containerType := getContainerType(f.Type()); containerType != nil {
			// TODO: importedPkg may be wrong here, it could be another package. How to fix?
			toTypeId = labelizeName(importedPkg, containerType.String())
		}

		// Don't link to basic types or containers of basic types.
		isSignature := fTypeType == "*types.Signature"
		isBasic := fTypeType == "*types.Basic"
		// HACK: better way to do this, e.g. chedcking NumExplicitMethods > 0?
		isEmptyInterface := fieldId == "time_interfacebraces"
		isContainerOfBasic := containerElemIsBasic(f.Type())

		if !isEmptyInterface && !isSignature && !isBasic && !isContainerOfBasic {
			fmt.Printf(
				"%s%s:port_%s -> %s;\n",
				strings.Repeat("  ", indentLevel),
				structTypeId,
				fieldId,
				toTypeId,
			)
		}
	}
}

func printInterface(obj types.Object, i *types.Interface, importedPkg string, posn token.Position, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name())
	if i.NumExplicitMethods() > 0 {
		interfaceLabel := fmt.Sprintf(
			`%s%v [shape=plaintext label=<
				<table border='2' cellborder='0' cellspacing='0' style='rounded' color='#4BAAD3'>
					<tr><td bgcolor='#e0ebf5' align='center' colspan='%d'>%s interface</td></tr>
			`,
			strings.Repeat("  ", indentLevel),
			typeId,
			i.NumExplicitMethods()+1,
			obj.Name(),
		)

		for idx := 0; idx < i.NumExplicitMethods(); idx += 1 {
			m := i.ExplicitMethod(idx)
			interfaceLabel = fmt.Sprintf(
				"%s%s<tr><td>%s <font color='#d0dae5'>%s</font></td></tr>\n",
				interfaceLabel,
				strings.Repeat("  ", indentLevel),
				m.Name(),
				m.Type(),
			)
		}
		interfaceLabel = fmt.Sprintf(`
			%s</table>
			>];%s`,
			interfaceLabel,
			"\n",
		)
		fmt.Print(interfaceLabel)
	}
}

func escapeHtml(s string) string {
	str := strings.Replace(s, "<", "&lt;", -1)
	str = strings.Replace(str, ">", "&gt;", -1)
	return str
}

func getTypeAssertion(t types.Type) types.Type {
	switch typeType := t.(type) {
	default:
		return typeType
	}
}

func getContainerType(t types.Type) types.Type {
	var containerType types.Type
	switch typeType := t.(type) {
	case *types.Array:
		containerType = getTypeAssertion(typeType.Elem())
	case *types.Map:
		containerType = getTypeAssertion(typeType.Elem())
	case *types.Chan:
		containerType = getTypeAssertion(typeType.Elem())
	case *types.Slice:
		containerType = getTypeAssertion(typeType.Elem())
	}
	return containerType
}

// For chans, slices, etc that have an underlying type.
func containerElemIsBasic(t types.Type) bool {
	switch typeType := t.(type) {
	case *types.Slice:
		switch typeType.Elem().(type) {
		case *types.Basic:
			return true
		default:
			return false
		}
	case *types.Chan:
		switch typeType.Elem().(type) {
		case *types.Basic:
			return true
		default:
			return false
		}
	case *types.Array:
		switch typeType.Elem().(type) {
		case *types.Basic:
			return true
		default:
			return false
		}
	case *types.Map:
		switch typeType.Elem().(type) {
		case *types.Basic:
			return true
		default:
			return false
		}
	default:
		return false // not actually a slice
	}
	return false
}

func getTypeId(t types.Type, typePkgName string) string {
	var typeId, typeName string

	switch namedTypeType := t.Underlying().(type) {
	case *types.Basic:
		typePkgName, typeName = "main", t.String()
	case *types.Chan:
		containerType := namedTypeType.Elem()
		typeName = strings.Join([]string{"chan", containerType.String()}, "_")
	case *types.Slice:
		sliceType := namedTypeType.Elem()
		typeName = strings.Join([]string{"slice", sliceType.String()}, "_")
	case *types.Struct:
		typeName = t.String()
	case *types.Interface:
		typeName = t.String()
		typeId = labelizeName(typePkgName, typeName)
	case *types.Pointer:
		pointerType := namedTypeType.Elem()
		typeName = pointerType.String()
	case *types.Signature:
		typePkgName, typeName = "main", t.String()
	case *types.Map:
		mapType := namedTypeType.Elem()
		typeName = strings.Join([]string{"chan", mapType.String()}, "_")
	}

	typeId = labelizeName(typePkgName, typeName)

	return typeId
}
