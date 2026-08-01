// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// RootGroupsFile is the on-disk knowledge-group configuration document.
type RootGroupsFile struct {
	Groups []RootGroup `yaml:"groups" json:"groups"`
}

// LoadRootGroupsFile parses a knowledge-groups / root-groups YAML config.
func LoadRootGroupsFile(path string) (RootGroupsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RootGroupsFile{}, err
	}
	var cfg RootGroupsFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return RootGroupsFile{}, fmt.Errorf("knowledge-groups yaml: %w", err)
	}
	if len(cfg.Groups) == 0 {
		return RootGroupsFile{}, fmt.Errorf("knowledge-groups yaml: no groups defined")
	}
	for i := range cfg.Groups {
		g := &cfg.Groups[i]
		if strings.TrimSpace(g.Name) == "" {
			return RootGroupsFile{}, fmt.Errorf("knowledge-groups yaml: groups[%d].name required", i)
		}
		if strings.TrimSpace(g.ID) == "" {
			g.ID = slugKnowledgeGroupID(g.Name)
		}
		if len(g.Members) == 0 {
			return RootGroupsFile{}, fmt.Errorf("knowledge-groups yaml: group %q has no members", g.Name)
		}
		for j, m := range g.Members {
			if strings.TrimSpace(m.RootName) == "" {
				return RootGroupsFile{}, fmt.Errorf("knowledge-groups yaml: group %q members[%d].rootName required", g.Name, j)
			}
			if role := strings.TrimSpace(m.Role); role != "" && !validMemberRole(role) {
				return RootGroupsFile{}, fmt.Errorf("knowledge-groups yaml: group %q member %q invalid role %q", g.Name, m.RootName, role)
			}
		}
		// Merge omitted policy fields with defaults.
		g.Policies = mergePolicies(DefaultKnowledgeGroupPolicies(), g.Policies)
	}
	return cfg, nil
}

func mergePolicies(base, overlay KnowledgeGroupPolicies) KnowledgeGroupPolicies {
	// YAML decode leaves false for omitted bools — we cannot distinguish omit vs false.
	// Config authors should set policies explicitly; we still normalize maxRelated.
	out := overlay
	// If overlay looks completely zeroed, use base defaults.
	if !overlay.AllowCrossRootRetrieval && !overlay.PreserveAuthorityRoles &&
		!overlay.PreferCurrentProjectWhenSpecified && !overlay.PreferOfficialDocsForAPIDefinitions &&
		!overlay.PreferOfficialExamplesForUsage && !overlay.IncludeRelatedProjectsForPatterns &&
		overlay.MaxRelatedProjects == 0 {
		return base
	}
	if out.MaxRelatedProjects <= 0 && out.IncludeRelatedProjectsForPatterns {
		out.MaxRelatedProjects = base.MaxRelatedProjects
		if out.MaxRelatedProjects <= 0 {
			out.MaxRelatedProjects = 3
		}
	}
	return out.Normalize()
}

// ApplyRootGroupsFile upserts every group from the config into the store.
// Members whose root_name is absent from the DB are still stored (optional roots).
func (s *Store) ApplyRootGroupsFile(ctx context.Context, path string) ([]string, error) {
	cfg, err := LoadRootGroupsFile(path)
	if err != nil {
		return nil, err
	}
	var applied []string
	for _, g := range cfg.Groups {
		if err := s.UpsertKnowledgeGroup(ctx, g); err != nil {
			return applied, fmt.Errorf("upsert group %q: %w", g.Name, err)
		}
		key := g.ID
		if key == "" {
			key = g.Name
		}
		applied = append(applied, key)
	}
	return applied, nil
}
