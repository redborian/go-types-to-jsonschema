# Go YAML-annotated types to JSON schema converter
[![Go Report Card](https://goreportcard.com/badge/github.com/redborian/go-yaml-to-jsonschema)](https://goreportcard.com/report/github.com/redborian/go-yaml-to-jsonschema)
[![Go Doc](https://godoc.org/github.com/redborian/go-yaml-to-jsonschema?status.svg)](https://godoc.org/github.com/redborian/go-yaml-to-jsonschema)

Command-line tool to convert yaml-annotated types specified in go file to json schema. It parses the Go file into an abstract-syntax-tree and generates its corresponding [json-schema](https://json-schema.org/) output.

### Input go file:
```go
type Person struct {
  Name     string    `yaml:"name"`
  Age      int       `yaml:"age,omitempty"`
  Address  *Address  `yaml:"address,omitempty"`
}

type Address struct {
}
```

### Output json file:
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
    "Address": { }
  }
}
```

## Running the tool
Currently, the only way to run the tool is through `go run`
1. Clone the repo
2. `go run main.go --input-file="<Go-file-path>" --output-file="<Output-json-path>"`



Note: This is not an official Google product
