package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"strings"
)

// Schema is the root schema.
// RFC draft-wright-json-schema-00, section 4.5
type Schema struct {
	*Definition
	Definitions Definitions `json:"definitions,omitempty"`
}

// Definitions hold schema definitions.
// http://json-schema.org/latest/json-schema-validation.html#rfc.section.5.26
// RFC draft-wright-json-schema-validation-00, section 5.26
type Definitions map[string]*Definition

// Definition type represents a JSON Schema object type.
type Definition struct {
	// RFC draft-wright-json-schema-00
	Version string `json:"$schema,omitempty"` // section 6.1
	Ref     string `json:"$ref,omitempty"`    // section 7
	// RFC draft-wright-json-schema-validation-00, section 5
	MultipleOf           int                    `json:"multipleOf,omitempty"`           // section 5.1
	Maximum              int                    `json:"maximum,omitempty"`              // section 5.2
	ExclusiveMaximum     bool                   `json:"exclusiveMaximum,omitempty"`     // section 5.3
	Minimum              int                    `json:"minimum,omitempty"`              // section 5.4
	ExclusiveMinimum     bool                   `json:"exclusiveMinimum,omitempty"`     // section 5.5
	MaxLength            int                    `json:"maxLength,omitempty"`            // section 5.6
	MinLength            int                    `json:"minLength,omitempty"`            // section 5.7
	Pattern              string                 `json:"pattern,omitempty"`              // section 5.8
	AdditionalItems      *Definition            `json:"additionalItems,omitempty"`      // section 5.9
	Items                *Definition            `json:"items,omitempty"`                // section 5.9
	MaxItems             int                    `json:"maxItems,omitempty"`             // section 5.10
	MinItems             int                    `json:"minItems,omitempty"`             // section 5.11
	UniqueItems          bool                   `json:"uniqueItems,omitempty"`          // section 5.12
	MaxProperties        int                    `json:"maxProperties,omitempty"`        // section 5.13
	MinProperties        int                    `json:"minProperties,omitempty"`        // section 5.14
	Required             []string               `json:"required,omitempty"`             // section 5.15
	Properties           map[string]*Definition `json:"properties,omitempty"`           // section 5.16
	PatternProperties    map[string]*Definition `json:"patternProperties,omitempty"`    // section 5.17
	AdditionalProperties *Definition            `json:"additionalProperties,omitempty"` // section 5.18
	Dependencies         map[string]*Definition `json:"dependencies,omitempty"`         // section 5.19
	Enum                 []interface{}          `json:"enum,omitempty"`                 // section 5.20
	Type                 string                 `json:"type,omitempty"`                 // section 5.21
	AllOf                []*Definition          `json:"allOf,omitempty"`                // section 5.22
	AnyOf                []*Definition          `json:"anyOf,omitempty"`                // section 5.23
	OneOf                []*Definition          `json:"oneOf,omitempty"`                // section 5.24
	Not                  *Definition            `json:"not,omitempty"`                  // section 5.25
	Definitions          Definitions            `json:"definitions,omitempty"`          // section 5.26
	// RFC draft-wright-json-schema-validation-00, section 6, 7
	Title       string      `json:"title,omitempty"`       // section 6.1
	Description string      `json:"description,omitempty"` // section 6.1
	Default     interface{} `json:"default,omitempty"`     // section 6.2
	Format      string      `json:"format,omitempty"`      // section 7
	// RFC draft-wright-json-schema-hyperschema-00, section 4
	Media          *Definition `json:"media,omitempty"`          // section 4.3
	BinaryEncoding string      `json:"binaryEncoding,omitempty"` // section 4.3
}

// Prettifies the json in the input string
func prettifyJSON(input string) string {
	var prettyInput bytes.Buffer
	json.Indent(&prettyInput, []byte(input), "", "  ")
	return string(prettyInput.Bytes())
}

const defPrefix = "#/definitions/"

// Checks whether the typeName represents a simple json type
func isSimpleType(typeName string) bool {
	return typeName == "string" || typeName == "int" || typeName == "int64" || typeName == "bool"
}

