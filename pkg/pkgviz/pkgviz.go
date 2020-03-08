package pkgviz

import (
	"encoding/json"
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

type goListResult struct {
	Dir        string
	ImportPath string
	GoFiles    []string
	Imports    []string
}

type dotGraphStructField struct {
	structFieldId string
	// structFieldName     string
	structFieldTypeName string
}

type dotGraphNode struct {
	pkgName              string
	typeId               string
	typeType             string
	typeUnderlyingType   string // e.g. for Basic underlying types, containers, etc
	typeName             string
	typeNodes            map[string]*dotGraphNode        // id -> node
	typeStructFields     map[string]*dotGraphStructField // name -> node (of field type)
	typeInterfaceMethods map[string]string               // name -> type
	typeLinks            []string
}

func WriteGraph(pkgName string) string {
	dotGraph := BuildGraph(pkgName)

	str := dotGraph.Print("", pkgName, 0)

	return str
}

func (dgn *dotGraphNode) Print(out string, pkgName string, indentLevel int) string {
	// fmt.Printf("Printing %v\t (%v vs %v): %v, %v\n", dgn.typeType, pkgName, dgn.pkgName, dgn.typeId, dgn.typeName)

	switch dgn.typeType {
	case "root":
		out = fmt.Sprintf("digraph V {\n"+
			"  graph [label=< <br/><b>%s</b> >, labelloc=b, fontsize=10 fontname=Arial];\n"+
			"  node [fontname=Arial];\n"+
			"  edge [fontname=Arial];\n",
			pkgName,
		)
		for _, node := range dgn.typeNodes {
			// fmt.Printf("Debug writing %v -> %v\n", id, node)
			if pkgName != node.pkgName {
				out = fmt.Sprintf(
					"%s%ssubgraph cluster_%v { \n",
					out,
					strings.Repeat("  ", indentLevel+1),
					node.typeId,
				)
				out = node.Print(out, pkgName, indentLevel+2)
				out = fmt.Sprintf("%s%snode [style=filled];\n", out, strings.Repeat("  ", indentLevel+2))
				out = fmt.Sprintf("%s%slabel=\"%s\";\n", out, strings.Repeat("  ", indentLevel+2), relativizeTypePkgName(node.pkgName, pkgName))
				out = fmt.Sprintf("%s%sgraph[style=dotted color=\"#7f8183\"];\n", out, strings.Repeat("  ", indentLevel+2))
				out = fmt.Sprintf("%s%s}\n", out, strings.Repeat("  ", indentLevel+1))
			} else {
				out = node.Print(out, pkgName, indentLevel+1)
			}
		}
		for _, typeLink := range dgn.typeLinks {
			out = fmt.Sprintf("%s  %s", out, typeLink)
		}
		out = fmt.Sprintf("%s}\n", out)
	case "struct":
		out = fmt.Sprintf("%s%s%s [shape=plaintext label=<"+
			"<table border='2' cellborder='0' cellspacing='0' style='rounded' color='#4BAAD3'>"+
			"<tr><td bgcolor='#e0ebf5' align='center' colspan='2'>%s</td></tr>",
			out,
			strings.Repeat("  ", indentLevel),
			dgn.typeId,
			dgn.typeName,
		)
		for structFieldName, structFieldNode := range dgn.typeStructFields {
			// fmt.Printf("  Debug struct %v - %v - %v ...\n", structFieldNode.structFieldTypeName, pkgName, structFieldName)
			out = fmt.Sprintf(
				"%s<tr><td port='port_%s' align='left'>%s</td><td align='left'><font color='#7f8183'>%s</font></td></tr>",
				out,
				structFieldNode.structFieldId,
				structFieldName,
				escapeHtml(relativizeTypePkgName(structFieldNode.structFieldTypeName, pkgName)),
			)
		}
		out = fmt.Sprintf("%s</table> >];\n", out)
	case "basic":
		out = fmt.Sprintf("%s%s%s [shape=plaintext label=< "+
			"<table border='2' cellborder='0' cellspacing='0' style='rounded' color='#4BAAD3'>"+
			"<tr><td bgcolor='#e0ebf5' align='center'>%v</td></tr>"+
			"<tr><td align='center'>%s</td></tr>"+
			"</table> >];\n",
			out,
			strings.Repeat("  ", indentLevel),
			dgn.typeId,
			dgn.typeName,
			dgn.typeUnderlyingType,
		)
	case "interface":
		out = fmt.Sprintf("%s%s%v [shape=plaintext label=< "+
			"<table border='2' cellborder='0' cellspacing='0' style='rounded' color='#4BAAD3'>"+
			"<tr><td bgcolor='#e0ebf5' align='center' colspan='%d'>%s interface</td></tr>",
			out,
			strings.Repeat("  ", indentLevel),
			dgn.typeId,
			len(dgn.typeInterfaceMethods),
			dgn.typeName,
		)
		for methodName, methodType := range dgn.typeInterfaceMethods {
			out = fmt.Sprintf("%s<tr><td>%s <font color='#d0dae5'>%s</font></td></tr>", out, methodName, methodType)
		}
		out = fmt.Sprintf("%s</table>>];\n", out)
	case "pointer":
		out = fmt.Sprintf(
			"%s\n%s%v [shape=record, label=\"pointer\", color=\"gray\"]\n",
			out,
			strings.Repeat("  ", indentLevel),
			dgn.typeId,
		)
	case "signature":
		fmt.Printf(
			"%s\n%s%v [shape=record, label=\"%s\", color=\"gray\"]\n",
			out,
			strings.Repeat("  ", indentLevel),
			dgn.typeId,
			// TODO: how can we escape in the label instead of removing {}?
			strings.Replace(strings.Replace(dgn.typeName, "{", "", -1), "}", "", -1),
		)
	case "chan":
		out = fmt.Sprintf(
			"%s\n%s%v [shape=record, label=\"chan %s\", color=\"gray\"];\n",
			out,
			strings.Repeat("  ", indentLevel),
			dgn.typeName,
			dgn.typeUnderlyingType,
		)
	case "slice":
		out = fmt.Sprintf(
			"%s\n%s%v [shape=record, label=\"%s\", color=\"gray\"];\n",
			out,
			strings.Repeat("  ", indentLevel),
			dgn.typeName,
			dgn.typeUnderlyingType,
		)
	case "map":
		// TODO: break down the map more and point each level to its type?
		fmt.Printf(
			"%s\n%s%v [shape=record, label=\"%s\", color=\"gray\"]\n",
			out,
			strings.Repeat("  ", indentLevel),
			dgn.typeName,
			dgn.typeUnderlyingType,
		)
	default:
		panic(dgn.typeType)
		//fmt.Printf("Unknown %v\n", dgn.typeType)
	}

	return out
}

func BuildGraph(pkgName string) *dotGraphNode {
	root := dotGraphNode{
		pkgName:          pkgName,
		typeId:           "root",
		typeType:         "root",
		typeName:         pkgName,
		typeNodes:        map[string]*dotGraphNode{},
		typeStructFields: map[string]*dotGraphStructField{},
	}

	recursivelyBuildGraph(&root, pkgName, pkgName)

	return &root
}

func recursivelyBuildGraph(dg *dotGraphNode, rootPkgName, pkgName string) { // , indentLevel int
	listData := listGoFilesInPackage(pkgName)

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

	// If the package is a part of the root package, just trim the
	// root package prefix so it's shorter to read.
	normalizedPkgName := strings.TrimPrefix(strings.TrimPrefix(pkgName, rootPkgName), "/")
	addTypesToGraph(dg, normalizedPkgName, fset, files)

	for _, pkgName := range listData.Imports {
		if strings.HasPrefix(pkgName, listData.ImportPath) {
			// fmt.Printf("In imported pkg: %v\n", pkgName)
			recursivelyBuildGraph(dg, rootPkgName, pkgName) // indentLevel
		}
	}
}

func listGoFilesInPackage(pkg string) goListResult {
	var listCmdOut []byte
	var err error

	cmd := exec.Command("go", "list", "-json", pkg)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=1")
	if listCmdOut, err = cmd.Output(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Printf("Debug: %v\n", listCmdOut)
		os.Exit(1)
	}

	var data goListResult
	if err := json.Unmarshal(listCmdOut, &data); err != nil {
		fmt.Printf("Error finding %v\n", pkg)
		panic(err)
	}

	return data
}

// func printNamedTypesFromTypeChecker(pkgName string, fset *token.FileSet, files []*ast.File, indentLevel int) {
func addTypesToGraph(dg *dotGraphNode, pkgName string, fset *token.FileSet, files []*ast.File) {
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
			// fmt.Printf("Adding pkg %v: %v\n", obj, pkgName)
			addTypeToGraph(dg, obj, fset.Position(id.Pos()), pkgName)
		}
	}

	// Print out all the links among Named types
	for id, obj := range info.Defs {
		if _, ok := obj.(*types.TypeName); ok {
			addTypeLinkToGraph(dg, obj, fset.Position(id.Pos()), pkgName)
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

func addTypeLinkToGraph(dg *dotGraphNode, obj types.Object, posn token.Position, pkgName string) {
	switch namedTypeType := obj.Type().Underlying().(type) {
	case *types.Struct:
		addStructLinksToGraph(dg, obj, namedTypeType, pkgName, posn)
	default:
		// no-op
	}
}

func addTypeToGraph(dg *dotGraphNode, obj types.Object, posn token.Position, pkgName string) { // , indentLevel int
	// Only print named types
	if reflect.TypeOf(obj.Type()).String() != "*types.Named" {
		return
	}

	switch namedTypeType := obj.Type().Underlying().(type) {
	case *types.Basic:
		addBasicToGraph(dg, obj, namedTypeType, pkgName) // , indentLevel)
	case *types.Interface:
		addInterfaceToGraph(dg, obj, namedTypeType, pkgName, posn) // , indentLevel)
	case *types.Pointer:
		addPointerToGraph(dg, obj, namedTypeType, pkgName, posn) // , indentLevel)
	case *types.Signature:
		addSignatureToGraph(dg, obj, namedTypeType, pkgName) // , indentLevel)
	case *types.Chan:
		addChanToGraph(dg, obj, namedTypeType, pkgName) // , indentLevel)
	case *types.Slice:
		addSliceToGraph(dg, obj, namedTypeType, pkgName) // , indentLevel)
	case *types.Map:
		addMapToGraph(dg, obj, namedTypeType, pkgName) // , indentLevel)
	case *types.Struct:
		addStructToGraph(dg, obj, namedTypeType, pkgName, posn) // , posn, indentLevel)
	default:
		fmt.Printf(
			"    // Unknown: %v <%T> - %v <%T>\n",
			obj, obj,
			namedTypeType, namedTypeType,
		)
	}
}

func addBasicToGraph(dg *dotGraphNode, obj types.Object, b *types.Basic, pkgName string) { // , indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	// TODO: check key first
	node := dotGraphNode{
		pkgName:            pkgName,
		typeId:             typeId,
		typeType:           "basic",
		typeName:           obj.Type().String(),
		typeUnderlyingType: b.String(),
		typeNodes:          map[string]*dotGraphNode{},
		typeStructFields:   map[string]*dotGraphStructField{},
	}
	// fmt.Printf("printBasic: %v ... %v\n", node)

	dg.typeNodes[typeId] = &node
}

func addChanToGraph(dg *dotGraphNode, obj types.Object, c *types.Chan, pkgName string) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	dg.typeNodes[typeId] = &dotGraphNode{
		pkgName:          pkgName,
		typeId:           typeId,
		typeType:         "chan",
		typeName:         c.Elem().String(),
		typeNodes:        map[string]*dotGraphNode{},
		typeStructFields: map[string]*dotGraphStructField{},
	}
}

func addSliceToGraph(dg *dotGraphNode, obj types.Object, s *types.Slice, pkgName string) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	dg.typeNodes[typeId] = &dotGraphNode{
		pkgName:            pkgName,
		typeId:             typeId,
		typeType:           "slice",
		typeUnderlyingType: s.String(),
		typeName:           s.String(),
		typeNodes:          map[string]*dotGraphNode{},
		typeStructFields:   map[string]*dotGraphStructField{},
	}
}

