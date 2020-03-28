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

type structField struct {
	structFieldId       string
	structFieldTypeName string
}

// A named type that was parsed, and will be represented in the graph.
type graphNode struct {
	pkgName              string
	typeId               string
	typeType             string
	typeUnderlyingType   string // e.g. for Basic underlying types, containers, etc
	typeName             string
	typeMapType          string
	typeNodes            map[string]*graphNode   // id -> node
	typeStructFields     map[string]*structField // name -> node (of field type)
	typeInterfaceMethods map[string]string       // name -> type
}

// A reference (e.g. arrow) from one type to another.
type graphNodeLink struct {
	fromStructTypeId    string
	fromStructFieldName string
	toTypePkgName       string
	toTypeName          string
}

// "pkg1" => {
//   subPkgs: {
//     "subpkg1" => { subPkgs: ..., nodes: { "node" => ... }}
//   },
//   nodes: { "node" => ... },
//   nodeLinks: { fromStructTypeId: "typeA", toTypeName: "typeB" }
// }
type pkg struct {
	pkgName     string
	rootPkgName string
	subPkgs     map[string]*pkg
	nodes       map[string]*graphNode
	nodeLinks   []graphNodeLink
}

func (p *pkg) Print(str string, pkgName string, indentLevel int, typeIdsPrinted map[string]bool) (string, map[string]bool) {
	for _, node := range (*p).nodes {
		str, typeIdsPrinted = node.Print(str, pkgName, indentLevel+1, typeIdsPrinted)
	}
	for subPkgName, subPkg := range (*p).subPkgs {
		if len(subPkgName) > 0 {
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
		} else {
			str, typeIdsPrinted = subPkg.Print(str, "FIXME", indentLevel, typeIdsPrinted)
		}
	}

	return str, typeIdsPrinted
}

func (p *pkg) PrintHeader() string {
	out := fmt.Sprintf("digraph V {\n"+
		"  graph [label=< <br/><b>%s</b> >, labelloc=b, fontsize=10 fontname=Arial];\n"+
		"  node [fontname=Arial];\n"+
		"  edge [fontname=Arial];\n",
		p.pkgName,
	)
	return out
}

func (p *pkg) PrintFooter(out string) string {
	return fmt.Sprintf("%s}\n", out)
}

func (p *pkg) PrintNodeLinks(out string, typeIdsPrinted map[string]bool) string {
	for _, nodeLink := range p.nodeLinks {
		toTypeId := labelizeName(nodeLink.toTypePkgName, nodeLink.toTypeName)
		out = fmt.Sprintf(
			"%s  %s:port_%s -> %s;\n",
			out,
			nodeLink.fromStructTypeId,
			nodeLink.fromStructFieldName,
			toTypeId,
		)
		// Render any referenced types that were not output (e.g. external packages)
		if _, ok := typeIdsPrinted[toTypeId]; !ok {
			out = fmt.Sprintf("%s  %s [shape=plaintext label=<"+
				"<table border='2' cellborder='0' cellspacing='0' style='rounded' color='#cccccc'>"+
				"<tr><td align='center' colspan='2'>%s.%s</td></tr>"+
				"</table> >];\n",
				out,
				toTypeId,
				nodeLink.toTypePkgName,
				nodeLink.toTypeName,
			)
		}
	}
	return out
}

// WriteGraph will build the graph based on the given pkgName, and write out the dot graph.
func WriteGraph(pkgName string) string {
	typeIdsPrinted := map[string]bool{}
	pkgGraph := BuildGraph(pkgName)

	out := pkgGraph.PrintHeader()
	out, typeIdsPrinted = pkgGraph.Print(out, pkgName, 0, typeIdsPrinted)
	out = pkgGraph.PrintNodeLinks(out, typeIdsPrinted)
	out = pkgGraph.PrintFooter(out)

	return out
}

