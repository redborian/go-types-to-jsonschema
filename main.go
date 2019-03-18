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
	"log"
	"os"
	"strings"
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
		// log.Panic("Tag value is empty")
		return "", ""
	}
	tagValue := tag.Value
	// fmt.Println("TagValue is ", tagValue)

	// return yamlFieldValue, yamlOptionValue
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

// Get Schema given ast.Expr
func newDefinition(t ast.Expr, comment string, importPaths map[string]string, curPkgPrefix string) (*Definition, []TypeReference) {
	def := &Definition{}
	externalTypeRefs := []TypeReference{}

	def.Description = comment

	switch tt := t.(type) {
	case *ast.Ident:
		typeName := tt.Name
		if isSimpleType(typeName) {
			def.Type = jsonifyType(typeName)
		} else {
			def.Ref = getPrefixedDefLink(typeName, curPkgPrefix)
		}
	case *ast.ArrayType:
		arrayType := tt
		var typeNameOfArray, packageAlias string
		switch arrayType.Elt.(type) {
		case *ast.Ident:
			identifier := arrayType.Elt.(*ast.Ident)
			typeNameOfArray = identifier.Name
		case *ast.StarExpr:
			starType := arrayType.Elt.(*ast.StarExpr)
			identifier := starType.X.(*ast.Ident)
			typeNameOfArray = identifier.Name
		case *ast.SelectorExpr:
			selectorType := arrayType.Elt.(*ast.SelectorExpr)
			packageAlias = selectorType.X.(*ast.Ident).Name
			typeName := selectorType.Sel.Name
			typeNameOfArray = typeName
			externalTypeRefs = append(externalTypeRefs, TypeReference{typeName, packageAlias})
		}

		def.Items = new(Definition)
		if isSimpleType(typeNameOfArray) {
			def.Items.Type = jsonifyType(typeNameOfArray)
		} else {
			if packageAlias != "" {
				def.Items.Ref = getPrefixedDefLink(typeNameOfArray, importPaths[packageAlias])
			} else {
				def.Items.Ref = getPrefixedDefLink(typeNameOfArray, curPkgPrefix)
			}
		}
		def.Type = "array"

	case *ast.MapType:
		mapType := tt
		switch mapType.Value.(type) {
		case *ast.Ident:
			valueType := mapType.Value.(*ast.Ident)
			def.AdditionalProperties = new(Definition)

			if isSimpleType(valueType.Name) {
				def.AdditionalProperties.Type = valueType.Name
			} else {
				def.AdditionalProperties.Ref = getPrefixedDefLink(valueType.Name, curPkgPrefix)
			}
		case *ast.InterfaceType:
			// No op
			panic("Map Interface Type")
		}
		def.Type = "object"
	case *ast.SelectorExpr:
		selectorType := tt
		packageAlias := selectorType.X.(*ast.Ident).Name
		typeName := selectorType.Sel.Name

		def.Ref = getPrefixedDefLink(typeName, importPaths[packageAlias])
		externalTypeRefs = append(externalTypeRefs, TypeReference{typeName, packageAlias})
	case *ast.StarExpr:
		starExpr := tt
		var typeName, packageAlias string
		switch starExpr.X.(type) {
		case *ast.Ident:
			starType := starExpr.X.(*ast.Ident)
			typeName = starType.Name

		case *ast.SelectorExpr:
			selectorType := starExpr.X.(*ast.SelectorExpr)
			packageAlias = selectorType.X.(*ast.Ident).Name
			typeName = selectorType.Sel.Name

			externalTypeRefs = append(externalTypeRefs, TypeReference{typeName, packageAlias})
		}
		if isSimpleType(typeName) {
			def.Type = jsonifyType(typeName)
		} else {
			if packageAlias != "" {
				def.Ref = getPrefixedDefLink(typeName, importPaths[packageAlias])
			} else {
				def.Ref = getPrefixedDefLink(typeName, curPkgPrefix)
			}
		}
	case *ast.StructType:
		structType := tt
		inlineDefinitions := []*Definition{}
		for _, field := range structType.Fields.List {
			yamlName, option := extractFromTag(field.Tag)
			var typeName, packageAlias string
			switch field.Type.(type) {
			case *ast.Ident:
				typeName = field.Type.(*ast.Ident).String()
			case *ast.StarExpr:
				// starType = field.Type.(*ast.StarExpr).X.(*ast.Ident).String()
				starType := field.Type.(*ast.StarExpr)
				switch starType.X.(type) {
				case *ast.Ident:
					typeName = starType.X.(*ast.Ident).Name
				case *ast.SelectorExpr:
					selectorType := starType.X.(*ast.SelectorExpr)
					packageAlias = selectorType.X.(*ast.Ident).Name
					typeName = selectorType.Sel.Name
					externalTypeRefs = append(externalTypeRefs, TypeReference{typeName, packageAlias})
				}
			case *ast.SelectorExpr:
				selectorType := field.Type.(*ast.SelectorExpr)
				// var packageAlias string
				// switch selectorType.X.(type) {
				// case *ast.Ident:
				packageAlias = selectorType.X.(*ast.Ident).Name
				// case *ast.SelectorExpr:
				// 	subSelectorType := selectorType.X.(*ast.SelectorExpr)
				// 	packageAlias = subSelectorType.X.(*ast.Ident).Name
				// }
				typeName = selectorType.Sel.Name
				externalTypeRefs = append(externalTypeRefs, TypeReference{typeName, packageAlias})
			}
			if option == "inline" {
				inlinedDef := new(Definition)
				if packageAlias != "" {
					inlinedDef.Ref = getPrefixedDefLink(typeName, importPaths[packageAlias])
				} else {
					inlinedDef.Ref = getPrefixedDefLink(typeName, curPkgPrefix)
				}
				inlineDefinitions = append(inlineDefinitions, inlinedDef)
				// def.AnyOf = append(def.AnyOf, &Definition{Ref: getPrefixedDefLink(typeName, curPkgPrefix)})
				continue
			}

			if yamlName == "" || yamlName == "-" {
				continue
			}
			// if yamlName == "-" {
			// 	debugPrint(def)
			// 	// debugPrint(structType)
			// 	panic("yamlName is -")
			// }

			if option == "required" {
				def.Required = append(def.Required, yamlName)
			}

			if def.Properties == nil {
				def.Properties = make(map[string]*Definition)
			}

			propDef, propExternalTypeDefs := newDefinition(field.Type, field.Doc.Text(), importPaths, curPkgPrefix)
			externalTypeRefs = append(externalTypeRefs, propExternalTypeDefs...)
			// if yamlName != "" && yamlName != "-" {
			def.Properties[yamlName] = propDef
			// } else {
			// 	def.Properties[typeName] = propDef
			// }
		}
		if len(inlineDefinitions) != 0 {
			childDef := def
			parentDef := new(Definition)
			parentDef.AllOf = inlineDefinitions

			if len(childDef.Properties) != 0 {
				parentDef.AllOf = append(inlineDefinitions, childDef)
			}
			def = parentDef
		} else {
			// if def.Type == "" && len(def.Properties) == 0 {
			// 	def.Type = "object"
			// }
		}
	}

	return def, externalTypeRefs
}

