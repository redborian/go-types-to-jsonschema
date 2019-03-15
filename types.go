// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at

//   http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

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

// TypeReference denotes the (typeName, packageName) tuple
type TypeReference struct {
	TypeName    string
	PackageName string
}

// ExternalReferences map contains list of "type: packageName" entries
type ExternalReferences map[string][]TypeReference