func (dgn *graphNode) Print(out string, pkgName string, indentLevel int, typeIdsPrinted map[string]bool) (string, map[string]bool) {
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
			"<tr><td bgcolor='#e0ebf5' align='center' colspan='2'>%s</td></tr>",
			out,
			strings.Repeat("  ", indentLevel),
			dgn.typeId,
			dgn.typeName,
		)
		for methodName, methodType := range dgn.typeInterfaceMethods {
			out = fmt.Sprintf("%s<tr><td align='left'>%s</td><td align='left'><font color='#7f8183'>%s</font></td></tr>", out, methodName, methodType)
		}
		out = fmt.Sprintf("%s</table>>];\n", out)
	case "pointer":
		out = fmt.Sprintf(
			"%s\n%s%v [shape=record, label=\"pointer\", color=\"#CCC\"]\n",
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
			"%s\n%s%v [shape=record, label=\"chan %s\", color=\"#CCC\"];\n",
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

// BuildGraph builds a graph of types in the given pkgName.
func BuildGraph(pkgName string) *pkg {
	root := graphNode{
		pkgName:              pkgName,
		typeId:               "root",
		typeType:             "root",
		typeName:             pkgName,
		typeNodes:            map[string]*graphNode{},
		typeStructFields:     map[string]*structField{},
		typeInterfaceMethods: map[string]string{},
	}

	pkgGraph := pkg{
		pkgName:     pkgName,
		rootPkgName: pkgName,
		subPkgs:     map[string]*pkg{},
		nodeLinks:   []graphNodeLink{},
	}

	recursivelyBuildGraph(&root, pkgName, pkgName, &pkgGraph)

	return &pkgGraph
}

func recursivelyBuildGraph(dg *graphNode, rootPkgName, pkgName string, p *pkg) {
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
	addTypesToGraph(dg, normalizedPkgName, fset, files, p)

	for _, pkgName := range listData.Imports {
		if strings.HasPrefix(pkgName, listData.ImportPath) {
			recursivelyBuildGraph(dg, rootPkgName, pkgName, p)
		}
	}
}

func listGoFilesInPackage(pkg string) goListResult {
	var listCmdOut []byte
	var err error

	// TODO check if pkg exists first?
	cmd := exec.Command("go", "list", "-json", pkg)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=1")
	if listCmdOut, err = cmd.CombinedOutput(); err != nil {
		fmt.Printf("Error running '%v'\n", cmd.String())
		fmt.Printf("Debug: %s\n", string(listCmdOut))
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var data goListResult
	if err := json.Unmarshal(listCmdOut, &data); err != nil {
		fmt.Printf("Error finding %v\n", pkg)
		panic(err)
	}

	return data
}

func addTypesToGraph(dg *graphNode, pkgName string, fset *token.FileSet, files []*ast.File, p *pkg) {
	// Type-check the package. Setup the maps that Check will fill.
	info := types.Info{
		Defs: make(map[*ast.Ident]types.Object),
	}

	var conf types.Config = types.Config{
		Importer:                 importer.For("source", nil),
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
			addTypeToGraph(dg, obj, pkgName, p)
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
	} else if len(pkgName) == 0 {
		label = typeName
	} else {
		label = strings.Join([]string{pkgName, typeName}, "_")
	}

	return strings.ToLower(label)
}

func addTypeToGraph(node *graphNode, obj types.Object, pkgName string, p *pkg) {
	// Only print named types
	if reflect.TypeOf(obj.Type()).String() != "*types.Named" {
		return
	}

	switch namedTypeType := obj.Type().Underlying().(type) {
	case *types.Basic:
		addBasicToGraph(node, obj, namedTypeType, pkgName, p)
	case *types.Interface:
		addInterfaceToGraph(node, obj, namedTypeType, pkgName, p)
	case *types.Pointer:
		addPointerToGraph(node, obj, namedTypeType, pkgName, p)
	case *types.Signature:
		addSignatureToGraph(node, obj, namedTypeType, pkgName, p)
	case *types.Chan:
		addChanToGraph(node, obj, namedTypeType, pkgName, p)
	case *types.Slice:
		addSliceToGraph(node, obj, namedTypeType, pkgName, p)
	case *types.Map:
		addMapToGraph(node, obj, namedTypeType, pkgName, p)
	case *types.Struct:
		addStructToGraph(node, obj, namedTypeType, pkgName, p)
	default:
		fmt.Printf(
			"    // Unknown: %v <%T> - %v <%T>\n",
			obj, obj,
			namedTypeType, namedTypeType,
		)
	}
}

func addBasicToGraph(dg *graphNode, obj types.Object, b *types.Basic, pkgName string, p *pkg) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	// TODO: check key first
	node := &graphNode{
		pkgName:              pkgName,
		typeId:               typeId,
		typeType:             "basic",
		typeName:             obj.Type().String(),
		typeUnderlyingType:   b.String(),
		typeNodes:            map[string]*graphNode{},
		typeStructFields:     map[string]*structField{},
		typeInterfaceMethods: map[string]string{},
	}

	deepSetNodeOnSubPkg(p, node, pkgName)
}

func addChanToGraph(dg *graphNode, obj types.Object, c *types.Chan, pkgName string, p *pkg) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	node := &graphNode{
		pkgName:              pkgName,
		typeId:               typeId,
		typeType:             "chan",
		typeName:             c.Elem().String(),
		typeNodes:            map[string]*graphNode{},
		typeStructFields:     map[string]*structField{},
		typeInterfaceMethods: map[string]string{},
	}
	deepSetNodeOnSubPkg(p, node, pkgName)
	dg.typeNodes[typeId] = node
}

