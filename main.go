// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/ghodss/yaml"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

const defPrefix = "#/definitions/"

// Checks whether the typeName represents a simple json type

// Removes a character by replacing it with a space
func removeChar(str string, removedStr string) string {
	return strings.Replace(str, removedStr, " ", -1)
}

// This is a hacky function that does the one job of
// extracting the tag values in the structs
// Example struct:
// type MyType struct {
//   MyField string `yaml:"myField,omitempty"`
// }
//
// From the above example struct, we need to extract
// and return this: ("myField", "omitempty")
func extractFromTag(tag *ast.BasicLit) (string, string) {
	if tag == nil || tag.Value == "" {
		return "", ""
	}
	tagValue := tag.Value

	tagValue = removeChar(tagValue, "`")
	tagValue = removeChar(tagValue, `"`)
	tagValue = strings.TrimSpace(tagValue)

	var tagContent, tagKey string
	fmt.Sscanf(tagValue, `%s %s`, &tagKey, &tagContent)

	if tagKey != "json:" && tagKey != "yaml:" {
		return "", ""
	}

	if strings.Contains(tagContent, ",") {
		splitContent := strings.Split(tagContent, ",")
		return splitContent[0], splitContent[1]
	}
	return tagContent, ""
}

// exprToSchema converts ast.Expr to JSONSchemaProps
func (f *file) exprToSchema(t ast.Expr, doc string, comments []*ast.CommentGroup) (*v1beta1.JSONSchemaProps, []TypeReference) {
	var def *v1beta1.JSONSchemaProps
	var externalTypeRefs []TypeReference

	switch tt := t.(type) {
	case *ast.Ident:
		def = f.identToSchema(tt, doc, comments)
	case *ast.ArrayType:
		def, externalTypeRefs = f.arrayTypeToSchema(tt, doc, comments)
	case *ast.MapType:
		def = f.mapTypeToSchema(tt, doc, comments)
	case *ast.SelectorExpr:
		def, externalTypeRefs = f.selectorExprToSchema(tt, doc, comments)
	case *ast.StarExpr:
		def, externalTypeRefs = f.exprToSchema(tt.X, "", comments)
	case *ast.StructType:
		def, externalTypeRefs = f.structTypeToSchema(tt, comments)
	case *ast.InterfaceType: // TODO: handle interface if necessary.
		return &v1beta1.JSONSchemaProps{}, []TypeReference{}
	}
	def.Description = filterDescription(doc)

	return def, externalTypeRefs
}

// identToSchema converts ast.Ident to JSONSchemaProps.
func (f *file) identToSchema(ident *ast.Ident, doc string, comments []*ast.CommentGroup) *v1beta1.JSONSchemaProps {
	def := &v1beta1.JSONSchemaProps{}
	if isSimpleType(ident.Name) {
		def.Type = jsonifyType(ident.Name)
	} else {
		def.Ref = getPrefixedDefLink(ident.Name, f.pkgPrefix)
	}
	processMarkersInComments(def, comments...)
	return def
}

// identToSchema converts ast.SelectorExpr to JSONSchemaProps.
func (f *file) selectorExprToSchema(selectorType *ast.SelectorExpr, doc string, comments []*ast.CommentGroup) (*v1beta1.JSONSchemaProps, []TypeReference) {
	pkgAlias := selectorType.X.(*ast.Ident).Name
	typeName := selectorType.Sel.Name

	typ := TypeReference{
		TypeName:    typeName,
		PackageName: f.importPaths[pkgAlias],
	}

	time := TypeReference{TypeName: "Time", PackageName: "k8s.io/apimachinery/pkg/apis/meta/v1"}
	duration := TypeReference{TypeName: "Duration", PackageName: "k8s.io/apimachinery/pkg/apis/meta/v1"}
	quantity := TypeReference{TypeName: "Quantity", PackageName: "k8s.io/apimachinery/pkg/api/resource"}
	unstructured := TypeReference{TypeName: "Unstructured", PackageName: "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"}
	rawExtension := TypeReference{TypeName: "RawExtension", PackageName: "k8s.io/apimachinery/pkg/runtime"}
	intOrString := TypeReference{TypeName: "IntOrString", PackageName: "k8s.io/apimachinery/pkg/util/intstr"}

	switch typ {
	case time:
		return &v1beta1.JSONSchemaProps{
			Type:   "string",
			Format: "date-time",
		}, []TypeReference{}
	case duration:
		return &v1beta1.JSONSchemaProps{
			Type: "string",
		}, []TypeReference{}
	case quantity:
		return &v1beta1.JSONSchemaProps{
			Type: "string",
		}, []TypeReference{}
	case unstructured, rawExtension:
		return &v1beta1.JSONSchemaProps{
			Type: "object",
		}, []TypeReference{}
	case intOrString:
		return &v1beta1.JSONSchemaProps{
			AnyOf: []v1beta1.JSONSchemaProps{
				{
					Type: "string",
				},
				{
					Type: "integer",
				},
			},
		}, []TypeReference{}
	}

	def := &v1beta1.JSONSchemaProps{
		Ref: getPrefixedDefLink(typeName, f.importPaths[pkgAlias]),
	}
	processMarkersInComments(def, comments...)
	return def, []TypeReference{{TypeName: typeName, PackageName: pkgAlias}}
}

