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

package main

import (
	"flag"
	"strings"

	"github.com/redborian/go-types-to-jsonschema/pkg/crd"
)

func main() {
	op := &crd.Options{}

	flag.StringVar(&op.InputPackage, "package-name", "", "Go package name")
	flag.StringVar(&op.OutputPath, "output-file", "", "Output schema json path")
	// TODO: use cobra StringSlice https://godoc.org/github.com/spf13/pflag#StringSlice
	typeList := flag.String("types", "", "List of types")
	flag.BoolVar(&op.Flatten, "flatten schema", false, "If flatten the schema using ref tag")
	flag.StringVar(&op.OutputFormat, "output-format", "json", "Output format of the schema")

	flag.Parse()

	op.Types = strings.Split(*typeList, ",")

	op.Generate()
}
