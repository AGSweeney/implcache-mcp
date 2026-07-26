// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarydocs

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// OptionalManifestPath is a future convention file (not required).
const OptionalManifestPath = "LibraryDocs/librarydocs.yaml"

// OptionalManifest is the future librarydocs.yaml schema (parser stub).
type OptionalManifest struct {
	Schema          string `yaml:"schema"`
	SchemaVersion   int    `yaml:"schema_version"`
	StandardVersion string `yaml:"standard_version"`
	Index           string `yaml:"index"`
	Inventory       string `yaml:"inventory"`
	Artifacts       string `yaml:"artifacts"`
	Validation      string `yaml:"validation"`
}

// LoadOptionalManifest loads librarydocs.yaml if present. Missing is not an error.
func LoadOptionalManifest(checkout string) (*OptionalManifest, error) {
	p := filepath.Join(checkout, filepath.FromSlash(OptionalManifestPath))
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m OptionalManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
