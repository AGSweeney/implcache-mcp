// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
// Command evaltasks runs a small retrieval evaluation harness for coding tasks.
// It measures context size and symbol recall against a local implcache.db.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"implcache-mcp/implctx"
	"implcache-mcp/store"
)

type taskCase struct {
	Name           string   `json:"name"`
	Task           string   `json:"task"`
	Language       string   `json:"language"`
	Technology     string   `json:"technology"`
	PreferredRoots []string `json:"preferredRoots"`
	ExpectedAPIs   []string `json:"expectedApis"`
	MaxTokens      int      `json:"maxTokens"`
}

func main() {
	dbPath := flag.String("db", "./implcache.db", "sqlite db")
	flag.Parse()

	cases := []taskCase{
		{
			Name: "toolkit-menubar", Task: "user_initialize ProCmdActionAdd menubar pushbutton",
			Language: "c", Technology: "Creo TOOLKIT",
			ExpectedAPIs: []string{"ProCmdActionAdd", "ProMenubarmenuPushbuttonAdd"},
			MaxTokens:    2500,
		},
		{
			Name: "sqlite-migrate", Task: "PRAGMA user_version schema migration",
			Language: "go", Technology: "SQLite",
			ExpectedAPIs: []string{"user_version"},
			MaxTokens:    2000,
		},
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	type row struct {
		Name            string  `json:"name"`
		Coverage        string  `json:"coverage"`
		EstimatedTokens int     `json:"estimatedTokens"`
		APIRecall       float64 `json:"apiRecall"`
		Citations       int     `json:"citations"`
		WebSearch       bool    `json:"webSearchRecommended"`
		Error           string  `json:"error,omitempty"`
	}
	var rows []row
	for _, tc := range cases {
		res, err := implctx.Get(ctx, st, implctx.Request{
			Task:             tc.Task,
			Language:         tc.Language,
			Technology:       tc.Technology,
			PreferredRoots:   tc.PreferredRoots,
			MaxContextTokens: tc.MaxTokens,
		})
		r := row{Name: tc.Name}
		if err != nil {
			r.Error = err.Error()
			rows = append(rows, r)
			continue
		}
		r.Coverage = res.Coverage
		r.EstimatedTokens = res.EstimatedTokens
		r.Citations = len(res.Citations)
		r.WebSearch = res.WebSearchRecommended
		hit := 0
		blob := strings.ToLower(strings.Join(res.RequiredAPIs, " ") + " " + res.Summary)
		for _, api := range tc.ExpectedAPIs {
			if strings.Contains(blob, strings.ToLower(api)) {
				hit++
			}
		}
		if len(tc.ExpectedAPIs) > 0 {
			r.APIRecall = float64(hit) / float64(len(tc.ExpectedAPIs))
		}
		rows = append(rows, r)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rows)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