// arrayTypeToSchema converts ast.ArrayType to JSONSchemaProps by examining the elements in the array.
func (f *file) arrayTypeToSchema(arrayType *ast.ArrayType, doc string, comments []*ast.CommentGroup) (*v1beta1.JSONSchemaProps, []TypeReference) {
	// not passing doc down to exprToSchema
	items, extRefs := f.exprToSchema(arrayType.Elt, "", comments)
	processMarkersInComments(items, comments...)

	def := &v1beta1.JSONSchemaProps{
		Type:        "array",
		Items:       &v1beta1.JSONSchemaPropsOrArray{Schema: items},
		Description: doc,
	}

	// TODO: clear the schema on the parent level, since it is on the children level.

	return def, extRefs
}

// mapTypeToSchema converts ast.MapType to JSONSchemaProps.
func (f *file) mapTypeToSchema(mapType *ast.MapType, doc string, comments []*ast.CommentGroup) *v1beta1.JSONSchemaProps {
	def := &v1beta1.JSONSchemaProps{}
	switch mapType.Value.(type) {
	case *ast.Ident:
		valueType := mapType.Value.(*ast.Ident)
		if def.AdditionalProperties == nil {
			def.AdditionalProperties = &v1beta1.JSONSchemaPropsOrBool{}
		}
		def.AdditionalProperties.Schema = new(v1beta1.JSONSchemaProps)

		if isSimpleType(valueType.Name) {
			def.AdditionalProperties.Schema.Type = valueType.Name
		} else {
			def.AdditionalProperties.Schema.Ref = getPrefixedDefLink(valueType.Name, f.pkgPrefix)
		}
	case *ast.InterfaceType:
		// No op
		panic("Map Interface Type")
	}
	def.Type = "object"
	def.Description = doc
	processMarkersInComments(def, comments...)
	return def
}

// structTypeToSchema converts ast.StructType to JSONSchemaProps by examining each field in the struct.
func (f *file) structTypeToSchema(structType *ast.StructType, comments []*ast.CommentGroup) (*v1beta1.JSONSchemaProps, []TypeReference) {
	def := &v1beta1.JSONSchemaProps{}
	externalTypeRefs := []TypeReference{}
	for _, field := range structType.Fields.List {
		yamlName, option := extractFromTag(field.Tag)

		if (yamlName == "" && option != "inline") || yamlName == "-" {
			continue
		}

		if option != "inline" && option != "omitempty" {
			def.Required = append(def.Required, yamlName)
		}

		if def.Properties == nil {
			def.Properties = make(map[string]v1beta1.JSONSchemaProps)
		}

		propDef, propExternalTypeDefs := f.exprToSchema(field.Type, field.Doc.Text(), f.commentMap[field])

		externalTypeRefs = append(externalTypeRefs, propExternalTypeDefs...)

		if option == "inline" {
			def.AllOf = append(def.AllOf, *propDef)
			continue
		}

		def.Properties[yamlName] = *propDef
	}

	return def, externalTypeRefs
}

