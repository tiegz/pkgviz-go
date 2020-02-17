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
	structFieldName     string
	structFieldTypeName string
}

type dotGraphNode struct {
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
	fmt.Printf(str)

	return str
}

func (dgn *dotGraphNode) Print(out string, pkgName string, indentLevel int) string {
	//	fmt.Printf("Printing %v\t: %v, %v\n", dgn.typeType, dgn.typeId, dgn.typeName)

	switch dgn.typeType {
	case "root":
		out = fmt.Sprintf(`
digraph V {
	graph [label=< <br/><b>%s</b> >, labelloc=b, fontsize=10 fontname=Arial];
	node [fontname=Arial];
	edge [fontname=Arial];

`,
			pkgName,
		)
		for _, node := range dgn.typeNodes {
			out = node.Print(out, pkgName, indentLevel+1)
		}
		for _, typeLink := range dgn.typeLinks {
			out = fmt.Sprintf("%s%s", out, typeLink)
		}
		out = fmt.Sprintf(`%s}`, out)
	case "struct":
		out = fmt.Sprintf(`
%s%s%s [shape=plaintext label=<
	<table border='2' cellborder='0' cellspacing='0' style='rounded' color='#4BAAD3'>
		<tr><td bgcolor='#e0ebf5' align='center' colspan='2'>&nbsp;&nbsp;%s&nbsp;&nbsp;</td></tr>
`,
			out,
			strings.Repeat("	", indentLevel),
			dgn.typeId,
			dgn.typeName,
			// fileAndPosn,
		)
		for structFieldId, structFieldNode := range dgn.typeStructFields {
			// fTypeId := getTypeId(f.Type(), f.Pkg().Name())
			out = fmt.Sprintf(
				"%s%s<tr><td port='port_%s' align='left'>&nbsp;&nbsp;%s&nbsp;&nbsp;</td><td align='left'><font color='#7f8183'>&nbsp;&nbsp;%s&nbsp;&nbsp;</font></td></tr>\n",
				out,
				strings.Repeat("	", indentLevel+1),
				structFieldId,
				structFieldNode.structFieldName,
				escapeHtml(structFieldNode.structFieldTypeName),
			)
		}
		out = fmt.Sprintf(`%s
%s</table>
			>];%s`,
			out,
			strings.Repeat("	", indentLevel),
			"\n",
		)
	case "basic":
		out = fmt.Sprintf(`%s%s%s [shape=plaintext label=< `+
			`<table border='2' cellborder='0' cellspacing='0' style='rounded' color='#4BAAD3'>`+
			`<tr><td bgcolor='#e0ebf5' align='center'>&nbsp;&nbsp;%v&nbsp;&nbsp;</td></tr>`+
			`<tr><td align='center'>&nbsp;&nbsp;%s&nbsp;&nbsp;</td></tr>`+
			`</table> >];
			`,
			out,
			strings.Repeat("	", indentLevel),
			dgn.typeId,
			dgn.typeName,
			dgn.typeUnderlyingType,
		)
	case "interface":
		out = fmt.Sprintf(
			`%s%s%v [shape=plaintext label=<
				<table border='2' cellborder='0' cellspacing='0' style='rounded' color='#4BAAD3'>
					<tr><td bgcolor='#e0ebf5' align='center' colspan='%d'>%s interface</td></tr>
			`,
			out,
			strings.Repeat("	", indentLevel),
			dgn.typeId,
			len(dgn.typeInterfaceMethods), // i.NumExplicitMethods()+1,
			dgn.typeName,
		)
		for methodName, methodType := range dgn.typeInterfaceMethods {
			out = fmt.Sprintf("%s%s<tr><td>%s <font color='#d0dae5'>%s</font></td></tr>\n", out, strings.Repeat("	", indentLevel), methodName, methodType)
		}
		out = fmt.Sprintf(`
			%s</table>
			>];%s`,
			out,
			"\n",
		)
		fmt.Print(out)
	// case "pointer":
	// case "signature":
	// case "chan":
	// case "slice":
	// case "map":
	default:
		panic(dgn.typeType)
		//fmt.Printf("Unknown %v\n", dgn.typeType)
	}

	return out
}

func BuildGraph(pkgName string) *dotGraphNode {
	root := dotGraphNode{
		typeId:           "root",
		typeType:         "root",
		typeName:         pkgName,
		typeNodes:        map[string]*dotGraphNode{},
		typeStructFields: map[string]*dotGraphStructField{},
	}

	recursivelyBuildGraph(&root, pkgName)

	return &root
}

