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
	structFieldId       string
	structFieldTypeName string
}

type dotGraphNode struct {
	pkgName              string
	typeId               string
	typeType             string
	typeUnderlyingType   string // e.g. for Basic underlying types, containers, etc
	typeName             string
	typeMapType          string
	typeNodes            map[string]*dotGraphNode        // id -> node
	typeStructFields     map[string]*dotGraphStructField // name -> node (of field type)
	typeInterfaceMethods map[string]string               // name -> type
}

// "pkg1" => {
//   subPkgs: {
//     "subpkg1" => { subPkgs: ..., nodes: { "node" => ... }}
//   },
//   nodes: { "node" => ... }
// }
type dotGraphPkg struct {
	rootPkgName string
	subPkgs     map[string]*dotGraphPkg
	nodes       map[string]*dotGraphNode
	nodeLinks   []nodeLink
}

type nodeLink struct {
	fromStructTypeId    string
	fromStructFieldName string
	toTypePkgName       string
	toTypeName          string
}

// { linkedlist -> SELF }
type dotGraphPkgByPkg map[string]*dotGraphPkg

func (byPkg *dotGraphPkg) Print(str string, pkgName string, indentLevel int, typeIdsPrinted map[string]bool) (string, map[string]bool) {
	for _, node := range (*byPkg).nodes {
		str, typeIdsPrinted = node.Print(str, pkgName, indentLevel+1, typeIdsPrinted)
	}
	for subPkgName, subPkg := range (*byPkg).subPkgs {
		str = fmt.Sprintf(
			"%s%ssubgraph cluster_%v { \n",
			str,
			strings.Repeat("  ", indentLevel+1),
			subPkgName,
		)
		str, typeIdsPrinted = subPkg.Print(str, "FIXME", indentLevel+1, typeIdsPrinted)

		// subgraph config
		str = fmt.Sprintf("%s%snode [style=filled];\n", str, strings.Repeat("  ", indentLevel+2))
		str = fmt.Sprintf("%s%slabel=\"%s\";\n", str, strings.Repeat("  ", indentLevel+2), relativizeTypePkgName(subPkgName, pkgName))
		str = fmt.Sprintf("%s%sgraph[style=dotted color=\"#7f8183\"];\n", str, strings.Repeat("  ", indentLevel+2))

		str = fmt.Sprintf("%s%s}\n", str, strings.Repeat("  ", indentLevel+1))
	}

	return str, typeIdsPrinted
}

func WriteGraph(pkgName string) string {
	typeIdsPrinted := map[string]bool{}

	_, byPkg := BuildGraph(pkgName)

	str := fmt.Sprintf("digraph V {\n"+
		"  graph [label=< <br/><b>%s</b> >, labelloc=b, fontsize=10 fontname=Arial];\n"+
		"  node [fontname=Arial];\n"+
		"  edge [fontname=Arial];\n",
		pkgName,
	)
	str, typeIdsPrinted = byPkg.Print(str, pkgName, 0, typeIdsPrinted)
	for _, nodeLink := range byPkg.nodeLinks {
		toTypeId := labelizeName(nodeLink.toTypePkgName, nodeLink.toTypeName)
		str = fmt.Sprintf(
			"%s  %s:port_%s -> %s;\n",
			str,
			nodeLink.fromStructTypeId,
			nodeLink.fromStructFieldName,
			toTypeId,
		)
		// Check if we actually printed the toTypeId node, so we can render it otherwise
		if _, ok := typeIdsPrinted[toTypeId]; !ok {
			str = fmt.Sprintf(
				"%s  %s [shape=record, label=\"%v.%v\", color=\"gray\"]\n",
				str,
				toTypeId,
				nodeLink.toTypePkgName,
				nodeLink.toTypeName,
			)
		}
	}
	str = fmt.Sprintf("%s}\n", str)

	return str
}

