package main

import (
	"log"
	"strings"
)

func embedSchema(defs Definitions, startingTypes map[string]bool) {
	for name := range startingTypes {
		embedDefinition(defs[name], defs)
	}
}

func embedDefinition(def *Definition, refs Definitions) {
	if def == nil {
		return
	}

	if len(def.Ref) > 0 {
		refName := strings.TrimPrefix(def.Ref, defPrefix)
		ref, ok := refs[refName]
		if !ok {
			log.Panicf("can't find the definition of %q", refName)
		}
		def.Properties = ref.Properties
		def.Type = ref.Type
		def.Ref = ""
	}

	embedDefinitionMap(def.Definitions, refs)
	embedDefinitionMap(def.Properties, refs)
	// TODO: decide if we want to do this.
	//embedDefinitionArray(def.AllOf, refs)
	embedDefinitionArray(def.AnyOf, refs)
	embedDefinitionArray(def.OneOf, refs)
	embedDefinition(def.AdditionalItems, refs)
	embedDefinition(def.Items, refs)
	embedDefinition(def.Not, refs)
	embedDefinition(def.Media, refs)
}

func embedDefinitionMap(defs Definitions, refs Definitions) {
	for i := range defs {
		embedDefinition(defs[i], refs)
	}
}

func embedDefinitionArray(defs []*Definition, refs Definitions) {
	for i := range defs {
		embedDefinition(defs[i], refs)
	}
}