func getReachableTypes(startingTypes map[string]bool, definitions v1beta1.JSONSchemaDefinitions) map[string]bool {
	pruner := DefinitionPruner{definitions, startingTypes}
	prunedTypes := pruner.Prune(true)
	return prunedTypes
}

type file struct {
	// name prefix of the package
	pkgPrefix string
	// importPaths contains a map from import alias to the import path for the file.
	importPaths map[string]string
	// commentMap is comment mapping for this file.
	commentMap ast.CommentMap
}

func parseTypesInFile(filePath string, curPkgPrefix string) (v1beta1.JSONSchemaDefinitions, ExternalReferences) {
	// Open the input go file and parse the Abstract Syntax Tree
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	definitions := make(v1beta1.JSONSchemaDefinitions)
	externalRefs := make(ExternalReferences)

	// Parse import statements to get "alias: pkgName" mapping
	importPaths := make(map[string]string)
	for _, importItem := range node.Imports {
		pathValue := strings.Trim(importItem.Path.Value, "\"")
		if importItem.Name != nil {
			// Process aliased import
			importPaths[importItem.Name.Name] = pathValue
		} else if strings.Contains(pathValue, "/") {
			// Process unnamed imports with "/"
			segments := strings.Split(pathValue, "/")
			importPaths[segments[len(segments)-1]] = pathValue
		} else {
			importPaths[pathValue] = pathValue
		}
	}

	// Create an ast.CommentMap from the ast.File's comments.
	// This helps keeping the association between comments and AST nodes.
	// TODO: if necessary, support our own rules of comments ownership, golang's
	// builtin rules are listed at https://golang.org/pkg/go/ast/#NewCommentMap.
	// It seems it can meet our need at the moment.
	cmap := ast.NewCommentMap(fset, node, node.Comments)

	f := &file{
		pkgPrefix:   curPkgPrefix,
		importPaths: importPaths,
		commentMap:  cmap,
	}

	for _, i := range node.Decls {
		declaration, ok := i.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range declaration.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			typeName := typeSpec.Name.Name
			typeDescription := declaration.Doc.Text()

			fmt.Println("Generating schema definition for type:", typeName)
			def, refTypes := f.exprToSchema(typeSpec.Type, typeDescription, []*ast.CommentGroup{})
			definitions[getFullName(typeName, curPkgPrefix)] = *def
			externalRefs[getFullName(typeName, curPkgPrefix)] = refTypes
		}
	}

	// Overwrite import aliases with actual package names
	for typeName := range externalRefs {
		for i, ref := range externalRefs[typeName] {
			externalRefs[typeName][i].PackageName = importPaths[ref.PackageName]
		}
	}

	return definitions, externalRefs
}

func parseTypesInPackage(pkgName string, referencedTypes map[string]bool, rootPackage bool) v1beta1.JSONSchemaDefinitions {
	fmt.Println("Fetching package ", pkgName)
	debugPrint(referencedTypes)
	curPackage := Package{pkgName}
	curPackage.Fetch()

	pkgDefs := make(v1beta1.JSONSchemaDefinitions)
	pkgExternalTypes := make(ExternalReferences)

	listOfFiles := curPackage.ListFiles()
	pkgPrefix := strings.Replace(pkgName, "/", ".", -1)
	if rootPackage {
		pkgPrefix = ""
	}
	fmt.Println("pkgPrefix=", pkgPrefix)
	for _, fileName := range listOfFiles {
		fmt.Println("Processing file ", fileName)
		fileDefs, fileExternalRefs := parseTypesInFile(fileName, pkgPrefix)
		mergeDefs(pkgDefs, fileDefs)
		mergeExternalRefs(pkgExternalTypes, fileExternalRefs)
	}

	// Add pkg prefix to referencedTypes
	newReferencedTypes := make(map[string]bool)
	for key := range referencedTypes {
		altKey := getFullName(key, pkgPrefix)
		newReferencedTypes[altKey] = referencedTypes[key]
	}
	referencedTypes = newReferencedTypes

	fmt.Println("referencedTypes")
	debugPrint(referencedTypes)

	allReachableTypes := getReachableTypes(referencedTypes, pkgDefs)
	for key := range pkgDefs {
		if _, exists := allReachableTypes[key]; !exists {
			delete(pkgDefs, key)
			delete(pkgExternalTypes, key)
		}
	}
	fmt.Println("allReachableTypes")
	debugPrint(allReachableTypes)
	fmt.Println("pkgDefs")
	debugPrint(pkgDefs)
	fmt.Println("pkgExternalTypes")
	debugPrint(pkgExternalTypes)

	uniquePkgTypeRefs := make(map[string]map[string]bool)
	for _, item := range pkgExternalTypes {
		for _, typeRef := range item {
			if _, ok := uniquePkgTypeRefs[typeRef.PackageName]; !ok {
				uniquePkgTypeRefs[typeRef.PackageName] = make(map[string]bool)
			}
			uniquePkgTypeRefs[typeRef.PackageName][typeRef.TypeName] = true
		}
	}

	for childPkgName := range uniquePkgTypeRefs {
		childTypes := uniquePkgTypeRefs[childPkgName]
		childDefs := parseTypesInPackage(childPkgName, childTypes, false)
		mergeDefs(pkgDefs, childDefs)
	}

	return pkgDefs
}