func (dgn *dotGraphNode) Print(out string, pkgName string, indentLevel int, typeIdsPrinted map[string]bool) (string, map[string]bool) {
	switch dgn.typeType {
	case "root":
		// no-op?
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
			out = fmt.Sprintf(
				"%s<tr><td port='port_%s' align='left'>%s</td><td align='left'><font color='#7f8183'>%s</font></td></tr>",
				out,
				structFieldName,
				structFieldName,
				escapeHtml(relativizeTypePkgName(structFieldNode.structFieldTypeName, pkgName)),
			)
		}
		out = fmt.Sprintf("%s</table> >];\n", out)
		typeIdsPrinted[dgn.typeId] = true
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
		typeIdsPrinted[dgn.typeId] = true
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
			"%s\n%s%v [shape=record, label=\"%s\", color=\"blue\"]\n",
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
			dgn.typeName, // TODO: should this be typeId?
			dgn.typeUnderlyingType,
		)
	case "slice":
		out = fmt.Sprintf("%s%s%v [shape=plaintext label=< "+
			"<table border='2' cellborder='0' cellspacing='0' style='rounded' color='#4BAAD3'>"+
			"<tr><td bgcolor='#e0ebf5' align='center'>%s</td></tr><tr><td>%s</td></tr>"+
			"</table> >];\n",
			out,
			strings.Repeat("  ", indentLevel),
			dgn.typeId,
			dgn.typeName,
			dgn.typeUnderlyingType,
		)
	case "map":
		// TODO: break down the map more and point each level to its type?
		out = fmt.Sprintf("%s%s%v [shape=plaintext label=< "+
			"<table border='2' cellborder='0' cellspacing='0' style='rounded' color='#4BAAD3'>"+
			"<tr><td bgcolor='#e0ebf5' align='center'>%s</td></tr><tr><td>%s</td></tr>"+
			"</table> >];\n",
			out,
			strings.Repeat("  ", indentLevel),
			dgn.typeId, // TODO: should this be typeId?
			dgn.typeName,
			dgn.typeMapType,
		)
	default:
		panic(dgn.typeType)
	}
	typeIdsPrinted[dgn.typeId] = true

	return out, typeIdsPrinted
}

func BuildGraph(pkgName string) (*dotGraphNode, *dotGraphPkg) {
	root := dotGraphNode{
		pkgName:              pkgName,
		typeId:               "root",
		typeType:             "root",
		typeName:             pkgName,
		typeNodes:            map[string]*dotGraphNode{},
		typeStructFields:     map[string]*dotGraphStructField{},
		typeInterfaceMethods: map[string]string{},
	}

	dgnByPkg := dotGraphPkg{
		rootPkgName: pkgName,
		subPkgs:     map[string]*dotGraphPkg{},
		nodeLinks:   []nodeLink{},
	}

	recursivelyBuildGraph(&root, pkgName, pkgName, &dgnByPkg)

	return &root, &dgnByPkg
}

func recursivelyBuildGraph(dg *dotGraphNode, rootPkgName, pkgName string, byPkg *dotGraphPkg) {
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
	addTypesToGraph(dg, normalizedPkgName, fset, files, byPkg)

	for _, pkgName := range listData.Imports {
		if strings.HasPrefix(pkgName, listData.ImportPath) {
			recursivelyBuildGraph(dg, rootPkgName, pkgName, byPkg) // indentLevel
		}
	}
}

