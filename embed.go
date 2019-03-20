package main

import (
	"log"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

func embedSchema(defs map[string]v1beta1.JSONSchemaProps, startingTypes map[string]bool) map[string]v1beta1.JSONSchemaProps {
	newDefs := map[string]v1beta1.JSONSchemaProps{}
	for name := range startingTypes {
		def := defs[name]
		embedDefinition(&def, defs)
		newDefs[name] = def
	}
	return newDefs
}

func embedDefinition(def *v1beta1.JSONSchemaProps, refs map[string]v1beta1.JSONSchemaProps) {
	if def == nil {
		return
	}

	if def.Ref != nil && len(*def.Ref) > 0 {
		refName := strings.TrimPrefix(*def.Ref, defPrefix)
		ref, ok := refs[refName]
		if !ok {
			log.Panicf("can't find the definition of %q", refName)
		}
		def.Properties = ref.Properties
		def.Type = ref.Type
		def.Ref = nil
	}

	def.Definitions = embedDefinitionMap(def.Definitions, refs)
	def.Properties = embedDefinitionMap(def.Properties, refs)
	// TODO: decide if we want to do this.
	//def.AllOf = embedDefinitionArray(def.AllOf, refs)
	def.AnyOf = embedDefinitionArray(def.AnyOf, refs)
	def.OneOf = embedDefinitionArray(def.OneOf, refs)
	if def.AdditionalItems != nil {
		embedDefinition(def.AdditionalItems.Schema, refs)
	}
	if def.Items != nil {
		embedDefinition(def.Items.Schema, refs)
	}
	embedDefinition(def.Not, refs)
}

func embedDefinitionMap(defs map[string]v1beta1.JSONSchemaProps, refs map[string]v1beta1.JSONSchemaProps) map[string]v1beta1.JSONSchemaProps {
	newDefs := map[string]v1beta1.JSONSchemaProps{}
	for i := range defs {
		def := defs[i]
		embedDefinition(&def, refs)
		newDefs[i] = def
	}
	return newDefs
}

func embedDefinitionArray(defs []v1beta1.JSONSchemaProps, refs map[string]v1beta1.JSONSchemaProps) []v1beta1.JSONSchemaProps {
	newDefs := make([]v1beta1.JSONSchemaProps, len(defs))
	for i := range defs {
		def := defs[i]
		embedDefinition(&def, refs)
		newDefs[i] = def
	}
	return newDefs
}
