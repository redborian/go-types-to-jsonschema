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

	var yamlTagContent string
	fmt.Sscanf(tagValue, `json: %s`, &yamlTagContent)

	if strings.Contains(yamlTagContent, ",") {
		splitContent := strings.Split(yamlTagContent, ",")
		return splitContent[0], splitContent[1]
	}
	return yamlTagContent, ""
}

// Gets the schema definition link of a resource
func getDefLink(resourceName string) string {
	return defPrefix + resourceName
}

// func parseIdentifier(identifier *ast.Ident) (*Definition, []TypeReference) {
// 	definition := new(Definition)
// 	externalTypeRefs := []TypeReference{}
// 	switch identifier
// }

// Parses array type and returns its corresponding
// schema definition.
func parseArrayType(arrayType *ast.ArrayType) (*Definition, []TypeReference) {
	definition := new(Definition)
	// typeNameOfArray := getTypeNameOfArray(arrayType)
	var typeNameOfArray string
	externalTypeRefs := []TypeReference{}
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

	definition.Items = new(Definition)
	if isSimpleType(typeNameOfArray) {
		definition.Items.Type = jsonifyType(typeNameOfArray)
	} else {
		definition.Items.Ref = getDefLink(typeNameOfArray)
	}
	definition.Type = "array"

	return definition, externalTypeRefs
}

// Gets the resource name from definitions url.
// Eg, returns 'TypeName' from '#/definitions/TypeName'
func getNameFromURL(url string) string {
	slice := strings.Split(url, "/")
	return slice[len(slice)-1]
}