func listGoFilesInPackage(pkg string) goListResult {
	var listCmdOut []byte
	var err error

	// TODO check if pkg exists first?
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
func addTypesToGraph(dg *dotGraphNode, pkgName string, fset *token.FileSet, files []*ast.File, byPkg *dotGraphPkg) {
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
	for _, obj := range info.Defs {
		if _, ok := obj.(*types.TypeName); ok {
			// NB to get the position of the type: fset.Position(id.Pos())
			addTypeToGraph(dg, obj, pkgName, byPkg)
		}
	}
}

func escapeName(name string) string {
	name = strings.Replace(name, "*", "", -1) // remove pointers, handle them separately by returning bool
	name = strings.Replace(name, "/", "_SLASH_", -1)
	name = strings.Replace(name, "[]", "_ARY_", -1)
	name = strings.Replace(name, "{}", "_BRACES_", -1)
	name = strings.Replace(name, ",", "_COMMA_", -1)
	name = strings.Replace(name, "(", "_LPARENS_", -1)
	name = strings.Replace(name, ")", "_RPARENS_", -1)
	name = strings.Replace(name, " ", "", -1)
	return name
}

// Turn a type string into a graphviz-friendly label, e.g. `func(interface{}, uintptr)` -> funclparensinterfacebracescommauintptrrparens
func labelizeName(pkgName, typeName string) string {
	pkgName = escapeName(pkgName)
	typeName = escapeName(typeName)

	var label string
	// If the type is from another package, don't prepend this package's name to it
	if strings.Contains(typeName, ".") {
		// TODO: handle cases when it's in another package
		label = strings.Replace(typeName, ".", "_DOT_", -1)
	} else {
		label = strings.Join([]string{pkgName, typeName}, "_")
	}

	return strings.ToLower(label)
}

func addTypeToGraph(dg *dotGraphNode, obj types.Object, pkgName string, byPkg *dotGraphPkg) { // , indentLevel int
	// Only print named types
	if reflect.TypeOf(obj.Type()).String() != "*types.Named" {
		return
	}

	switch namedTypeType := obj.Type().Underlying().(type) {
	case *types.Basic:
		addBasicToGraph(dg, obj, namedTypeType, pkgName, byPkg)
	case *types.Interface:
		addInterfaceToGraph(dg, obj, namedTypeType, pkgName, byPkg)
	case *types.Pointer:
		addPointerToGraph(dg, obj, namedTypeType, pkgName, byPkg)
	case *types.Signature:
		addSignatureToGraph(dg, obj, namedTypeType, pkgName, byPkg)
	case *types.Chan:
		addChanToGraph(dg, obj, namedTypeType, pkgName, byPkg)
	case *types.Slice:
		addSliceToGraph(dg, obj, namedTypeType, pkgName, byPkg)
	case *types.Map:
		addMapToGraph(dg, obj, namedTypeType, pkgName, byPkg)
	case *types.Struct:
		addStructToGraph(dg, obj, namedTypeType, pkgName, byPkg)
	default:
		fmt.Printf(
			"    // Unknown: %v <%T> - %v <%T>\n",
			obj, obj,
			namedTypeType, namedTypeType,
		)
	}
}

func addBasicToGraph(dg *dotGraphNode, obj types.Object, b *types.Basic, pkgName string, byPkg *dotGraphPkg) { // , indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	// TODO: check key first
	node := &dotGraphNode{
		pkgName:              pkgName,
		typeId:               typeId,
		typeType:             "basic",
		typeName:             obj.Type().String(),
		typeUnderlyingType:   b.String(),
		typeNodes:            map[string]*dotGraphNode{},
		typeStructFields:     map[string]*dotGraphStructField{},
		typeInterfaceMethods: map[string]string{},
	}

	currentByPkg := byPkg
	for _, currentPart := range strings.Split(pkgName, "/") {
		// subPkgs map[string]*dotGraphPkg
		// nodes   map[string]*dotGraphNode
		if (*byPkg).subPkgs[currentPart] == nil {
			(*byPkg).subPkgs[currentPart] = &dotGraphPkg{
				rootPkgName: byPkg.rootPkgName,
				subPkgs:     map[string]*dotGraphPkg{},
				nodes:       map[string]*dotGraphNode{},
				nodeLinks:   []nodeLink{},
			}
		}
		currentByPkg = (*byPkg).subPkgs[currentPart]
	}
	currentByPkg.nodes[node.typeName] = node
	addNodeToByPkg(byPkg, node, pkgName, typeId)
}

func addChanToGraph(dg *dotGraphNode, obj types.Object, c *types.Chan, pkgName string, byPkg *dotGraphPkg) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	node := &dotGraphNode{
		pkgName:              pkgName,
		typeId:               typeId,
		typeType:             "chan",
		typeName:             c.Elem().String(),
		typeNodes:            map[string]*dotGraphNode{},
		typeStructFields:     map[string]*dotGraphStructField{},
		typeInterfaceMethods: map[string]string{},
	}
	addNodeToByPkg(byPkg, node, pkgName, typeId)
	dg.typeNodes[typeId] = node
}