// Converts the typeName simple type to json type
func jsonifyType(typeName string) string {
	switch typeName {
	case "string":
		return "string"
	case "bool":
		return "boolean"
	case "int":
		return "number"
	case "int64":
		return "number"
	}
	panic("jsonifyType called with a complex type")
}

// Gets the type name of the array
func getTypeNameOfArray(arrayType *ast.ArrayType) string {
	switch arrayType.Elt.(type) {
	case *ast.Ident:
		identifier := arrayType.Elt.(*ast.Ident)
		return identifier.Name
	case *ast.StarExpr:
		starType := arrayType.Elt.(*ast.StarExpr)
		identifier := starType.X.(*ast.Ident)
		return identifier.Name
	}
	panic("undefined type")
}

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
	tagValue := tag.Value
	if tagValue == "" {
		log.Panic("Tag value is empty")
	}

	// return yamlFieldValue, yamlOptionValue
	tagValue = removeChar(tagValue, "`")
	tagValue = removeChar(tagValue, `"`)
	tagValue = strings.TrimSpace(tagValue)

	var yamlTagContent string
	fmt.Sscanf(tagValue, `yaml: %s`, &yamlTagContent)

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

// Parses array type and returns its corresponding
// schema definition.
func parseArrayType(arrayType *ast.ArrayType) *Definition {
	definition := new(Definition)
	typeNameOfArray := getTypeNameOfArray(arrayType)

	definition.Items = new(Definition)
	if isSimpleType(typeNameOfArray) {
		definition.Items.Type = jsonifyType(typeNameOfArray)
	} else {
		definition.Items.Ref = getDefLink(typeNameOfArray)
	}
	definition.Type = "array"

	return definition
}

// Parses a struct type and returns its corresponding
// schema definition.
func parseStructType(structType *ast.StructType, typeName string, typeDescription string) *Definition {
	definition := &Definition{}
	definition.Description = typeDescription
	definition.Properties = make(map[string]*Definition)
	definition.Required = []string{}
	inlineDefinitions := []*Definition{}

	for _, field := range structType.Fields.List {
		property := new(Definition)
		yamlFieldName, option := extractFromTag(field.Tag)

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
			}
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
			property = parseArrayType(arrayType)
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
		case *ast.StarExpr:
			starExpr := field.Type.(*ast.StarExpr)
			starType := starExpr.X.(*ast.Ident)
			typeName := starType.Name

			if isSimpleType(typeName) {
				property.Type = jsonifyType(typeName)
			} else {
				property.Ref = getDefLink(typeName)
			}
		}
		// Set the common properties here as the cases might
		// overwrite 'property' pointer.
		property.Description = field.Doc.Text()

		definition.Properties[yamlFieldName] = property
	}

	if len(inlineDefinitions) == 0 {
		// There are no inlined definitions
		return definition
	}
	if len(inlineDefinitions) == 1 && len(definition.Properties) == 0 {
		// If there is only one "inline" definition
		// and there are no properties associated with
		// the parent definition, we just refer to the
		// "inline" definition.
		definition.Ref = inlineDefinitions[0].Ref
		return definition
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

	return parentDefinition
}

func main() {
	inputPath := flag.String("input-file", "", "Input go file path")
	outputPath := flag.String("output-file", "", "Output schema json path")

	flag.Parse()

	if len(*inputPath) == 0 || len(*outputPath) == 0 {
		log.Panic("Both input path and output paths need to be set")
	}

	// Open the input go file and parse the Abstract Syntax Tree
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, *inputPath, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	schema := Schema{
		Definition:  &Definition{},
		Definitions: make(map[string]*Definition)}
	schema.Type = "object"

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
				parsedArrayDef := parseArrayType(arrayType)

				schema.Definitions[typeName] = parsedArrayDef
			case *ast.StructType:
				structType := typeSpec.Type.(*ast.StructType)
				parsedStructDef := parseStructType(structType, typeName, typeDescription)

				schema.Definitions[typeName] = parsedStructDef
			}
		}
	}

	marshalledSchema, err := json.Marshal(schema)
	if err != nil {
		log.Panic(err)
	}
	output := prettifyJSON(string(marshalledSchema))
	err = ioutil.WriteFile(*outputPath, []byte(output), 0644)
	if err != nil {
		log.Panic(err)
	}
}
