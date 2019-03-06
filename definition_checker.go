package main

import "fmt"

func checkDefinitions(defs Definitions, startingTypes map[string]bool) {
	fmt.Printf("Type checking Starting expecting %d types\n", len(defs))
	pruner := DefinitionPruner{defs, startingTypes}
	newDefs := pruner.Prune(false)
	if len(defs) != len(defs) {
		fmt.Printf("Type checking failed. Expected %d actual %d\n", len(defs), len(newDefs))
	} else {
		fmt.Println("Type checking PASSED")
	}
}