func main() {
	op := &Options{}

	flag.StringVar(&op.InputPackage, "package-name", "", "Go package name")
	flag.StringVar(&op.OutputPath, "output-file", "", "Output schema json path")
	// TODO: use cobra StringSlice https://godoc.org/github.com/spf13/pflag#StringSlice
	typeList := flag.String("types", "", "List of types")
	flag.BoolVar(&op.Flatten, "flatten schema", false, "If flatten the schema using ref tag")
	flag.StringVar(&op.OutputFormat, "output-format", "json", "Output format of the schema")

	flag.Parse()

	op.Types = strings.Split(*typeList, ",")

	op.Generate()
}

type Options struct {
	// InputPackage is the path of the input package that contains source files.
	InputPackage string
	// OutputPath is the path that the schema will be written to.
	OutputPath string
	// Types is a list of target types.
	Types []string
	// Flatten contains if we use a flattened structure or a embedded structure.
	Flatten bool
	// OutputFormat should be either json or yaml. Default to json
	OutputFormat string
}

func (op *Options) Generate() {
	if len(op.InputPackage) == 0 || len(op.OutputPath) == 0 {
		log.Panic("Both input path and output paths need to be set")
	}

	schema := v1beta1.JSONSchemaProps{Definitions: make(map[string]v1beta1.JSONSchemaProps)}
	schema.Type = "object"
	startingPointMap := make(map[string]bool)
	for i := range op.Types {
		startingPointMap[op.Types[i]] = true
	}
	schema.Definitions = parseTypesInPackage(op.InputPackage, startingPointMap, true)
	schema.AnyOf = []v1beta1.JSONSchemaProps{}

	for _, typeName := range op.Types {
		schema.AnyOf = append(schema.AnyOf, v1beta1.JSONSchemaProps{Ref: getDefLink(typeName)})
	}

	// flattenAllOf only flattens allOf tags
	flattenAllOf(&schema)

	reachableTypes := getReachableTypes(startingPointMap, schema.Definitions)
	for key := range schema.Definitions {
		if _, exists := reachableTypes[key]; !exists {
			delete(schema.Definitions, key)
		}
	}

	checkDefinitions(schema.Definitions, startingPointMap)

	if !op.Flatten {
		schema.Definitions = embedSchema(schema.Definitions, startingPointMap)

		newDefs := v1beta1.JSONSchemaDefinitions{}
		for name := range startingPointMap {
			newDefs[name] = schema.Definitions[name]
		}
		schema.Definitions = newDefs
	}

	out, err := os.Create(op.OutputPath)
	if err != nil {
		log.Panic(err)
	}

	switch strings.ToLower(op.OutputFormat) {
	// default to json
	case "json", "":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		err = enc.Encode(schema)
		if err2 := out.Close(); err == nil {
			err = err2
		}
		if err != nil {
			log.Panic(err)
		}
	case "yaml":
		m, err := yaml.Marshal(schema)
		if err != nil {
			log.Panic(err)
		}
		err = ioutil.WriteFile(op.OutputPath, m, 0644)
		if err != nil {
			log.Panic(err)
		}
	}
}
