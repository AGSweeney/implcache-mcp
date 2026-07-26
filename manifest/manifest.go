// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package manifest loads optional .implcache.yaml workspace configuration.
package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultFilename is the conventional workspace manifest name.
const DefaultFilename = ".implcache.yaml"

// Manifest describes the current project knowledge root and related corpora.
type Manifest struct {
	RootName              string            `yaml:"rootName" json:"rootName"`
	Technology            []string          `yaml:"technology" json:"technology,omitempty"`
	Languages             []string          `yaml:"languages" json:"languages,omitempty"`
	Authority             string            `yaml:"authority" json:"authority,omitempty"`
	RelatedRoots          []string          `yaml:"relatedRoots" json:"relatedRoots,omitempty"`
	Versions              map[string]string `yaml:"versions" json:"versions,omitempty"`
	LibraryDocsHandling   string            `yaml:"libraryDocsHandling" json:"libraryDocsHandling,omitempty"` // auto|normal|exclude
}

// Load reads a manifest from path. Missing files return (nil, nil).
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return Parse(data)
}

// LoadFromDir looks for .implcache.yaml under dir.
func LoadFromDir(dir string) (*Manifest, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil
	}
	return Load(filepath.Join(dir, DefaultFilename))
}

// Parse validates and returns a Manifest from YAML bytes.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse .implcache.yaml: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks required fields.
func (m *Manifest) Validate() error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}
	m.RootName = strings.TrimSpace(m.RootName)
	if m.RootName == "" {
		return fmt.Errorf(".implcache.yaml: rootName is required")
	}
	if strings.ContainsAny(m.RootName, `/\`) {
		return fmt.Errorf(".implcache.yaml: rootName must not contain path separators")
	}
	if m.Authority == "" {
		m.Authority = "current_project"
	}
	return nil
}

// PreferredRoots returns project root first, then related roots.
func (m *Manifest) PreferredRoots() []string {
	if m == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(r string) {
		r = strings.TrimSpace(r)
		if r == "" {
			return
		}
		if _, ok := seen[r]; ok {
			return
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}
	add(m.RootName)
	for _, r := range m.RelatedRoots {
		add(r)
	}
	return out
}
