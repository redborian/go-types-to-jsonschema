package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func isSimpleType(typeName string) bool {
	return typeName == "string" || typeName == "int" || typeName == "int32" || typeName == "int64" || typeName == "bool"
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
	case "int32":
		return "number"
	case "int64":
		return "number"
	}
	fmt.Println("jsonifyType called with a complex type ", typeName)
	panic("jsonifyType called with a complex type")
}

func mergeDefs(lhs Definitions, rhs Definitions) {
	for key := range rhs {
		_, ok := lhs[key]
		if ok {
			fmt.Println("Definition ", key, " already present")
			continue
		}
		lhs[key] = rhs[key]
	}
}

func mergeExternalRefs(lhs ExternalReferences, rhs ExternalReferences) {
	for key := range rhs {
		_, ok := lhs[key]
		if !ok {
			lhs[key] = rhs[key]
		} else {
			lhs[key] = append(lhs[key], rhs[key]...)
		}
	}
}

func debugPrint(obj interface{}) {
	b, err3 := json.Marshal(obj)
	if err3 != nil {
		panic("Error")
	}
	fmt.Println(string(b))
}

// Gets the schema definition link of a resource
func getDefLink(resourceName string) string {
	return defPrefix + resourceName
}

// Gets the resource name from definitions url.
// Eg, returns 'TypeName' from '#/definitions/TypeName'
func getNameFromURL(url string) string {
	slice := strings.Split(url, "/")
	return slice[len(slice)-1]
}