func addMapToGraph(dg *dotGraphNode, obj types.Object, m *types.Map, pkgName string) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	dg.typeNodes[typeId] = &dotGraphNode{
		pkgName:          pkgName,
		typeId:           typeId,
		typeType:         "map",
		typeName:         m.String(),
		typeNodes:        map[string]*dotGraphNode{},
		typeStructFields: map[string]*dotGraphStructField{},
	}
}

func addSignatureToGraph(dg *dotGraphNode, obj types.Object, s *types.Signature, pkgName string) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)
	typeString := obj.Type().String()
	// TODO: how can we escape in the label instead of removing {}?
	typeString = strings.Replace(strings.Replace(typeString, "{", "", -1), "}", "", -1)

	dg.typeNodes[typeId] = &dotGraphNode{
		pkgName:          pkgName,
		typeId:           typeId,
		typeType:         "signature",
		typeName:         typeString,
		typeNodes:        map[string]*dotGraphNode{},
		typeStructFields: map[string]*dotGraphStructField{},
	}
}

func addPointerToGraph(dg *dotGraphNode, obj types.Object, p *types.Pointer, pkgName string, posn token.Position) { //, indentLevel int) {
	// TODO finish? make sure it looks like a pointer
	// dg.typeNodes[typeId] = &dotGraphNode{
	// pkgName:            pkgName,
	//	typeId: typeId,
	// 	typeType: "pointer",
	// 	typeName: p.String(),
	//  typeNodes: map[string]*dotGraphNode{},
	// }
}