// Parses a struct type and returns its corresponding
// schema definition.
func parseStructType(structType *ast.StructType, structTypeName string, typeDescription string) (*Definition, []TypeReference) {
	definition := &Definition{}
	definition.Description = typeDescription
	definition.Properties = make(map[string]*Definition)
	definition.Required = []string{}
	inlineDefinitions := []*Definition{}
	// externalTypes := make(ExternalReferences)
	externalTypeRefs := []TypeReference{}

	for _, field := range structType.Fields.List {
		property := new(Definition)
		yamlFieldName, option := extractFromTag(field.Tag)

		if yamlFieldName == "" && option == "" {
			continue
		}

		// If the 'inline' option is enabled, we need to merge
		// the type with its parent definition. We do it with
		// 'anyOf' json schema property.
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
			// if structTypeName == "Revision" {
			// 	fmt.Print("REVISION ")
			// 	fmt.Println(reflect.TypeOf(field.Type))
			// }
			inlinedDef := new(Definition)
			inlinedDef.Ref = getDefLink(typeName)
			inlineDefinitions = append(inlineDefinitions, inlinedDef)
			continue
		}
		// if 'omitempty' is not present, then the field is required
		if option != "omitempty" {
			definition.Required = append(definition.Required, yamlFieldName)
		}

		switch field.Type.(type) {
		case *ast.Ident:
			typeName := field.Type.(*ast.Ident).String()
			if isSimpleType(typeName) {
				property.Type = jsonifyType(typeName)
			} else {
				property.Ref = getDefLink(typeName)
			}
		case *ast.ArrayType:
			arrayType := field.Type.(*ast.ArrayType)
			var arrayExternalRef []TypeReference
			property, arrayExternalRef = parseArrayType(arrayType)
			if len(arrayExternalRef) == 1 {
				externalTypeRefs = append(externalTypeRefs, arrayExternalRef[0])
			}
		case *ast.MapType:
			mapType := field.Type.(*ast.MapType)
			switch mapType.Value.(type) {
			case *ast.Ident:
				valueType := mapType.Value.(*ast.Ident)
				property.AdditionalProperties = new(Definition)

				if isSimpleType(valueType.Name) {
					property.AdditionalProperties.Type = valueType.Name
				} else {
					property.AdditionalProperties.Ref = getDefLink(valueType.Name)
				}
			case *ast.InterfaceType:
				// No op
			}
			property.Type = "object"
		case *ast.SelectorExpr:
			selectorType := field.Type.(*ast.SelectorExpr)
			packageAlias := selectorType.X.(*ast.Ident).Name
			typeName := selectorType.Sel.Name

			property.Ref = getDefLink(typeName)
			// Collect external references
			// externalTypes[structTypeName] = append(externalTypes[structTypeName], TypeReference{typeName, packageAlias})
			externalTypeRefs = append(externalTypeRefs, TypeReference{typeName, packageAlias})
		case *ast.StarExpr:
			starExpr := field.Type.(*ast.StarExpr)
			// starType := starExpr.X.(*ast.Ident)
			// typeName := starType.Name

			// if isSimpleType(typeName) {
			// 	property.Type = jsonifyType(typeName)
			// } else {
			// 	property.Ref = getDefLink(typeName)
			// }
			switch starExpr.X.(type) {
			case *ast.Ident:
				starType := starExpr.X.(*ast.Ident)
				typeName := starType.Name

				if isSimpleType(typeName) {
					property.Type = jsonifyType(typeName)
				} else {
					property.Ref = getDefLink(typeName)
				}
			case *ast.SelectorExpr:
				selectorType := starExpr.X.(*ast.SelectorExpr)
				packageAlias := selectorType.X.(*ast.Ident).Name
				typeName := selectorType.Sel.Name

				externalTypeRefs = append(externalTypeRefs, TypeReference{typeName, packageAlias})
				// debugPrint(selectorType)
				// externalTypes[]
				// fmt.Printf("%+v\n", selectorType)
				// fmt.Printf("TODO")
				// panic("Stop Execution")
			}
		}
		// Set the common properties here as the cases might
		// overwrite 'property' pointer.
		property.Description = field.Doc.Text()

		definition.Properties[yamlFieldName] = property
	}

	if len(inlineDefinitions) == 0 {
		// if len(definition.Properties) == 0 {
		// 	return nil, nil
		// }
		// There are no inlined definitions
		return definition, externalTypeRefs
	}

	// There are inlined definitions; we need to set
	// the "anyOf" property of a new parent node and attach
	// the inline definitions, along with the currently
	// parsed definition
	parentDefinition := new(Definition)
	parentDefinition.AllOf = inlineDefinitions

	if len(definition.Properties) != 0 {
		parentDefinition.AllOf = append(inlineDefinitions, definition)
	}

	return parentDefinition, externalTypeRefs
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

			// Currently schema generation only takes Structs
			// and Array types into account.
			switch typeSpec.Type.(type) {
			case *ast.ArrayType:
				arrayType := typeSpec.Type.(*ast.ArrayType)
				parsedArrayDef, referencedTypes := parseArrayType(arrayType)

				definitions[typeName] = parsedArrayDef
				externalRefs[typeName] = referencedTypes
				// TODO
				// collectExternalTypeFromArray(externalTypes, arrayType)
			case *ast.StructType:
				structType := typeSpec.Type.(*ast.StructType)
				parsedStructDef, referencedTypes := parseStructType(structType, typeName, typeDescription)
				// mergeExternalRefs(externalRefs, referencedTypes)
				if parsedStructDef != nil && referencedTypes != nil {
					externalRefs[typeName] = referencedTypes
					definitions[typeName] = parsedStructDef
				}
			case *ast.Ident:
				identType := typeSpec.Type.(*ast.Ident)
				def := &Definition{}
				if isSimpleType(identType.Name) {
					def.Type = jsonifyType(identType.Name)
				} else {
					def.Ref = getDefLink(identType.Name)
				}
				def.Description = typeDescription
				definitions[typeName] = def
			}
			// if typeName == "UID" {
			// 	ident := typeSpec.Type.(*ast.Ident)
			// 	fmt.Println("REFLECTION", reflect.TypeOf(typeSpec.Type))
			// 	debugPrint(ident)
			// }
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

	// fmt.Print("ImportPaths: ")
	// debugPrint(importPaths)

	return definitions, externalRefs
}

