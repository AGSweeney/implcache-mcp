// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Knowledge-group member roles (participation in a group; distinct from document authority).
const (
	MemberRoleOfficialDocs     = "official_documentation"
	MemberRoleOfficialExample  = "official_example"
	MemberRoleCurrentProject   = "current_project"
	MemberRoleRelatedProject   = "related_project"
	MemberRoleCuratedKnowledge = "curated_knowledge"
)

// KnowledgeGroupPolicies controls how a group's members may be combined at search time.
type KnowledgeGroupPolicies struct {
	AllowCrossRootRetrieval             bool `json:"allowCrossRootRetrieval" yaml:"allowCrossRootRetrieval"`
	PreserveAuthorityRoles              bool `json:"preserveAuthorityRoles" yaml:"preserveAuthorityRoles"`
	PreferCurrentProjectWhenSpecified   bool `json:"preferCurrentProjectWhenSpecified" yaml:"preferCurrentProjectWhenSpecified"`
	PreferOfficialDocsForAPIDefinitions bool `json:"preferOfficialDocsForApiDefinitions" yaml:"preferOfficialDocsForApiDefinitions"`
	PreferOfficialExamplesForUsage      bool `json:"preferOfficialExamplesForUsage" yaml:"preferOfficialExamplesForUsage"`
	IncludeRelatedProjectsForPatterns   bool `json:"includeRelatedProjectsForPatterns" yaml:"includeRelatedProjectsForPatterns"`
	MaxRelatedProjects                  int  `json:"maxRelatedProjects" yaml:"maxRelatedProjects"`
}

// DefaultKnowledgeGroupPolicies returns the recommended NetBurner-style defaults.
func DefaultKnowledgeGroupPolicies() KnowledgeGroupPolicies {
	return KnowledgeGroupPolicies{
		AllowCrossRootRetrieval:             true,
		PreserveAuthorityRoles:              true,
		PreferCurrentProjectWhenSpecified:   true,
		PreferOfficialDocsForAPIDefinitions: true,
		PreferOfficialExamplesForUsage:      true,
		IncludeRelatedProjectsForPatterns:   true,
		MaxRelatedProjects:                  3,
	}
}

// Normalize clamps invalid negative MaxRelatedProjects. A value of 0 means
// "include no related projects" when IncludeRelatedProjectsForPatterns is true.
func (p KnowledgeGroupPolicies) Normalize() KnowledgeGroupPolicies {
	out := p
	if out.MaxRelatedProjects < 0 {
		out.MaxRelatedProjects = 0
	}
	return out
}

// RootGroupMember is one root inside a knowledge group.
type RootGroupMember struct {
	RootName string `json:"rootName" yaml:"rootName"`
	Priority int    `json:"priority" yaml:"priority"`
	Role     string `json:"role,omitempty" yaml:"role,omitempty"`
}

// RootGroup is a named knowledge group of roots (DB table remains root_groups).
type RootGroup struct {
	ID          string                 `json:"id,omitempty" yaml:"id,omitempty"`
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Policies    KnowledgeGroupPolicies `json:"policies,omitempty" yaml:"policies,omitempty"`
	Members     []RootGroupMember      `json:"members,omitempty" yaml:"members,omitempty"`
}