func recursivelyBuildGraph(dg *dotGraphNode, pkgName string) { // , indentLevel int
	// pkgLabel := labelizeName("", importedPkg)
	// fmt.Printf("  subgraph pkg%v { \n", pkgLabel)
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
	addTypesToGraph(dg, pkgName, fset, files)

	for _, importedPkgName := range listData.Imports {
		if strings.HasPrefix(importedPkgName, listData.ImportPath) {
			recursivelyBuildGraph(dg, importedPkgName) // indentLevel
		}
	}
}

func listGoFilesInPackage(pkg string) goListResult {
	var listCmdOut []byte
	var err error
	if listCmdOut, err = exec.Command("go", "list", "-json", pkg).Output(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Printf("Debug: %v\n", listCmdOut)
		os.Exit(1)
	}

	var data goListResult
	if err := json.Unmarshal(listCmdOut, &data); err != nil {
		fmt.PrintF("Error finding %v\n", pkg)
		panic(err)
	}

	return data
}

// func printNamedTypesFromTypeChecker(importedPkg string, fset *token.FileSet, files []*ast.File, indentLevel int) {
func addTypesToGraph(dg *dotGraphNode, importedPkg string, fset *token.FileSet, files []*ast.File) {
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
			addTypeToGraph(dg, obj, fset.Position(id.Pos()), importedPkg)
		}
	}

	// Print out all the links among Named types
	for id, obj := range info.Defs {
		if _, ok := obj.(*types.TypeName); ok {
			addTypeLinkToGraph(dg, obj, fset.Position(id.Pos()), importedPkg)
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

func addTypeLinkToGraph(dg *dotGraphNode, obj types.Object, posn token.Position, importedPkg string) {
	switch namedTypeType := obj.Type().Underlying().(type) {
	case *types.Struct:
		addStructLinksToGraph(dg, obj, namedTypeType, importedPkg, posn)
	default:
		// no-op
	}
}

func addTypeToGraph(dg *dotGraphNode, obj types.Object, posn token.Position, importedPkg string) { // , indentLevel int
	// func printType(obj types.Object, posn token.Position, importedPkg string, indentLevel int) {
	// Only print named types
	if reflect.TypeOf(obj.Type()).String() != "*types.Named" {
		return
	}

	switch namedTypeType := obj.Type().Underlying().(type) {
	case *types.Basic:
		addBasicToGraph(dg, obj, namedTypeType, importedPkg) // , indentLevel)
	case *types.Interface:
		addInterfaceToGraph(dg, obj, namedTypeType, importedPkg, posn) // , indentLevel)
	case *types.Pointer:
		addPointerToGraph(dg, obj, namedTypeType, importedPkg, posn) // , indentLevel)
	case *types.Signature:
		addSignatureToGraph(dg, obj, namedTypeType, importedPkg) // , indentLevel)
	case *types.Chan:
		addChanToGraph(dg, obj, namedTypeType, importedPkg) // , indentLevel)
	case *types.Slice:
		addSliceToGraph(dg, obj, namedTypeType, importedPkg) // , indentLevel)
	case *types.Map:
		addMapToGraph(dg, obj, namedTypeType, importedPkg) // , indentLevel)
	case *types.Struct:
		addStructToGraph(dg, obj, namedTypeType, importedPkg, posn) // , posn, indentLevel)
	default:
		fmt.Printf(
			"    // Unknown: %v <%T> - %v <%T>\n",
			obj, obj,
			namedTypeType, namedTypeType,
		)
	}
}

func addBasicToGraph(dg *dotGraphNode, obj types.Object, b *types.Basic, importedPkg string) { // , indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name())

	// TODO: check key first
	node := dotGraphNode{
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

func addChanToGraph(dg *dotGraphNode, obj types.Object, c *types.Chan, importedPkg string) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name())
	// chanType := c.Elem()

	dg.typeNodes[typeId] = &dotGraphNode{
		typeId:           typeId,
		typeType:         "chan",
		typeName:         c.Elem().String(),
		typeNodes:        map[string]*dotGraphNode{},
		typeStructFields: map[string]*dotGraphStructField{},
	}
	// fmt.Printf(
	// 	"%s%v [shape=record, label=\"chan %s\", color=\"gray\"];\n",
	// 	strings.Repeat("	", indentLevel),
	// 	typeId,
	// 	chanType.String()
	// )
}

func addSliceToGraph(dg *dotGraphNode, obj types.Object, s *types.Slice, importedPkg string) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name())

	dg.typeNodes[typeId] = &dotGraphNode{
		typeId:           typeId,
		typeType:         "slice",
		typeName:         s.String(),
		typeNodes:        map[string]*dotGraphNode{},
		typeStructFields: map[string]*dotGraphStructField{},
	}

	// fmt.Printf(
	// 	"%s%v [shape=record, label=\"%s\", color=\"gray\"];\n",
	// 	strings.Repeat("	", indentLevel),
	// 	typeId,
	// 	s,
	// )
}

