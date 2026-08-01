// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import (
	"testing"

	"implcache-mcp/store"
)

func TestAttachRootContribution(t *testing.T) {
	resp := &Response{
		Citations: []Citation{
			{URI: "project://NetBurner Documents/tcp.md", RootName: "NetBurner Documents", Title: "TCP"},
			{URI: "project://NetBurner Documents/listen.md", RootName: "NetBurner Documents", Title: "Listen"},
			{URI: "project://NetBurner Examples/main.cpp", RootName: "NetBurner Examples", Title: "main"},
			{URI: "project://NetBurner_MQTT_Broker/tcp.cpp", RootName: "NetBurner_MQTT_Broker", Title: "broker"},
		},
		Examples: []ExampleRef{
			{URI: "project://NetBurner Examples/main.cpp", RootName: "NetBurner Examples", Title: "main"},
		},
	}
	roles := map[string]string{
		"NetBurner Documents":    store.MemberRoleOfficialDocs,
		"NetBurner Examples":     store.MemberRoleOfficialExample,
		"NetBurner_MQTT_Broker":  store.MemberRoleRelatedProject,
		"NetBurner_MSSQL_Client": store.MemberRoleRelatedProject,
	}
	searched := []string{
		"NetBurner Documents", "NetBurner Examples", "NetBurner_MQTT_Broker",
		"NetBurner_MSSQL_Client", "NetBurner_Micro800_MQTT_Gateway",
	}
	policies := store.DefaultKnowledgeGroupPolicies()
	policies.MaxRelatedProjects = 1

	attachRootContribution(resp, searched, roles, policies)
	c := resp.RootContribution
	if c == nil {
		t.Fatal("nil contribution")
	}
	if c.RootsSearched != 5 {
		t.Fatalf("rootsSearched=%d", c.RootsSearched)
	}
	if c.RootsContributing != 3 {
		t.Fatalf("rootsContributing=%d want 3; contrib=%v", c.RootsContributing, c.ContributingRoots)
	}
	if len(c.UnusedSearchedRoots) != 2 {
		t.Fatalf("unused=%v", c.UnusedSearchedRoots)
	}
	if c.RelatedContributing != 1 || c.RelatedOverLimit != 0 {
		t.Fatalf("related=%d over=%d", c.RelatedContributing, c.RelatedOverLimit)
	}
	if len(c.CitationsByRoot) < 3 || c.CitationsByRoot[0].Count < c.CitationsByRoot[1].Count {
		t.Fatalf("citationsByRoot not sorted: %+v", c.CitationsByRoot)
	}
}

func TestRelatedOverLimitAndNearDup(t *testing.T) {
	resp := &Response{
		Citations: []Citation{
			{URI: "project://A/src/tcp/listen.md", RootName: "A", Title: "Listen Accept"},
			{URI: "project://B/docs/tcp/listen.md", RootName: "B", Title: "Listen Accept"},
			{URI: "project://C/foo.cpp", RootName: "C", Title: "other"},
			{URI: "project://D/bar.cpp", RootName: "D", Title: "other2"},
		},
	}
	roles := map[string]string{
		"A": store.MemberRoleOfficialDocs,
		"B": store.MemberRoleRelatedProject,
		"C": store.MemberRoleRelatedProject,
		"D": store.MemberRoleRelatedProject,
	}
	policies := store.DefaultKnowledgeGroupPolicies()
	policies.MaxRelatedProjects = 1
	attachRootContribution(resp, []string{"A", "B", "C", "D"}, roles, policies)
	c := resp.RootContribution
	if c.RelatedOverLimit != 2 {
		t.Fatalf("relatedOverLimit=%d want 2", c.RelatedOverLimit)
	}
	if c.NearDuplicatePairs < 1 {
		t.Fatalf("expected near-duplicate pairs, got %d", c.NearDuplicatePairs)
	}
}
