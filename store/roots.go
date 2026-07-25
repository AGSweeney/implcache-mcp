// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// RootInference is the result of resolving which knowledge root(s) to search.
type RootInference struct {
	Roots          []string `json:"roots,omitempty"`
	NeedsChoice    bool     `json:"needsChoice"`
	Message        string   `json:"message,omitempty"`
	AvailableRoots []string `json:"availableRoots,omitempty"`
	MatchedHints   []string `json:"matchedHints,omitempty"`
}

// rootAlias maps a lowercase cue to preferred root names (intersected with DB).
// Sanitized demo corpora only — add production aliases via the aliases table.
var rootAliases = []struct {
	cue    string
	roots  []string
	weight int
}{
	{"example-device-sdk", []string{"example-device-sdk"}, 100},
	{"example-control-app", []string{"example-control-app"}, 100},
	{"example-plugin-sdk", []string{"example-plugin-sdk"}, 100},
	{"example-network-sdk", []string{"example-network-sdk"}, 100},
	{"demo-embedded-project", []string{"demo-embedded-project"}, 100},
	{"example-database-tool", []string{"example-database-tool"}, 100},
	{"sqlite-reference", []string{"sqlite-reference"}, 100},

	{"device sdk", []string{"example-device-sdk"}, 80},
	{"gpio expander", []string{"example-device-sdk", "demo-embedded-project"}, 85},
	{"spitransfer", []string{"example-device-sdk"}, 80},
	{"configurepin", []string{"example-device-sdk"}, 75},

	{"control app", []string{"example-control-app"}, 80},
	{"plugin sdk", []string{"example-plugin-sdk"}, 80},
	{"registercommand", []string{"example-plugin-sdk", "example-control-app"}, 85},
	{"addmenuitem", []string{"example-plugin-sdk"}, 80},

	{"network sdk", []string{"example-network-sdk"}, 80},
	{"retrypolicy", []string{"example-network-sdk"}, 85},
	{"reconnect", []string{"example-network-sdk"}, 70},

	{"sqlite", []string{"example-database-tool", "sqlite-reference"}, 70},
	{"user_version", []string{"example-database-tool"}, 80},
	{"schema migration", []string{"example-database-tool"}, 75},
}

func rootFamily(root string) string {
	r := strings.ToLower(root)
	switch {
	case strings.Contains(r, "device"):
		return "device"
	case strings.Contains(r, "plugin"):
		return "plugin"
	case strings.Contains(r, "network"):
		return "network"
	case strings.Contains(r, "database") || strings.Contains(r, "sqlite"):
		return "database"
	case strings.Contains(r, "embedded"):
		return "embedded"
	case strings.Contains(r, "control"):
		return "control"
	default:
		return "other:" + r
	}
}

// ListRootNames returns distinct non-empty root_name values in the DB.
func (s *Store) ListRootNames(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT root_name FROM documents
		WHERE root_name IS NOT NULL AND root_name != ''
		ORDER BY root_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ResolveRoots picks knowledge roots from an optional explicit list and/or
// query/subject context. When the space is ambiguous, NeedsChoice is set.
func (s *Store) ResolveRoots(ctx context.Context, query string, explicit []string) (RootInference, error) {
	available, err := s.ListRootNames(ctx)
	if err != nil {
		return RootInference{}, err
	}
	availSet := map[string]struct{}{}
	for _, r := range available {
		availSet[r] = struct{}{}
	}

	inf := RootInference{AvailableRoots: available}

	var explicitClean []string
	for _, r := range explicit {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if _, ok := availSet[r]; !ok {
			return RootInference{
				NeedsChoice:    true,
				AvailableRoots: available,
				Message: fmt.Sprintf(
					"Unknown rootName %q. Choose one of: %s",
					r, strings.Join(available, ", ")),
			}, nil
		}
		explicitClean = append(explicitClean, r)
	}
	if len(explicitClean) > 0 {
		inf.Roots = uniqueSorted(explicitClean)
		inf.MatchedHints = []string{"explicit rootName"}
		return inf, nil
	}

	if len(available) == 0 {
		inf.NeedsChoice = true
		inf.Message = "No knowledge roots are ingested yet. Ingest a corpus with rootName first."
		return inf, nil
	}
	if len(available) == 1 {
		inf.Roots = available
		inf.MatchedHints = []string{"only one root in database"}
		return inf, nil
	}

	inferred, hints := inferRootsFromText(query, availSet)
	inf.MatchedHints = hints

	if len(inferred) == 0 {
		inf.NeedsChoice = true
		inf.Message = formatRootPrompt(
			"Could not infer a knowledge root from the query/subject.",
			query, available)
		return inf, nil
	}

	families := map[string]struct{}{}
	for _, r := range inferred {
		families[rootFamily(r)] = struct{}{}
	}
	if len(families) > 1 {
		inf.NeedsChoice = true
		inf.Message = formatRootPrompt(
			"Query matches multiple product families — pick a root.",
			query, available)
		inf.MatchedHints = hints
		return inf, nil
	}

	inf.Roots = inferred
	return inf, nil
}

func inferRootsFromText(query string, avail map[string]struct{}) (roots []string, hints []string) {
	q := strings.ToLower(query)
	scores := map[string]int{}
	for _, a := range rootAliases {
		if !strings.Contains(q, a.cue) {
			continue
		}
		hints = append(hints, a.cue)
		for _, r := range a.roots {
			if _, ok := avail[r]; ok {
				scores[r] += a.weight
			}
		}
	}
	for r := range avail {
		rl := strings.ToLower(r)
		if rl != "" && strings.Contains(q, rl) {
			scores[r] += 100
			hints = append(hints, "root:"+r)
		}
	}
	if len(scores) == 0 {
		return nil, hints
	}
	type kv struct {
		k string
		v int
	}
	list := make([]kv, 0, len(scores))
	for k, v := range scores {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].v == list[j].v {
			return list[i].k < list[j].k
		}
		return list[i].v > list[j].v
	})
	top := list[0].v
	for _, item := range list {
		if item.v*100 >= top*60 {
			roots = append(roots, item.k)
		}
	}
	return uniqueSorted(roots), uniqueSorted(hints)
}

func formatRootPrompt(lead, query string, available []string) string {
	var b strings.Builder
	b.WriteString(lead)
	b.WriteString("\n\n")
	if strings.TrimSpace(query) != "" {
		b.WriteString("Query/subject: ")
		b.WriteString(query)
		b.WriteString("\n\n")
	}
	b.WriteString("Available knowledge roots:\n")
	for _, r := range available {
		b.WriteString("  - ")
		b.WriteString(r)
		b.WriteString("\n")
	}
	b.WriteString("\nRe-run with rootName set to one of the above (e.g. rootName=\"example-device-sdk\").")
	return b.String()
}

func uniqueSorted(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// ErrNeedsRoot is returned by helpers that refuse to search without a root.
type ErrNeedsRoot struct {
	Inference RootInference
}

func (e *ErrNeedsRoot) Error() string {
	if e.Inference.Message != "" {
		return e.Inference.Message
	}
	return "knowledge root is ambiguous; specify rootName"
}