// ListRootGroups returns all knowledge groups with members.
func (s *Store) ListRootGroups(ctx context.Context) ([]RootGroup, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT name, COALESCE(description, ''), COALESCE(id, ''), COALESCE(policies_json, '{}')
		FROM root_groups ORDER BY name`)
	if err != nil {
		return nil, err
	}
	var out []RootGroup
	for rows.Next() {
		var g RootGroup
		var policies string
		if err := rows.Scan(&g.Name, &g.Description, &g.ID, &policies); err != nil {
			rows.Close()
			return nil, err
		}
		g.Policies = parsePoliciesJSON(policies)
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for i := range out {
		members, err := s.ListRootGroupMembers(ctx, out[i].Name)
		if err != nil {
			return nil, err
		}
		out[i].Members = members
	}
	return out, nil
}

// LookupKnowledgeGroup finds a group by stable id or display name (case-insensitive).
func (s *Store) LookupKnowledgeGroup(ctx context.Context, idOrName string) (*RootGroup, error) {
	key := strings.TrimSpace(idOrName)
	if key == "" {
		return nil, fmt.Errorf("knowledgeGroup is required")
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT name, COALESCE(description, ''), COALESCE(id, ''), COALESCE(policies_json, '{}')
		FROM root_groups
		WHERE lower(id)=lower(?) OR lower(name)=lower(?)
		LIMIT 1`, key, key)
	var g RootGroup
	var policies string
	if err := row.Scan(&g.Name, &g.Description, &g.ID, &policies); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	g.Policies = parsePoliciesJSON(policies)
	members, err := s.ListRootGroupMembers(ctx, g.Name)
	if err != nil {
		return nil, err
	}
	g.Members = members
	return &g, nil
}

// KnowledgeGroupsForRoots maps each root to its knowledge-group id (or name if id empty).
// Roots that are not members of any group are omitted from the map.
func (s *Store) KnowledgeGroupsForRoots(ctx context.Context, roots []string) (map[string]string, error) {
	out := map[string]string{}
	for _, r := range roots {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		row := s.db.QueryRowContext(ctx, `
			SELECT COALESCE(g.id, ''), g.name
			FROM root_group_members m
			JOIN root_groups g ON g.name = m.group_name
			WHERE m.root_name = ?
			ORDER BY g.name
			LIMIT 1`, r)
		var id, name string
		if err := row.Scan(&id, &name); err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return nil, err
		}
		if id == "" {
			id = name
		}
		out[r] = id
	}
	return out, nil
}