func parseTypesInPackage(pkgName string, referencedTypes map[string]bool, containsAllTypes bool) Definitions {
	fmt.Println("Fetching package ", pkgName)
	curPackage := Package{pkgName}
	curPackage.Fetch()

	pkgDefs := make(Definitions)
	pkgExternalTypes := make(ExternalReferences)

	fmt.Println("referencedTypes: ")
	debugPrint(referencedTypes)

	listOfFiles := curPackage.ListFiles()
	for _, fileName := range listOfFiles {
		fmt.Println("Processing file ", fileName)
		fileDefs, fileExternalRefs := parseTypesInFile(fileName)
		mergeDefs(pkgDefs, fileDefs)
		mergeExternalRefs(pkgExternalTypes, fileExternalRefs)
	}

	// fmt.Print("AllDefs: ")
	// debugPrint(pkgDefs)

	var allReachableTypes map[string]bool
	// Prune unreferenced types
	// if !containsAllTypes {
	allReachableTypes = getReachableTypes(referencedTypes, pkgDefs)
	fmt.Print("allReachableTypes: ")
	debugPrint(allReachableTypes)
	for key := range pkgDefs {
		if _, exists := allReachableTypes[key]; !exists {
			delete(pkgDefs, key)
			delete(pkgExternalTypes, key)
		}
	}
	// }

	// fmt.Print("PrunedDefs: ")
	// debugPrint(pkgDefs)

	// Expand external references
	// Assume no cyclic references across files
	// fmt.Println("%+v", node.Imports)
	// debugPrint(node.Imports)
	// debugPrint(externalTypes)
	// debugPrint(importPaths)

	uniquePkgTypeRefs := make(map[string]map[string]bool)
	for _, item := range pkgExternalTypes {
		for _, typeRef := range item {
			if _, ok := uniquePkgTypeRefs[typeRef.PackageName]; !ok {
				uniquePkgTypeRefs[typeRef.PackageName] = make(map[string]bool)
			}
			uniquePkgTypeRefs[typeRef.PackageName][typeRef.TypeName] = true
		}
	}

	// TODO remove this condition
	// if containsAllTypes {
	for childPkgName := range uniquePkgTypeRefs {
		childTypes := uniquePkgTypeRefs[childPkgName]
		childDefs := parseTypesInPackage(childPkgName, childTypes, false)
		mergeDefs(pkgDefs, childDefs)
	}
	// }

	// fmt.Println("PackageDefs for package ", pkgName, ": ")
	// debugPrint(pkgDefs)

	return pkgDefs
}

// func getUniqueImports(typeSubset []string, importLookup map[string][]string, includeAll bool) map[string]bool {
// 	uniqueImports := make(map[string]bool)
// 	if !includeAll {
// 		for _, typeName := range typeSubset {
// 			imports := importLookup[typeName]
// 			for _, singleImport := range imports {
// 				uniqueImports[singleImport] = true
// 			}
// 		}
// 	} else {
// 		for _, referencedTypes := range importLookup {
// 			for _, singleImport := range referencedTypes {
// 				uniqueImports[singleImport] = true
// 			}
// 		}
// 	}
// 	return uniqueImports
// }

func main() {
	inputPath := flag.String("input-file", "", "Input go file path")
	outputPath := flag.String("output-file", "", "Output schema json path")
	// removeAllOfs := flag.Bool("remove-allof", false, "Flattens the json schema by removing \"allOf\"s")

	flag.Parse()

	if len(*inputPath) == 0 || len(*outputPath) == 0 {
		log.Panic("Both input path and output paths need to be set")
	}

	schema := Schema{
		Definition:  &Definition{},
		Definitions: make(map[string]*Definition)}
	schema.Type = "object"
	// fmt.Println("%+v", node.Imports)
	// b, err3 := json.Marshal(node.Imports)
	// if err3 != nil {
	// 	panic(err)
	// }
	// fmt.Println(string(b))
	startingPoint := []string{"Configuration", "Revision", "Route", "Service"}
	startingPointMap := make(map[string]bool)
	for i := range startingPoint {
		startingPointMap[startingPoint[i]] = true
	}
	schema.Definitions = parseTypesInPackage(*inputPath, startingPointMap, true)

	checkDefinitions(schema.Definitions, startingPointMap)

	// if *removeAllOfs {
	// 	fmt.Println("Flattening the schema by removing \"anyOf\" nodes")
	// flattenSchema(&schema)
	// }

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