func addSliceToGraph(dg *dotGraphNode, obj types.Object, s *types.Slice, pkgName string, byPkg *dotGraphPkg) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	node := &dotGraphNode{
		pkgName:              pkgName,
		typeId:               typeId,
		typeType:             "slice",
		typeUnderlyingType:   s.String(),
		typeName:             obj.Name(),
		typeNodes:            map[string]*dotGraphNode{},
		typeStructFields:     map[string]*dotGraphStructField{},
		typeInterfaceMethods: map[string]string{},
	}
	addNodeToByPkg(byPkg, node, pkgName, typeId)
	dg.typeNodes[typeId] = node
}

func addMapToGraph(dg *dotGraphNode, obj types.Object, m *types.Map, pkgName string, byPkg *dotGraphPkg) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	// fmt.Printf("Adding map to graph: %v, %v, %v\n", pkgName, typeId, m.String())
	// fmt.Printf("                   : %v, %T, %v, %v, %v\n", obj, obj, obj.Name(), m.String(), m.Elem())
	// fmt.Printf("Adding map %s, %s\n", typeId, m)
	node := &dotGraphNode{
		pkgName:              pkgName,
		typeId:               typeId,
		typeType:             "map",
		typeName:             obj.Name(),
		typeNodes:            map[string]*dotGraphNode{},
		typeMapType:          m.String(),
		typeStructFields:     map[string]*dotGraphStructField{},
		typeInterfaceMethods: map[string]string{},
	}
	addNodeToByPkg(byPkg, node, pkgName, typeId)
	dg.typeNodes[typeId] = node
}

func addSignatureToGraph(dg *dotGraphNode, obj types.Object, s *types.Signature, pkgName string, byPkg *dotGraphPkg) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)
	typeString := obj.Type().String()
	// TODO: how can we escape in the label instead of removing {}?
	typeString = strings.Replace(strings.Replace(typeString, "{", "", -1), "}", "", -1)

	node := &dotGraphNode{
		pkgName:              pkgName,
		typeId:               typeId,
		typeType:             "signature",
		typeName:             typeString,
		typeNodes:            map[string]*dotGraphNode{},
		typeStructFields:     map[string]*dotGraphStructField{},
		typeInterfaceMethods: map[string]string{},
	}
	addNodeToByPkg(byPkg, node, pkgName, typeId)
	dg.typeNodes[typeId] = node
}

func addPointerToGraph(dg *dotGraphNode, obj types.Object, p *types.Pointer, pkgName string, byPkg *dotGraphPkg) { //, indentLevel int) {
	// TODO finish? make sure it looks like a pointer
	// dg.typeNodes[typeId] = &dotGraphNode{
	// pkgName:            pkgName,
	//	typeId: typeId,
	// 	typeType: "pointer",
	// 	typeName: p.String(),
	//  typeNodes: map[string]*dotGraphNode{},
	// }
}

func addStructToGraph(dg *dotGraphNode, obj types.Object, ss *types.Struct, pkgName string, byPkg *dotGraphPkg) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)
	// sName := obj.Name()

	node := &dotGraphNode{
		pkgName:              pkgName,
		typeId:               typeId,
		typeType:             "struct",
		typeName:             obj.Name(),
		typeNodes:            map[string]*dotGraphNode{},
		typeStructFields:     map[string]*dotGraphStructField{},
		typeInterfaceMethods: map[string]string{},
	}

	for i := 0; i < ss.NumFields(); i++ {
		f := ss.Field(i)
		fieldPkgName := f.Pkg().Name()
		// fmt.Printf("debug:  (%v) adding struct field %v from %v\n", pkgName, field.Name(), fieldPkgName)
		fieldTypeId := getTypeId(f.Type(), fieldPkgName, pkgName)
		fieldTypeName := stripPkgPrefix(stripPointer(f.Type().String()), byPkg.rootPkgName)

		node.typeStructFields[f.Name()] = &dotGraphStructField{
			structFieldId:       fieldTypeId,
			structFieldTypeName: fieldTypeName,
		}
		// TODO can we recreate the field here as a node, so we can set it in this map?
		// (*byPkg)[fieldPkgName][escapeHtml(field.Type().String())] = node
	}

	dg.typeNodes[typeId] = node
	addNodeToByPkg(byPkg, node, pkgName, typeId)
	addStructLinksToGraph(byPkg, obj, ss, pkgName)
}

