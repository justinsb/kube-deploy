package main

import (
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"path/filepath"
	"strings"
)

type GoParser struct {
	pkg *Package // Package we are scanning.
}

// File holds a single parsed file and associated data.
type File struct {
	pkg      *Package  // Package to which this file belongs.
	file     *ast.File // Parsed AST.
			   // These fields are reset for each type being generated.
	typeName string    // Name of the constant type.
			   //values   []Value // Accumulator for constant values of that type.
}

type Package struct {
	dir      string
	name     string
	defs     map[*ast.Ident]types.Object
	files    []*File
	typesPkg *types.Package
}


// prefixDirectory prepends the directory name on the beginning of each name in the list.
func prefixDirectory(directory string, names []string) []string {
	if directory == "." {
		return names
	}
	ret := make([]string, len(names))
	for i, name := range names {
		ret[i] = filepath.Join(directory, name)
	}
	return ret
}

// parsePackageDir parses the package residing in the directory.
func (g *GoParser) parsePackageDir(directory string) {
	pkg, err := build.Default.ImportDir(directory, 0)
	if err != nil {
		log.Fatalf("cannot process directory %s: %s", directory, err)
	}
	var names []string
	names = append(names, pkg.GoFiles...)
	//names = append(names, pkg.CgoFiles...)
	//names = append(names, pkg.SFiles...)
	names = prefixDirectory(directory, names)
	g.parsePackage(directory, names, nil)
}


// parsePackage analyzes the single package constructed from the named files.
// If text is non-nil, it is a string to be used instead of the content of the file,
// to be used for testing. parsePackage exits if there is an error.
func (g *GoParser) parsePackage(directory string, names []string, text interface{}) {
	var files []*File
	var astFiles []*ast.File
	g.pkg = new(Package)
	fs := token.NewFileSet()
	for _, name := range names {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		parsedFile, err := parser.ParseFile(fs, name, text, 0)
		if err != nil {
			log.Fatalf("parsing package: %s: %s", name, err)
		}
		astFiles = append(astFiles, parsedFile)
		files = append(files, &File{
			file: parsedFile,
			pkg:  g.pkg,
		})
	}
	if len(astFiles) == 0 {
		log.Fatalf("%s: no buildable Go files", directory)
	}
	g.pkg.name = astFiles[0].Name.Name
	g.pkg.files = files
	g.pkg.dir = directory
	// Type check the package.
	g.pkg.check(fs, astFiles)
}

// check type-checks the package. The package must be OK to proceed.
func (pkg *Package) check(fs *token.FileSet, astFiles []*ast.File) {
	pkg.defs = make(map[*ast.Ident]types.Object)
	config := types.Config{Importer: importer.Default(), FakeImportC: true}
	info := &types.Info{
		Defs: pkg.defs,
	}
	typesPkg, err := config.Check(pkg.dir, fs, astFiles, info)
	if err != nil {
		log.Fatalf("checking package: %s", err)
	}
	pkg.typesPkg = typesPkg
}


//func loadFile(inputPath string) (string, []GeneratedType) {
//	fset := token.NewFileSet()
//	f, err := parser.ParseFile(fset, inputPath, nil, parser.ParseComments)
//	if err != nil {
//		log.Fatalf("Could not parse file: %s", err)
//	}
//
//	packageName := identifyPackage(f)
//	if packageName == "" {
//		log.Fatalf("Could not determine package name of %s", inputPath)
//	}
//
//	joiners := map[string]bool{}
//	stringers := map[string]bool{}
//	for _, decl := range f.Decls {
//		typeName, ok := identifyJoinerType(decl)
//		if ok {
//			joiners[typeName] = true
//			continue
//		}
//
//		typeName, ok = identifyStringer(decl)
//		if ok {
//			stringers[typeName] = true
//			continue
//		}
//	}
//
//	types := []GeneratedType{}
//	for typeName, _ := range joiners {
//		_, isStringer := stringers[typeName]
//		joiner := GeneratedType{typeName, isStringer}
//		types = append(types, joiner)
//	}
//
//	return packageName, types
//}
//
//func identifyPackage(f *ast.File) string {
//	if f.Name == nil {
//		return ""
//	}
//	return f.Name.Name
//}
//
//func identifyJoinerType(decl ast.Decl) (typeName string, match bool) {
//	genDecl, ok := decl.(*ast.GenDecl)
//	if !ok {
//		return
//	}
//	if genDecl.Doc == nil {
//		return
//	}
//
//	found := false
//	for _, comment := range genDecl.Doc.List {
//		if strings.Contains(comment.Text, "@joiner") {
//			found = true
//			break
//		}
//	}
//	if !found {
//		return
//	}
//
//	for _, spec := range genDecl.Specs {
//		if typeSpec, ok := spec.(*ast.TypeSpec); ok {
//			if typeSpec.Name != nil {
//				typeName = typeSpec.Name.Name
//				break
//			}
//		}
//	}
//	if typeName == "" {
//		return
//	}
//
//	match = true
//	return
//}
//
//func identifyStringer(decl ast.Decl) (typeName string, match bool) {
//	funcDecl, ok := decl.(*ast.FuncDecl)
//	if !ok {
//		return
//	}
//
//	// Method name should match fmt.Stringer
//	if funcDecl.Name == nil {
//		return
//	}
//	if funcDecl.Name.Name != "String" {
//		return
//	}
//
//	// Should have no arguments
//	if funcDecl.Type == nil {
//		return
//	}
//	if funcDecl.Type.Params == nil {
//		return
//	}
//	if len(funcDecl.Type.Params.List) != 0 {
//		return
//	}
//
//	// Return value should be a string
//	if funcDecl.Type.Results == nil {
//		return
//	}
//	if len(funcDecl.Type.Results.List) != 1 {
//		return
//	}
//	result := funcDecl.Type.Results.List[0]
//	if result.Type == nil {
//		return
//	}
//	if ident, ok := result.Type.(*ast.Ident); !ok {
//		return
//	} else if ident.Name != "string" {
//		return
//	}
//
//	// Receiver type
//	if funcDecl.Recv == nil {
//		return
//	}
//	if len(funcDecl.Recv.List) != 1 {
//		return
//	}
//	recv := funcDecl.Recv.List[0]
//	if recv.Type == nil {
//		return
//	}
//	if ident, ok := recv.Type.(*ast.Ident); !ok {
//		return
//	} else {
//		typeName = ident.Name
//	}
//
//	match = true
//	return
//}