func addMapToGraph(dg *dotGraphNode, obj types.Object, m *types.Map, importedPkg string) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name())

	dg.typeNodes[typeId] = &dotGraphNode{
		typeId:           typeId,
		typeType:         "map",
		typeName:         m.String(),
		typeNodes:        map[string]*dotGraphNode{},
		typeStructFields: map[string]*dotGraphStructField{},
	}

	// // TODO: break down the map more and point each level to its type?
	// fmt.Printf(
	// 	"%s%v [shape=record, label=\"%s\", color=\"gray\"]\n",
	// 	strings.Repeat("	", indentLevel),
	// 	typeId,
	// 	m,
	// )
}

func addSignatureToGraph(dg *dotGraphNode, obj types.Object, s *types.Signature, importedPkg string) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name())
	typeString := obj.Type().String()
	// TODO: how can we escape in the label instead of removing {}?
	typeString = strings.Replace(strings.Replace(typeString, "{", "", -1), "}", "", -1)

	dg.typeNodes[typeId] = &dotGraphNode{
		typeId:           typeId,
		typeType:         "signature",
		typeName:         typeString,
		typeNodes:        map[string]*dotGraphNode{},
		typeStructFields: map[string]*dotGraphStructField{},
	}

	// fmt.Printf(
	// 	"%s%v [shape=record, label=\"%s\", color=\"gray\"]\n",
	// 	strings.Repeat("	", indentLevel),
	// 	typeId,
	// 	// TODO: how can we escape in the label instead of removing {}?
	// 	strings.Replace(strings.Replace(typeString, "{", "", -1), "}", "", -1),
	// )
}

func addPointerToGraph(dg *dotGraphNode, obj types.Object, p *types.Pointer, importedPkg string, posn token.Position) { //, indentLevel int) {
	// TODO finish? make sure it looks like a pointer
	// fmt.Printf("%s%v [shape=record, label=\"pointer\", color=\"gray\"]\n", strings.Repeat("	", indentLevel), typeId)
	// dg.typeNodes[typeId] = &dotGraphNode{
	//	typeId: typeId,
	// 	typeType: "pointer",
	// 	typeName: p.String(),
	//  typeNodes: map[string]*dotGraphNode{},
	// }
}

func addStructToGraph(dg *dotGraphNode, obj types.Object, ss *types.Struct, importedPkg string, posn token.Position) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name())
	// sName := obj.Name()

	// TODO: bring back file/line no strings?
	// pathAry := strings.Split(posn.String(), importedPkg)
	// var fileAndPosn string
	// // TODO: make file location/column an optional flag
	// if false {
	// 	fileAndPosn = pathAry[len(pathAry)-1]
	// } else {
	// 	fileAndPosn = ""
	// }

	node := dotGraphNode{
		typeId:           typeId,
		typeType:         "struct",
		typeName:         obj.Name(),
		typeNodes:        map[string]*dotGraphNode{},
		typeStructFields: map[string]*dotGraphStructField{},
	}

	for i := 0; i < ss.NumFields(); i++ {
		field := ss.Field(i)

		fieldTypeId := getTypeId(field.Type(), field.Pkg().Name())
		node.typeStructFields[fieldTypeId] = &dotGraphStructField{
			structFieldName:     field.Name(),
			structFieldTypeName: escapeHtml(field.Type().String()),
		}
	}

	dg.typeNodes[typeId] = &node
}

func addStructLinksToGraph(dg *dotGraphNode, obj types.Object, ss *types.Struct, importedPkg string, posn token.Position) { // , indentLevel int
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
			link := fmt.Sprintf("%s:port_%s -> %s;\n", structTypeId, fieldId, toTypeId)
			dg.typeLinks = append(dg.typeLinks, link)
		}
	}
}

func addInterfaceToGraph(dg *dotGraphNode, obj types.Object, i *types.Interface, importedPkg string, posn token.Position) { // indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name())

	node := dotGraphNode{
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