func getReachableTypes(startingTypes map[string]bool, definitions Definitions) map[string]bool {
	pruner := DefinitionPruner{definitions, startingTypes}
	prunedTypes := pruner.Prune(true)
	return prunedTypes
}

func parseTypesInFile(filePath string, curPkgPrefix string) (Definitions, ExternalReferences) {
	// Open the input go file and parse the Abstract Syntax Tree
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	definitions := make(Definitions)
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
			def, refTypes := newDefinition(typeSpec.Type, typeDescription, importPaths, curPkgPrefix)
			definitions[getFullName(typeName, curPkgPrefix)] = def
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

func parseTypesInPackage(pkgName string, referencedTypes map[string]bool, rootPackage bool) Definitions {
	fmt.Println("Fetching package ", pkgName)
	debugPrint(referencedTypes)
	curPackage := Package{pkgName}
	curPackage.Fetch()

	pkgDefs := make(Definitions)
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
}

func (op *Options) Generate() {
	if len(op.InputPackage) == 0 || len(op.OutputPath) == 0 {
		log.Panic("Both input path and output paths need to be set")
	}

	schema := Schema{
		Definition:  &Definition{},
		Definitions: make(map[string]*Definition)}
	schema.Type = "object"
	startingPointMap := make(map[string]bool)
	for i := range op.Types {
		startingPointMap[op.Types[i]] = true
	}
	schema.Definitions = parseTypesInPackage(op.InputPackage, startingPointMap, true)
	schema.Version = ""
	schema.AnyOf = []*Definition{}

	for _, typeName := range op.Types {
		schema.AnyOf = append(schema.AnyOf, &Definition{Ref: getDefLink(typeName)})
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
		embedSchema(schema.Definitions, startingPointMap)

		newDefs := Definitions{}
		for name := range startingPointMap {
			newDefs[name] = schema.Definitions[name]
		}
		schema.Definitions = newDefs
	}

	out, err := os.Create(op.OutputPath)
	if err != nil {
		log.Panic(err)
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	err = enc.Encode(schema)
	if err2 := out.Close(); err == nil {
		err = err2
	}
	if err != nil {
		log.Panic(err)
	}
}
