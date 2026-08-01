// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"implcache-mcp/store"
)

func main() {
	dbPath := flag.String("db", "./implcache.db", "sqlite db path")
	configPath := flag.String("config", "config/knowledge-groups.yaml", "knowledge-groups YAML")
	list := flag.Bool("list", false, "list groups after apply (or instead of apply with -list-only)")
	listOnly := flag.Bool("list-only", false, "only list; do not apply config")
	jsonOut := flag.Bool("json", false, "JSON output")
	flag.Parse()

	ctx := context.Background()
	st, err := store.Open(*dbPath)
	if err != nil {
		fatalf("open: %v", err)
	}
	defer st.Close()

	if !*listOnly {
		applied, err := st.ApplyRootGroupsFile(ctx, *configPath)
		if err != nil {
			fatalf("apply: %v", err)
		}
		if !*jsonOut {
			fmt.Fprintf(os.Stderr, "applied groups from %s: %v\n", *configPath, applied)
		}
	}
	if *list || *listOnly || *jsonOut {
		groups, err := st.ListRootGroups(ctx)
		if err != nil {
			fatalf("list: %v", err)
		}
		if *jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"database": *dbPath, "groups": groups})
			return
		}
		for _, g := range groups {
			fmt.Printf("%s — %s\n", g.Name, g.Description)
			for _, m := range g.Members {
				fmt.Printf("  [%d] %s\n", m.Priority, m.RootName)
			}
		}
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