// DistinctKnowledgeGroups returns sorted unique group ids covering the given roots.
func (s *Store) DistinctKnowledgeGroups(ctx context.Context, roots []string) ([]string, error) {
	m, err := s.KnowledgeGroupsForRoots(ctx, roots)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var out []string
	for _, id := range m {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// MemberRoleByRoot returns role keyed by root name for a loaded group.
func MemberRoleByRoot(g *RootGroup) map[string]string {
	out := map[string]string{}
	if g == nil {
		return out
	}
	for _, m := range g.Members {
		if m.RootName == "" {
			continue
		}
		out[m.RootName] = strings.TrimSpace(m.Role)
	}
	return out
}

// DeleteRootGroup removes a group and its members (by name or id).
func (s *Store) DeleteRootGroup(ctx context.Context, name string) (bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, fmt.Errorf("group name is required")
	}
	g, err := s.LookupKnowledgeGroup(ctx, name)
	if err != nil {
		return false, err
	}
	if g == nil {
		return false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM root_group_members WHERE group_name=?`, g.Name); err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM root_groups WHERE name=?`, g.Name)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return n > 0, nil
}

// UpsertRootGroup replaces membership for a named root group (legacy helper).
func (s *Store) UpsertRootGroup(ctx context.Context, name, description string, members []RootGroupMember) error {
	return s.UpsertKnowledgeGroup(ctx, RootGroup{
		Name:        name,
		Description: description,
		Members:     members,
		Policies:    DefaultKnowledgeGroupPolicies(),
	})
}

// UpsertKnowledgeGroup upserts a full knowledge group (id, policies, roles).
func (s *Store) UpsertKnowledgeGroup(ctx context.Context, g RootGroup) error {
	name := strings.TrimSpace(g.Name)
	if name == "" {
		return fmt.Errorf("group name is required")
	}
	id := strings.TrimSpace(g.ID)
	if id == "" {
		id = slugKnowledgeGroupID(name)
	}
	policies := g.Policies
	// When all-zero from incomplete callers, use defaults.
	if !policies.AllowCrossRootRetrieval && !policies.PreserveAuthorityRoles &&
		!policies.PreferCurrentProjectWhenSpecified && policies.MaxRelatedProjects == 0 &&
		!policies.IncludeRelatedProjectsForPatterns {
		policies = DefaultKnowledgeGroupPolicies()
	} else {
		policies = policies.Normalize()
	}
	raw, err := json.Marshal(policies)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO root_groups(name, description, id, policies_json) VALUES(?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			description=excluded.description,
			id=excluded.id,
			policies_json=excluded.policies_json`,
		name, g.Description, id, string(raw)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM root_group_members WHERE group_name=?`, name); err != nil {
		return err
	}
	for _, m := range g.Members {
		root := strings.TrimSpace(m.RootName)
		if root == "" {
			continue
		}
		role := strings.TrimSpace(m.Role)
		if role != "" && !validMemberRole(role) {
			return fmt.Errorf("group %q member %q: invalid role %q", name, root, role)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO root_group_members(group_name, root_name, priority, role) VALUES(?,?,?,?)`,
			name, root, m.Priority, role); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListRootGroupMembers returns roots ordered by priority desc.
func (s *Store) ListRootGroupMembers(ctx context.Context, group string) ([]RootGroupMember, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT root_name, priority, COALESCE(role, '') FROM root_group_members
		WHERE group_name=? ORDER BY priority DESC, root_name`, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RootGroupMember
	for rows.Next() {
		var m RootGroupMember
		if err := rows.Scan(&m.RootName, &m.Priority, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ExpandOpts configures ExpandKnowledgeGroup.
type ExpandOpts struct {
	PreferredRoots []string
	ProjectRoot    string
	// FilterToPreferred: when true and PreferredRoots intersect the group, return
	// only that intersection (explicit knowledgeGroup + preferredRoots).
	FilterToPreferred bool
}

// ExpandKnowledgeGroup builds the searchable root list for a knowledge group.
func ExpandKnowledgeGroup(g *RootGroup, available []string, opt ExpandOpts) ([]string, error) {
	if g == nil {
		return nil, fmt.Errorf("knowledge group is required")
	}
	pol := g.Policies.Normalize()
	if !pol.AllowCrossRootRetrieval {
		return nil, fmt.Errorf("knowledge group %q forbids cross-root retrieval", groupKey(g))
	}
	availSet := map[string]struct{}{}
	for _, r := range available {
		availSet[r] = struct{}{}
	}

	var present []RootGroupMember
	for _, m := range g.Members {
		if _, ok := availSet[m.RootName]; !ok {
			continue
		}
		present = append(present, m)
	}
	if len(present) == 0 {
		return nil, fmt.Errorf("knowledge group %q members are not present in this database", groupKey(g))
	}

	groupSet := map[string]struct{}{}
	roleByRoot := map[string]string{}
	for _, m := range present {
		groupSet[m.RootName] = struct{}{}
		roleByRoot[m.RootName] = m.Role
	}

	if opt.FilterToPreferred {
		var preferredInGroup []string
		for _, r := range opt.PreferredRoots {
			r = strings.TrimSpace(r)
			if _, ok := groupSet[r]; ok {
				preferredInGroup = append(preferredInGroup, r)
			}
		}
		if len(preferredInGroup) > 0 {
			out := uniqueSorted(preferredInGroup)
			if pr := strings.TrimSpace(opt.ProjectRoot); pr != "" {
				if _, ok := groupSet[pr]; ok {
					out = uniqueSorted(append(out, pr))
				}
			}
			return out, nil
		}
	}

	seen := map[string]struct{}{}
	var out []string
	add := func(r string) {
		r = strings.TrimSpace(r)
		if r == "" {
			return
		}
		if _, ok := groupSet[r]; !ok {
			return
		}
		if _, ok := seen[r]; ok {
			return
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}

	// Core corpora by role.
	for _, m := range present {
		switch m.Role {
		case MemberRoleOfficialDocs, MemberRoleOfficialExample, MemberRoleCuratedKnowledge,
			MemberRoleCurrentProject:
			add(m.RootName)
		}
	}

	pr := strings.TrimSpace(opt.ProjectRoot)
	if pr != "" && pol.PreferCurrentProjectWhenSpecified {
		add(pr)
	}
	for _, r := range opt.PreferredRoots {
		r = strings.TrimSpace(r)
		switch roleByRoot[r] {
		case MemberRoleCurrentProject, MemberRoleRelatedProject:
			add(r)
		}
	}

	if pol.IncludeRelatedProjectsForPatterns {
		var related []RootGroupMember
		for _, m := range present {
			if m.Role == MemberRoleRelatedProject {
				related = append(related, m)
			}
		}
		sort.SliceStable(related, func(i, j int) bool {
			if related[i].Priority != related[j].Priority {
				return related[i].Priority > related[j].Priority
			}
			return related[i].RootName < related[j].RootName
		})
		n := pol.MaxRelatedProjects
		if n < 0 {
			n = 0
		}
		for i := 0; i < len(related) && i < n; i++ {
			add(related[i].RootName)
		}
	}

	// If nothing selected (roles missing on all members), fall back to all present members.
	if len(out) == 0 {
		for _, m := range present {
			add(m.RootName)
		}
	}
	return out, nil
}

func parsePoliciesJSON(s string) KnowledgeGroupPolicies {
	def := DefaultKnowledgeGroupPolicies()
	s = strings.TrimSpace(s)
	if s == "" || s == "{}" {
		return def
	}
	var p KnowledgeGroupPolicies
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return def
	}
	// Detect completely empty decode vs intentional false: if JSON had keys, use as-is with Normalize.
	if !json.Valid([]byte(s)) {
		return def
	}
	// If unmarshaled all-false and max 0, and input was "{}", already handled.
	// Partial YAML often sets only some fields — merge with defaults for unset booleans is hard.
	// Convention: config loader always sends full defaults; DB stores full JSON.
	p = p.Normalize()
	// If MaxRelatedProjects defaulted but AllowCross was false in JSON intentionally, keep.
	return p
}

func validMemberRole(role string) bool {
	switch role {
	case MemberRoleOfficialDocs, MemberRoleOfficialExample, MemberRoleCurrentProject,
		MemberRoleRelatedProject, MemberRoleCuratedKnowledge:
		return true
	default:
		return false
	}
}

func slugKnowledgeGroupID(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}

func groupKey(g *RootGroup) string {
	if g == nil {
		return ""
	}
	if g.ID != "" {
		return g.ID
	}
	return g.Name
}

// ExpandAliases returns canonical forms for a query token (bounded).
func (s *Store) ExpandAliases(ctx context.Context, token, technology string, limit int) ([]string, error) {
	token = strings.TrimSpace(strings.ToLower(token))
	if token == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	q := `SELECT DISTINCT canonical FROM aliases WHERE lower(alias)=?`
	args := []any{token}
	if technology != "" {
		q += ` AND (technology='' OR technology=?)`
		args = append(args, technology)
	}
	q += ` LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpsertAlias stores a controlled alias mapping.
func (s *Store) UpsertAlias(ctx context.Context, canonical, alias, technology, rootName string, confidence float64) error {
	if strings.TrimSpace(canonical) == "" || strings.TrimSpace(alias) == "" {
		return fmt.Errorf("canonical and alias are required")
	}
	if confidence <= 0 {
		confidence = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO aliases(canonical, alias, technology, root_name, confidence)
		VALUES(?,?,?,?,?)
		ON CONFLICT(alias, technology, root_name) DO UPDATE SET
			canonical=excluded.canonical, confidence=excluded.confidence`,
		canonical, alias, technology, rootName, confidence)
	return err
}