func addSliceToGraph(dg *graphNode, obj types.Object, s *types.Slice, pkgName string, p *pkg) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	node := &graphNode{
		pkgName:              pkgName,
		typeId:               typeId,
		typeType:             "slice",
		typeUnderlyingType:   s.String(),
		typeName:             obj.Name(),
		typeNodes:            map[string]*graphNode{},
		typeStructFields:     map[string]*structField{},
		typeInterfaceMethods: map[string]string{},
	}
	deepSetNodeOnSubPkg(p, node, pkgName)
	dg.typeNodes[typeId] = node
}

func addMapToGraph(dg *graphNode, obj types.Object, m *types.Map, pkgName string, p *pkg) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	// fmt.Printf("Adding map to graph: %v, %v, %v\n", pkgName, typeId, m.String())
	// fmt.Printf("                   : %v, %T, %v, %v, %v\n", obj, obj, obj.Name(), m.String(), m.Elem())
	// fmt.Printf("Adding map %s, %s\n", typeId, m)
	node := &graphNode{
		pkgName:              pkgName,
		typeId:               typeId,
		typeType:             "map",
		typeName:             obj.Name(),
		typeNodes:            map[string]*graphNode{},
		typeMapType:          m.String(),
		typeStructFields:     map[string]*structField{},
		typeInterfaceMethods: map[string]string{},
	}
	deepSetNodeOnSubPkg(p, node, pkgName)
	dg.typeNodes[typeId] = node
}

func addSignatureToGraph(dg *graphNode, obj types.Object, s *types.Signature, pkgName string, p *pkg) { //, indentLevel int) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)
	typeString := obj.Type().String()
	// TODO: how can we escape in the label instead of removing {}?
	typeString = strings.Replace(strings.Replace(typeString, "{", "", -1), "}", "", -1)

	node := &graphNode{
		pkgName:              pkgName,
		typeId:               typeId,
		typeType:             "signature",
		typeName:             typeString,
		typeNodes:            map[string]*graphNode{},
		typeStructFields:     map[string]*structField{},
		typeInterfaceMethods: map[string]string{},
	}
	deepSetNodeOnSubPkg(p, node, pkgName)
	dg.typeNodes[typeId] = node
}

func addPointerToGraph(dg *graphNode, obj types.Object, pointer *types.Pointer, pkgName string, p *pkg) { //, indentLevel int) {
	// TODO finish? make sure it looks like a pointer
	// dg.typeNodes[typeId] = &graphNode{
	// pkgName:            pkgName,
	//	typeId: typeId,
	// 	typeType: "pointer",
	// 	typeName: p.String(),
	//  typeNodes: map[string]*graphNode{},
	// }
}

func addStructToGraph(dg *graphNode, obj types.Object, ss *types.Struct, pkgName string, p *pkg) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	node := &graphNode{
		pkgName:              pkgName,
		typeId:               typeId,
		typeType:             "struct",
		typeName:             obj.Name(),
		typeNodes:            map[string]*graphNode{},
		typeStructFields:     map[string]*structField{},
		typeInterfaceMethods: map[string]string{},
	}

	for i := 0; i < ss.NumFields(); i++ {
		f := ss.Field(i)
		fieldPkgName := f.Pkg().Name()
		fieldTypeId := labelizeName(fieldPkgName, f.Type().String()) // TODO: this might break when the type of a struct field is from a different package
		fieldTypeName := stripPkgPrefix(stripPointer(f.Type().String()), fieldPkgName)

		node.typeStructFields[f.Name()] = &structField{
			structFieldId:       fieldTypeId,
			structFieldTypeName: fieldTypeName,
		}
		// TODO can we recreate the field here as a node, so we can set it in this map?
		// (*p)[fieldPkgName][escapeHtml(field.Type().String())] = node
	}

	dg.typeNodes[typeId] = node
	deepSetNodeOnSubPkg(p, node, pkgName)
	addStructLinksToGraph(p, obj, ss, pkgName)
}

