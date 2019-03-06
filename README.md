# Go annotated types to JSON schema converter
[![Go Report Card](https://goreportcard.com/badge/github.com/redborian/go-types-to-jsonschema)](https://goreportcard.com/report/github.com/redborian/go-types-to-jsonschema)
[![Go Doc](https://godoc.org/github.com/redborian/go-types-to-jsonschema?status.svg)](https://godoc.org/github.com/redborian/go-types-to-jsonschema)

Command-line tool to convert annotated go types specified in a package to json schema. It parses the Go file into an abstract-syntax-tree and generates its corresponding [json-schema](https://json-schema.org/) output. If any of the types depend on types from other packages, they will also be recursively converted.

## How it works
The tool uses go parser to parse all the go files in the provided package. If it accesses types from other packages, it recursively processes those as well. It uses `go get` and `go list` commands to fetch and list files in a package. It is smart enough to not process types that are not relevant.

## Example
### Package contents
```go
package types

type Person struct {
  Name     string    `yaml:"name"`
  Age      int       `yaml:"age,omitempty"`
  Address  *Address  `yaml:"address,omitempty"`
}

type Car struct {
  Make     string    `json:"make"`
}

type Address struct {
}
```

### Command to run
```
$> go build
$> go-types-to-json --package-name="github.com/pkg/name" --output-file="output.json" --types="Person,Car"
```

### Contents of output.json
```json
{
  "definitions": {
    "Person": {
      "properties": {
        "name": {
          "type": "string"
        },
        "age": {
          "type": "number"
        },
        "address": {
          "$ref": "#/definitions/Address"
        }
      },
      "required": ["name"]
    },
    "Address": { },
    "Car": {
      "properties": {
        "make": {
          "type": "string"
        }
      },
      "required": ["make"]
    }
  }
}
```

Note: This is not an official Google product
