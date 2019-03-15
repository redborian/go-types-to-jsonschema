// Copyright 2019 Google LLC

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     https://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import "fmt"

// DefinitionPruner prunes unwanted definitions
type DefinitionPruner struct {
	definitions   Definitions
	startingTypes map[string]bool
}

// Prune prunes the definitions
func (pruner *DefinitionPruner) Prune(ignoreUnknownTypes bool) map[string]bool {
	visitedDefs := make(map[string]bool)
	queue := make([]string, 0)
	// Push starting types into queue
	for typeName := range pruner.startingTypes {
		queue = append(queue, typeName)
	}

	// Perform BFS and keep track of visited types
	for len(queue) > 0 {
		curType := queue[0]
		queue = queue[1:]
		// If already visited, skip it
		if _, exists := visitedDefs[curType]; exists {
			continue
		}
		// If no definitions present, (probably an external reference)
		// Skip it
		if _, exists := pruner.definitions[curType]; !exists {
			if ignoreUnknownTypes {
				continue
			} else {
				fmt.Println("Unknown type ", curType)
				panic("Unknown type")
			}
		}
		visitedDefs[curType] = true
		curDef := pruner.definitions[curType]
		queue = append(queue, processDefinition(curDef)...)
	}

	return visitedDefs
}

func processDefinition(def *Definition) []string {
	allTypes := []string{}
	if def == nil {
		return allTypes
	}
	if def.Ref != "" {
		allTypes = append(allTypes, getNameFromURL(def.Ref))
	}
	allTypes = append(allTypes, processDefinitionMap(def.Definitions)...)
	allTypes = append(allTypes, processDefinitionMap(def.Properties)...)
	allTypes = append(allTypes, processDefinitionArray(def.AllOf)...)
	allTypes = append(allTypes, processDefinitionArray(def.AnyOf)...)
	allTypes = append(allTypes, processDefinitionArray(def.OneOf)...)
	allTypes = append(allTypes, processDefinition(def.AdditionalItems)...)
	allTypes = append(allTypes, processDefinition(def.Items)...)
	allTypes = append(allTypes, processDefinition(def.Not)...)
	allTypes = append(allTypes, processDefinition(def.Media)...)
	return allTypes
}

func processDefinitionMap(defMap map[string]*Definition) []string {
	allTypes := []string{}
	if defMap == nil {
		return allTypes
	}
	for key := range defMap {
		allTypes = append(allTypes, processDefinition(defMap[key])...)
	}
	return allTypes
}

func processDefinitionArray(defArray []*Definition) []string {
	allTypes := []string{}
	if defArray == nil {
		return allTypes
	}
	for _, def := range defArray {
		allTypes = append(allTypes, processDefinition(def)...)
	}
	return allTypes
}
