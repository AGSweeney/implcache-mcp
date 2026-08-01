// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestKnowledgeGroupAutoExpandAndCrossGroupNeedsChoice(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "kg.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	docs := []UpsertInput{
		{URI: "project://NetBurner Documents/tcp.md", Title: "tcp", SourceType: SourceMarkdown, Path: "tcp.md",
			RootName: "NetBurner Documents", Authority: AuthorityOfficialDocs, Hash: "d1",
			Chunks: []Chunk{{Body: "TCP Listen Accept server API docs", StartLine: 1, EndLine: 5}}},
		{URI: "project://NetBurner Examples/main.cpp", Title: "ex", SourceType: SourceSource, Path: "main.cpp",
			RootName: "NetBurner Examples", Authority: AuthorityOfficialExample, Hash: "e1",
			Chunks: []Chunk{{Body: "Example TCP Listen Accept server", StartLine: 1, EndLine: 5}}},
		{URI: "project://NetBurner_MQTT_Broker/b.cpp", Title: "mqtt", SourceType: SourceSource, Path: "b.cpp",
			RootName: "NetBurner_MQTT_Broker", Authority: AuthorityCurrentProject, Hash: "m1",
			Chunks: []Chunk{{Body: "MQTT broker TCP patterns", StartLine: 1, EndLine: 5}}},
		{URI: "git://EnhancedClearCoreLibrary/a.cpp", Title: "cc", SourceType: SourceSource, Path: "a.cpp",
			RootName: "EnhancedClearCoreLibrary", Authority: AuthorityCurrentProject, Hash: "c1",
			Chunks: []Chunk{{Body: "ClearCore motor move", StartLine: 1, EndLine: 5}}},
	}
	for _, d := range docs {
		if _, err := st.UpsertDocument(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.UpsertKnowledgeGroup(ctx, RootGroup{
		ID: "netburner", Name: "NetBurner", Policies: DefaultKnowledgeGroupPolicies(),
		Members: []RootGroupMember{
			{RootName: "NetBurner Examples", Role: MemberRoleOfficialExample, Priority: 100},
			{RootName: "NetBurner Documents", Role: MemberRoleOfficialDocs, Priority: 90},
			{RootName: "NetBurner_MQTT_Broker", Role: MemberRoleRelatedProject, Priority: 80},
			{RootName: "NetBurner_Extra", Role: MemberRoleRelatedProject, Priority: 70},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertKnowledgeGroup(ctx, RootGroup{
		ID: "clearcore", Name: "ClearCore", Policies: DefaultKnowledgeGroupPolicies(),
		Members: []RootGroupMember{
			{RootName: "EnhancedClearCoreLibrary", Role: MemberRoleCurrentProject, Priority: 100},
		},
	}); err != nil {
		t.Fatal(err)
	}

	avail, _ := st.ListRootNames(ctx)

	inf, err := st.ValidateRootScope(ctx, []string{"NetBurner Documents", "NetBurner Examples"}, avail)
	if err != nil {
		t.Fatal(err)
	}
	if inf.NeedsChoice {
		t.Fatalf("same knowledge group should not needsChoice: %+v", inf)
	}
	if inf.KnowledgeGroup != "netburner" {
		t.Fatalf("KnowledgeGroup=%q", inf.KnowledgeGroup)
	}

	expanded, err := ExpandKnowledgeGroup(mustLookupKG(t, st, "netburner"), avail, ExpandOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(expanded, "NetBurner Documents") || !containsStr(expanded, "NetBurner Examples") {
		t.Fatalf("expanded missing docs/examples: %v", expanded)
	}
	if !containsStr(expanded, "NetBurner_MQTT_Broker") {
		t.Fatalf("expected related project included, got %v", expanded)
	}

	g := mustLookupKG(t, st, "netburner")
	g.Policies.MaxRelatedProjects = 0
	g.Policies.IncludeRelatedProjectsForPatterns = true
	capped, err := ExpandKnowledgeGroup(g, avail, ExpandOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if containsStr(capped, "NetBurner_MQTT_Broker") {
		t.Fatalf("maxRelatedProjects=0 should exclude related: %v", capped)
	}

	inf, err = st.ValidateRootScope(ctx, []string{"NetBurner Documents", "EnhancedClearCoreLibrary"}, avail)
	if err != nil {
		t.Fatal(err)
	}
	if !inf.NeedsChoice || len(inf.AvailableGroups) < 2 {
		t.Fatalf("expected cross-group needsChoice, got %+v", inf)
	}

	inf, err = st.ResolveRoots(ctx, "anything", []string{"NetBurner_MQTT_Broker"})
	if err != nil {
		t.Fatal(err)
	}
	if inf.NeedsChoice || len(inf.Roots) != 1 || inf.Roots[0] != "NetBurner_MQTT_Broker" {
		t.Fatalf("single root should stay narrow: %+v", inf)
	}

	deny := DefaultKnowledgeGroupPolicies()
	deny.AllowCrossRootRetrieval = false
	if err := st.UpsertKnowledgeGroup(ctx, RootGroup{
		ID: "netburner", Name: "NetBurner", Policies: deny,
		Members: []RootGroupMember{
			{RootName: "NetBurner Examples", Role: MemberRoleOfficialExample, Priority: 100},
			{RootName: "NetBurner Documents", Role: MemberRoleOfficialDocs, Priority: 90},
		},
	}); err != nil {
		t.Fatal(err)
	}
	inf, err = st.ValidateRootScope(ctx, []string{"NetBurner Documents", "NetBurner Examples"}, avail)
	if err != nil {
		t.Fatal(err)
	}
	if !inf.NeedsChoice {
		t.Fatalf("allowCrossRootRetrieval=false should needsChoice: %+v", inf)
	}
}

func TestSchema11To12Migration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mig.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`PRAGMA user_version = 11`); err != nil {
		t.Fatal(err)
	}
	st.Close()

	st2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen migrate: %v", err)
	}
	defer st2.Close()
	v, err := st2.SchemaVersion(context.Background())
	if err != nil || v != 12 {
		t.Fatalf("after migrate version=%d err=%v", v, err)
	}
	if err := st2.UpsertKnowledgeGroup(context.Background(), RootGroup{
		ID: "netburner", Name: "NetBurner", Policies: DefaultKnowledgeGroupPolicies(),
		Members: []RootGroupMember{{RootName: "NetBurner Documents", Role: MemberRoleOfficialDocs, Priority: 90}},
	}); err != nil {
		t.Fatal(err)
	}
	g, err := st2.LookupKnowledgeGroup(context.Background(), "netburner")
	if err != nil || g == nil || len(g.Members) == 0 || g.Members[0].Role != MemberRoleOfficialDocs {
		t.Fatalf("lookup after migrate: g=%+v err=%v", g, err)
	}
}

func mustLookupKG(t *testing.T, st *Store, id string) *RootGroup {
	t.Helper()
	g, err := st.LookupKnowledgeGroup(context.Background(), id)
	if err != nil || g == nil {
		t.Fatalf("lookup %s: %v %#v", id, err, g)
	}
	return g
}

func containsStr(in []string, want string) bool {
	for _, s := range in {
		if s == want {
			return true
		}
	}
	return false
}
