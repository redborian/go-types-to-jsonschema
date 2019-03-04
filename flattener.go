package main

// Recursively flattens "anyOf" tags. If there is cyclic
// dependency, execution is aborted.
func recursiveFlatten(schema *Schema, definition *Definition, defName string, visited *map[string]bool) *Definition {
	if len(definition.AllOf) == 0 {
		return definition
	}
	isAlreadyVisited := (*visited)[defName]
	if isAlreadyVisited {
		panic("Cycle detected in definitions")
	}
	(*visited)[defName] = true

	aggregatedDef := new(Definition)
	for _, allOfDef := range definition.AllOf {
		var newDef *Definition
		if allOfDef.Ref != "" {
			// If the definition has $ref url, fetch the referred resource
			// after flattening it.
			nameOfRef := getNameFromURL(allOfDef.Ref)
			newDef = recursiveFlatten(schema, schema.Definitions[nameOfRef], nameOfRef, visited)
		} else {
			newDef = allOfDef
		}
		mergeDefinitions(aggregatedDef, newDef)
	}

	delete(*visited, defName)
	return aggregatedDef
}

// Flattens the schema by inlining 'anyOf' tags.
func flattenSchema(schema *Schema) {
	for nameOfDef, def := range schema.Definitions {
		visited := make(map[string]bool)
		schema.Definitions[nameOfDef] = recursiveFlatten(schema, def, nameOfDef, &visited)
	}
}