func addStructToGraph(dg *dotGraphNode, obj types.Object, ss *types.Struct, pkgName string, posn token.Position) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)
	// sName := obj.Name()

	node := dotGraphNode{
		pkgName:          pkgName,
		typeId:           typeId,
		typeType:         "struct",
		typeName:         obj.Name(),
		typeNodes:        map[string]*dotGraphNode{},
		typeStructFields: map[string]*dotGraphStructField{},
	}

	for i := 0; i < ss.NumFields(); i++ {
		field := ss.Field(i)
		// fmt.Printf("debug:  (%v) adding struct field %v from %v\n", pkgName, field.Name(), fieldPkgName)
		fieldTypeId := getTypeId(field.Type(), field.Pkg().Name(), pkgName)
		node.typeStructFields[field.Name()] = &dotGraphStructField{
			structFieldId:       fieldTypeId,
			structFieldTypeName: escapeHtml(field.Type().String()),
		}
	}

	dg.typeNodes[typeId] = &node
}

func addStructLinksToGraph(dg *dotGraphNode, obj types.Object, ss *types.Struct, pkgName string, posn token.Position) { // , indentLevel int
	structTypeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	// TODO: move this into the printTypeLinks() func
	for i := 0; i < ss.NumFields(); i++ {
		f := ss.Field(i)
		fieldId := getTypeId(f.Type(), f.Pkg().Name(), pkgName)
		fTypeType := reflect.TypeOf(f.Type()).String()

		toTypeId := fieldId
		// Link to underlying type instead of slice-of-underlying type
		if containerType := getContainerType(f.Type()); containerType != nil {
			// TODO: pkgName may be wrong here, it could be another package. How to fix?
			toTypeId = labelizeName(pkgName, containerType.String())
		}

		// Don't link to basic types or containers of basic types.
		isSignature := fTypeType == "*types.Signature"
		isBasic := fTypeType == "*types.Basic"
		// HACK: better way to do this, e.g. chedcking NumExplicitMethods > 0?
		isEmptyInterface := fieldId == "time_interfacebraces"
		isContainerOfBasic := containerElemIsBasic(f.Type())

		if !isEmptyInterface && !isSignature && !isBasic && !isContainerOfBasic {
			link := fmt.Sprintf("%s:port_%s -> %s;\n", structTypeId, fieldId, toTypeId)
			dg.typeLinks = append(dg.typeLinks, link)
		}
	}
}