//
func deepSetNodeOnSubPkg(p *pkg, node *graphNode, pkgName string) {
	currentp := p
	// If this is a node in the root package namespace, pkgName could be blank, so traverse the full package name in those cases.
	// if len(pkgName) == 0 {
	// 	pkgName = p.rootPkgName
	// }
	for _, currentPart := range strings.Split(pkgName, "/") {
		if (*currentp).subPkgs[currentPart] == nil {
			(*currentp).subPkgs[currentPart] = &pkg{
				pkgName:     currentPart,
				rootPkgName: p.rootPkgName,
				subPkgs:     map[string]*pkg{},
				nodes:       map[string]*graphNode{},
				nodeLinks:   []graphNodeLink{},
			}
		}
		currentp = (*currentp).subPkgs[currentPart]
	}
	currentp.nodes[node.typeName] = node
}

func stripPointer(typeName string) string {
	return strings.TrimPrefix(typeName, "*")
}

func stripPkgPrefix(typeName, pkgName string) string {
	return strings.TrimPrefix(strings.TrimPrefix(typeName, pkgName), "/")
}

func addStructLinksToGraph(p *pkg, obj types.Object, ss *types.Struct, pkgName string) {
	structTypeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	// TODO: move this into the printTypeLinks() func?
	for i := 0; i < ss.NumFields(); i++ {
		f := ss.Field(i)
		fieldId := getTypeId(f.Type(), f.Pkg().Name(), pkgName)
		fTypeType := reflect.TypeOf(f.Type()).String()

		// HACK: This is the only way I know to get the typeId when the pkgname
		// is a fully-qualified package, which doesn't really work with getTypeId() :shruggie:
		strippedType := stripPkgPrefix(stripPointer(f.Type().String()), p.rootPkgName)
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

		// fmt.Printf(
		// 	"debug: adding struct field link: %v, %v, %v, %v, %v, %v\n",
		// 	f.Name(),
		// 	pkgName,
		// 	toTypePkgName,
		// 	toTypeTypeName,
		// 	fieldId,
		// 	fTypeType,
		// )

		// Don't link to basic types or containers of basic types.
		isSignature := fTypeType == "*types.Signature"
		isBasic := fTypeType == "*types.Basic"
		// HACK: better way to do this, e.g. checking NumExplicitMethods > 0?
		isEmptyInterface := fieldId == "time_interfacebraces"
		isContainerOfBasic := containerElemIsBasic(f.Type())

		if !isEmptyInterface && !isSignature && !isBasic && !isContainerOfBasic {
			p.nodeLinks = append(p.nodeLinks, graphNodeLink{
				fromStructTypeId:    structTypeId,
				fromStructFieldName: f.Name(),
				toTypePkgName:       toTypePkgName,
				toTypeName:          toTypeTypeName,
			})
		}
	}
}

func addInterfaceToGraph(dg *graphNode, obj types.Object, i *types.Interface, pkgName string, p *pkg) {
	typeId := getTypeId(obj.Type(), obj.Pkg().Name(), pkgName)

	methods := map[string]string{}
	if i.NumMethods() > 0 {
		for idx := 0; idx < i.NumMethods(); idx += 1 {
			m := i.Method(idx)
			methods[m.Name()] = m.Type().String()
		}
	}
	node := &graphNode{
		pkgName:              pkgName,
		typeId:               typeId,
		typeType:             "interface",
		typeName:             obj.Name(),
		typeNodes:            map[string]*graphNode{},
		typeStructFields:     map[string]*structField{},
		typeInterfaceMethods: methods,
	}

	dg.typeNodes[typeId] = node
	deepSetNodeOnSubPkg(p, node, pkgName)
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
}

func getTypeId(t types.Type, typePkgName, originalPkgName string) string {
	var typeId, typeName string

	switch namedTypeType := t.Underlying().(type) {
	case *types.Basic:
		typeName = t.String()
	case *types.Chan:
		typeName = t.String()
	case *types.Slice:
		typeName = t.String()
	case *types.Struct:
		typeName = t.String()
	case *types.Interface:
		typeName = t.String()
		typeId = labelizeName(typePkgName, typeName)
	case *types.Pointer:
		pointerType := namedTypeType.Elem()
		typeName = pointerType.String()
	case *types.Signature:
		typeName = t.String()
	case *types.Map:
		typeName = t.String()
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