func addNodeToByPkg(byPkg *dotGraphPkg, node *dotGraphNode, pkgName, typeId string) {
	for _, currentPart := range strings.Split(pkgName, "/") {
		if (*byPkg).subPkgs[currentPart] == nil {
			(*byPkg).subPkgs[currentPart] = &dotGraphPkg{
				rootPkgName: byPkg.rootPkgName,
				subPkgs:     map[string]*dotGraphPkg{},
				nodes:       map[string]*dotGraphNode{},
				nodeLinks:   []nodeLink{},
			}
		}
		byPkg = (*byPkg).subPkgs[currentPart]
	}
	byPkg.nodes[node.typeName] = node
}

func stripPointer(typeName string) string {
	return strings.TrimPrefix(typeName, "*")
}

func stripPkgPrefix(typeName, pkgName string) string {
	return strings.TrimPrefix(strings.TrimPrefix(typeName, pkgName), "/")
}

func addStructLinksToGraph(byPkg *dotGraphPkg, obj types.Object, ss *types.Struct, pkgName string) {
	structTypeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	// TODO: move this into the printTypeLinks() func?
	for i := 0; i < ss.NumFields(); i++ {
		f := ss.Field(i)
		fieldId := getTypeId(f.Type(), f.Pkg().Name(), pkgName)
		fTypeType := reflect.TypeOf(f.Type()).String()

		// HACK: This is the only way I know to get the typeId when the pkgname
		// is a fully-qualified package, which doesn't really work with getTypeId() :shruggie:
		strippedType := stripPkgPrefix(stripPointer(f.Type().String()), byPkg.rootPkgName)
		pkgName := pkgName
		typeName := strippedType
		if strings.Contains(strippedType, ".") {
			split := strings.Split(strippedType, ".")
			pkgName = split[0]
			typeName = split[1]
		}
		toTypePkgName := pkgName
		toTypeTypeName := typeName

		// Link to underlying type instead of slice-of-underlying type
		if containerType := getContainerType(f.Type()); containerType != nil {
			// TODO: pkgName may be wrong here, it could be another package. How to fix?
			toTypeTypeName = containerType.String()
		}

		// Don't link to basic types or containers of basic types.
		isSignature := fTypeType == "*types.Signature"
		isBasic := fTypeType == "*types.Basic"
		// HACK: better way to do this, e.g. chedcking NumExplicitMethods > 0?
		isEmptyInterface := fieldId == "time_interfacebraces"
		isContainerOfBasic := containerElemIsBasic(f.Type())

		if !isEmptyInterface && !isSignature && !isBasic && !isContainerOfBasic {
			byPkg.nodeLinks = append(byPkg.nodeLinks, nodeLink{
				fromStructTypeId:    structTypeId,
				fromStructFieldName: f.Name(),
				toTypePkgName:       toTypePkgName,
				toTypeName:          toTypeTypeName,
			})
		}
	}
}

func addInterfaceToGraph(dg *dotGraphNode, obj types.Object, i *types.Interface, pkgName string, byPkg *dotGraphPkg) { // indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	node := &dotGraphNode{
		pkgName:              pkgName,
		typeId:               typeId,
		typeType:             "interface",
		typeName:             obj.Name(),
		typeNodes:            map[string]*dotGraphNode{},
		typeStructFields:     map[string]*dotGraphStructField{},
		typeInterfaceMethods: map[string]string{},
	}

	if i.NumExplicitMethods() > 0 {
		for idx := 0; idx < i.NumExplicitMethods(); idx += 1 {
			m := i.ExplicitMethod(idx)
			dg.typeInterfaceMethods[m.Name()] = m.Type().String()
		}
	}

	dg.typeNodes[typeId] = node
	addNodeToByPkg(byPkg, node, pkgName, typeId)
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
		// sliceType := namedTypeType.Elem()
		// fmt.Printf("DEBUG: adding slice %s <%T>, <%T> %s, <%T>\n", t.String(), t, namedTypeType, sliceType.String(), sliceType)
		typeName = strings.Join([]string{"slice", t.String()}, "_")
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
		typeName = strings.Join([]string{"map", mapType.String()}, "_")
	}

	typeId = labelizeName(originalPkgName, typeName)

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