func addInterfaceToGraph(dg *dotGraphNode, obj types.Object, i *types.Interface, pkgName string, posn token.Position) { // indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	node := dotGraphNode{
		pkgName:          pkgName,
		typeId:           typeId,
		typeType:         "interface",
		typeName:         obj.Name(),
		typeNodes:        map[string]*dotGraphNode{},
		typeStructFields: map[string]*dotGraphStructField{},
	}

	if i.NumExplicitMethods() > 0 {
		for idx := 0; idx < i.NumExplicitMethods(); idx += 1 {
			m := i.ExplicitMethod(idx)
			dg.typeInterfaceMethods[m.Name()] = m.Type().String()
		}
	}

	dg.typeNodes[typeId] = &node
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

func getTypeId(t types.Type, typePkgName, originalPkgName string) string {
	// fmt.Printf("debug getTypeId: %v - %v - %v\n", t, typePkgName, originalPkgName)
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

// If the given type is part of pkgName, strip off the pkgName.
// relativizeTypePkgName("github.com/foo/bar/baz.Node", "github.com/foo/bar")
// => "baz.Node"
func relativizeTypePkgName(typeName, pkgName string) string {
	pkgName = pkgName + "/"
	pkgNameAsPointer := "*" + pkgName
	if strings.HasPrefix(typeName, pkgName) {
		return "../" + strings.TrimPrefix(typeName, pkgName)
	} else if strings.HasPrefix(typeName, pkgNameAsPointer) {
		return "../" + strings.TrimPrefix(typeName, pkgNameAsPointer)
	} else {
		return typeName
	}
}
