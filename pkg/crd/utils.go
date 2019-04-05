// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crd

import (
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

func isSimpleType(typeName string) bool {
	return typeName == "string" || typeName == "int" ||
		typeName == "int32" || typeName == "int64" ||
		typeName == "bool" || typeName == "byte" ||
		typeName == "float32" || typeName == "float64"
}

// Converts the typeName simple type to json type
func jsonifyType(typeName string) string {
	switch typeName {
	case "string":
		return "string"
	case "bool":
		return "boolean"
	case "int":
		return "integer"
	case "int32":
		return "integer"
	case "int64":
		return "integer"
	case "float32":
		return "float"
	case "float64":
		return "float"
	case "byte":
		return "string"
	}
	fmt.Println("jsonifyType called with a complex type ", typeName)
	panic("jsonifyType called with a complex type")
}

func mergeDefs(lhs v1beta1.JSONSchemaDefinitions, rhs v1beta1.JSONSchemaDefinitions) {
	if lhs == nil || rhs == nil {
		return
	}
	for key := range rhs {
		_, ok := lhs[key]
		if ok {
			// change this to logger
			fmt.Println("JSONSchemaProps ", key, " already present")
			continue
		}
		lhs[key] = rhs[key]
	}
}

func mergeExternalRefs(lhs ExternalReferences, rhs ExternalReferences) {
	if lhs == nil || rhs == nil {
		return
	}
	for key := range rhs {
		_, ok := lhs[key]
		if !ok {
			lhs[key] = rhs[key]
		} else {
			lhs[key] = append(lhs[key], rhs[key]...)
		}
	}
}

func mergeCRDSpecs(lhs, rhs crdSpecByKind) {
	if lhs == nil || rhs == nil {
		return
	}
	for key := range rhs {
		_, ok := lhs[key]
		if ok {
			// TODO: change this to use logger
			fmt.Printf("CRD spec for kind %q already present", key)
			continue
		}
		lhs[key] = rhs[key]
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
func getDefLink(resourceName string) *string {
	ret := defPrefix + resourceName
	return &ret
}

func getFullName(resourceName string, prefix string) string {
	if prefix == "" {
		return resourceName
	}
	prefix = strings.Replace(prefix, "/", ".", -1)
	return prefix + "." + resourceName
}

func getPrefixedDefLink(resourceName string, prefix string) *string {
	ret := defPrefix + getFullName(resourceName, prefix)
	return &ret
}

// Gets the resource name from definitions url.
// Eg, returns 'TypeName' from '#/definitions/TypeName'
func getNameFromURL(url string) string {
	slice := strings.Split(url, "/")
	return slice[len(slice)-1]
}
