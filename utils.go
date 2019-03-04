package main

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
