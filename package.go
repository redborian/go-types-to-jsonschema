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
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
)

// Package fetches relevant packages
type Package struct {
	Name string
}

// GoListOutput is
type GoListOutput struct {
	// Dir is
	Dir string `json:"Dir"`
	// ImportPath string `json:ImportPath`
	// Name string `json:Name`
	// Doc string `json:Doc`
	// Target string `json:Target`
	// Root string `json:Root`
	// Match []string `json:Match`
	// GoFiles is the list of GoFiles
	GoFiles []string `json:"GoFiles"`
	// Imports []string `json:Imports`
	// ImportMap map[string]string `json:ImportMap`
	// Deps []string ``
	// TestGoFiles []string
	// TestImports []string
}

// Fetch fetches package `pkgName`
func (p *Package) Fetch() {
	// fetch package
	// go get pkgName
	// pkgName := strings.Trim(p.Name, "\"")
	cmd := exec.Command("go", "get", p.Name)
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error while running go get command:", err)
	}
}

// GetBaseDirectory returns base directory
func (p *Package) GetBaseDirectory() string {
	return p.runGoList().Dir
}

// ListFiles lists all files
func (p *Package) ListFiles() []string {
	// go list -json pkgName
	// basePath = go list -f "{{.Dir}}" pkgName
	// godoc -src -html pkgName typeName
	// get filename from godoc output
	output := p.runGoList()
	for i := range output.GoFiles {
		output.GoFiles[i] = path.Join(output.Dir, output.GoFiles[i])
	}
	return output.GoFiles
}

func (p *Package) runGoList() GoListOutput {
	// Run go list command
	// pkgName := strings.Trim(p.Name, "\"")
	// fmt.Println("Go List package name: ", pkgName)
	cmd := exec.Command("go", "list", "-json", p.Name)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error running go list command:", err)
	}

	// fmt.Println("Go List output: ", stdout.String())
	// Unmarshal json output
	var output GoListOutput
	err = json.Unmarshal(stdout.Bytes(), &output)
	if err != nil {
		fmt.Println("error:", err)
	}
	return output
}
