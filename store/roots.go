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
	// Roots to search when NeedsChoice is false.
	Roots []string `json:"roots,omitempty"`
	// NeedsChoice means the caller should ask the user to pick a rootName.
	NeedsChoice bool `json:"needsChoice"`
	// Message is a human/agent-facing prompt when NeedsChoice is true.
	Message string `json:"message,omitempty"`
	// AvailableRoots are distinct root_name values present in the DB.
	AvailableRoots []string `json:"availableRoots,omitempty"`
	// MatchedHints explains which context cues fired (debug / UI).
	MatchedHints []string `json:"matchedHints,omitempty"`
}

// rootAlias maps a lowercase cue to preferred root names (intersected with DB).
// More specific cues should win when we score.
var rootAliases = []struct {
	cue    string
	roots  []string
	weight int
}{
	// Explicit root ids
	{"ccw_help", []string{"ccw_help"}, 100},
	{"creo_toolkit_help", []string{"creo_toolkit_help"}, 100},
	{"otk_cpp_doc", []string{"otk_cpp_doc"}, 100},
	{"project://ccw_help", []string{"ccw_help"}, 100},
	{"project://creo_toolkit_help", []string{"creo_toolkit_help"}, 100},
	{"project://otk_cpp_doc", []string{"otk_cpp_doc"}, 100},
	{"project://otk/", []string{"otk"}, 100},

	// CCW / Rockwell
	{"connected components workbench", []string{"ccw_help"}, 90},
	{"connected components", []string{"ccw_help"}, 80},
	{"micro800", []string{"ccw_help"}, 90},
	{"panelview", []string{"ccw_help"}, 70},
	{"rockwell", []string{"ccw_help"}, 60},
	{"ccw", []string{"ccw_help"}, 85},

	// Creo TOOLKIT (C)
	{"user_initialize", []string{"creo_toolkit_help"}, 90},
	{"user_terminate", []string{"creo_toolkit_help"}, 80},
	{"promenubar", []string{"creo_toolkit_help"}, 90},
	{"procmdaction", []string{"creo_toolkit_help"}, 85},
	{"protoolkit", []string{"creo_toolkit_help"}, 90},
	{"creotk.dat", []string{"creo_toolkit_help"}, 90},
	{"protk.dat", []string{"creo_toolkit_help"}, 80},
	{"creo toolkit", []string{"creo_toolkit_help"}, 85},
	{"pro/toolkit", []string{"creo_toolkit_help"}, 85},

	// OTK
	{"object toolkit", []string{"otk_cpp_doc", "otk"}, 90},
	{"otk c++", []string{"otk_cpp_doc", "otk"}, 90},
	{"otk", []string{"otk_cpp_doc", "otk"}, 80},
	{"pfcsession", []string{"otk_cpp_doc", "otk"}, 90},
	{"uicreatecommand", []string{"otk_cpp_doc", "otk"}, 85},
	{"wfc", []string{"otk_cpp_doc", "otk"}, 60},
	{"pfc", []string{"otk_cpp_doc", "otk"}, 50},
}

// family of a root for conflict detection.
func rootFamily(root string) string {
	switch strings.ToLower(root) {
	case "ccw_help":
		return "ccw"
	case "creo_toolkit_help":
		return "creo_toolkit"
	case "otk", "otk_cpp_doc":
		return "otk"
	default:
		return "other:" + strings.ToLower(root)
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
// query/subject context. When the space is ambiguous, NeedsChoice is set and
// the caller should prompt the user (do not search all roots).
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

	// Explicit wins (validated).
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
	// CCW vs Creo/OTK is a hard conflict — ask.
	_, hasCCW := families["ccw"]
	_, hasCreo := families["creo_toolkit"]
	_, hasOTK := families["otk"]
	if hasCCW && (hasCreo || hasOTK) {
		inf.NeedsChoice = true
		inf.Message = formatRootPrompt(
			"Query matches both Rockwell CCW and Creo/OTK cues — pick a root.",
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
	// Also: bare root name token in the query.
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
	// Keep roots within 40% of the top score (same family cluster).
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
	b.WriteString("\nRe-run with rootName set to one of the above (e.g. rootName=\"ccw_help\").")
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
