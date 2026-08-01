// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRootGroupsYAMLRootNameAndRole(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "groups.yaml")
	if err := os.WriteFile(path, []byte(`
groups:
  - id: netburner
    name: NetBurner
    description: test
    policies:
      allowCrossRootRetrieval: true
      preserveAuthorityRoles: true
      preferCurrentProjectWhenSpecified: true
      preferOfficialDocsForApiDefinitions: true
      preferOfficialExamplesForUsage: true
      includeRelatedProjectsForPatterns: true
      maxRelatedProjects: 2
    members:
      - rootName: NetBurner Examples
        role: official_example
        priority: 100
      - rootName: NetBurner Documents
        role: official_documentation
        priority: 90
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadRootGroupsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Groups) != 1 || cfg.Groups[0].ID != "netburner" {
		t.Fatalf("got %+v", cfg)
	}
	if cfg.Groups[0].Members[0].RootName != "NetBurner Examples" {
		t.Fatalf("yaml rootName not bound: %+v", cfg.Groups[0].Members[0])
	}
	if cfg.Groups[0].Members[0].Role != MemberRoleOfficialExample {
		t.Fatalf("role=%q", cfg.Groups[0].Members[0].Role)
	}
	if cfg.Groups[0].Policies.MaxRelatedProjects != 2 {
		t.Fatalf("policies=%+v", cfg.Groups[0].Policies)
	}

	st, err := Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	applied, err := st.ApplyRootGroupsFile(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 1 || applied[0] != "netburner" {
		t.Fatalf("applied=%v", applied)
	}
	g, err := st.LookupKnowledgeGroup(ctx, "netburner")
	if err != nil || g == nil {
		t.Fatal(err)
	}
	if len(g.Members) != 2 || g.Members[0].Role != MemberRoleOfficialExample {
		t.Fatalf("members=%+v", g.Members)
	}
}
