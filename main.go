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

	if strings.Contains(tagContent, ",") {
		splitContent := strings.Split(tagContent, ",")
		return splitContent[0], splitContent[1]
	}
	return tagContent, ""
}

func newDefinition(t ast.Expr, comment string) (*Definition, []TypeReference) {
	def := &Definition{}
	externalTypeRefs := []TypeReference{}

	def.Description = comment

	switch tt := t.(type) {
	case *ast.Ident:
		typeName := tt.Name
		if isSimpleType(typeName) {
			def.Type = jsonifyType(typeName)
		} else {
			def.Ref = getDefLink(typeName)
		}
	case *ast.ArrayType:
		arrayType := tt
		var typeNameOfArray string
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
			packageAlias := selectorType.X.(*ast.Ident).Name
			typeName := selectorType.Sel.Name
			typeNameOfArray = typeName
			externalTypeRefs = append(externalTypeRefs, TypeReference{typeName, packageAlias})
		}

		def.Items = new(Definition)
		if isSimpleType(typeNameOfArray) {
			def.Items.Type = jsonifyType(typeNameOfArray)
		} else {
			def.Items.Ref = getDefLink(typeNameOfArray)
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
				def.AdditionalProperties.Ref = getDefLink(valueType.Name)
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

		def.Ref = getDefLink(typeName)
		externalTypeRefs = append(externalTypeRefs, TypeReference{typeName, packageAlias})
	case *ast.StarExpr:
		starExpr := tt
		switch starExpr.X.(type) {
		case *ast.Ident:
			starType := starExpr.X.(*ast.Ident)
			typeName := starType.Name

			if isSimpleType(typeName) {
				def.Type = jsonifyType(typeName)
			} else {
				def.Ref = getDefLink(typeName)
			}
		case *ast.SelectorExpr:
			selectorType := starExpr.X.(*ast.SelectorExpr)
			packageAlias := selectorType.X.(*ast.Ident).Name
			typeName := selectorType.Sel.Name

			externalTypeRefs = append(externalTypeRefs, TypeReference{typeName, packageAlias})
		}
	case *ast.StructType:
		structType := tt
		inlineDefinitions := []*Definition{}
		for _, field := range structType.Fields.List {
			yamlName, option := extractFromTag(field.Tag)
			if option == "inline" {
				var typeName string
				switch field.Type.(type) {
				case *ast.Ident:
					typeName = field.Type.(*ast.Ident).String()
				case *ast.StarExpr:
					typeName = field.Type.(*ast.StarExpr).X.(*ast.Ident).String()
				case *ast.SelectorExpr:
					selectorType := field.Type.(*ast.SelectorExpr)
					packageAlias := selectorType.X.(*ast.Ident).Name
					typeName = selectorType.Sel.Name
					externalTypeRefs = append(externalTypeRefs, TypeReference{typeName, packageAlias})
				}
				inlinedDef := new(Definition)
				inlinedDef.Ref = getDefLink(typeName)
				inlineDefinitions = append(inlineDefinitions, inlinedDef)
				def.AnyOf = append(def.AnyOf, &Definition{Ref: getDefLink(typeName)})
				continue
			}

			if yamlName == "" {
				continue
			}

			if option == "required" {
				def.Required = append(def.Required, yamlName)
			}

			if def.Properties == nil {
				def.Properties = make(map[string]*Definition)
			}

			propDef, propExternalTypeDefs := newDefinition(field.Type, field.Doc.Text())
			externalTypeRefs = append(externalTypeRefs, propExternalTypeDefs...)
			def.Properties[yamlName] = propDef
		}
		if len(inlineDefinitions) != 0 {
			childDef := def
			parentDef := new(Definition)
			parentDef.AllOf = inlineDefinitions

			if len(childDef.Properties) != 0 {
				parentDef.AllOf = append(inlineDefinitions, childDef)
			}
			def = parentDef
		}
	}

	return def, externalTypeRefs
}

func getReachableTypes(startingTypes map[string]bool, definitions Definitions) map[string]bool {
	pruner := DefinitionPruner{definitions, startingTypes}
	prunedTypes := pruner.Prune(true)
	return prunedTypes
}

func parseTypesInFile(filePath string) (Definitions, ExternalReferences) {
	// Open the input go file and parse the Abstract Syntax Tree
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	definitions := make(Definitions)
	externalRefs := make(ExternalReferences)

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
			def, refTypes := newDefinition(typeSpec.Type, typeDescription)
			definitions[typeName] = def
			externalRefs[typeName] = refTypes
		}
	}

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

func parseTypesInPackage(pkgName string, referencedTypes map[string]bool, containsAllTypes bool) Definitions {
	fmt.Println("Fetching package ", pkgName)
	curPackage := Package{pkgName}
	curPackage.Fetch()

	pkgDefs := make(Definitions)
	pkgExternalTypes := make(ExternalReferences)

	listOfFiles := curPackage.ListFiles()
	for _, fileName := range listOfFiles {
		fmt.Println("Processing file ", fileName)
		fileDefs, fileExternalRefs := parseTypesInFile(fileName)
		mergeDefs(pkgDefs, fileDefs)
		mergeExternalRefs(pkgExternalTypes, fileExternalRefs)
	}

	allReachableTypes := getReachableTypes(referencedTypes, pkgDefs)
	for key := range pkgDefs {
		if _, exists := allReachableTypes[key]; !exists {
			delete(pkgDefs, key)
			delete(pkgExternalTypes, key)
		}
	}

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
	inputPath := flag.String("package-name", "", "Go package name")
	outputPath := flag.String("output-file", "", "Output schema json path")
	typeList := flag.String("types", "", "List of types")

	flag.Parse()

	if len(*inputPath) == 0 || len(*outputPath) == 0 {
		log.Panic("Both input path and output paths need to be set")
	}

	schema := Schema{
		Definition:  &Definition{},
		Definitions: make(map[string]*Definition)}
	schema.Type = "object"
	startingPoint := strings.Split(*typeList, ",")
	startingPointMap := make(map[string]bool)
	for i := range startingPoint {
		startingPointMap[startingPoint[i]] = true
	}
	schema.Definitions = parseTypesInPackage(*inputPath, startingPointMap, true)
	schema.Version = ""
	schema.AnyOf = []*Definition{}

	for _, typeName := range startingPoint {
		schema.AnyOf = append(schema.AnyOf, &Definition{Ref: getDefLink(typeName)})
	}

	checkDefinitions(schema.Definitions, startingPointMap)

	flattenSchema(&schema)

	out, err := os.Create(*outputPath)
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
