// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Command evaltasks runs a sanitized coding-task retrieval evaluation harness.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"implcache-mcp/implctx"
	"implcache-mcp/store"

	"gopkg.in/yaml.v3"
)

type taskCase struct {
	ID               string   `yaml:"id" json:"id"`
	Task             string   `yaml:"task" json:"task"`
	Language         string   `yaml:"language" json:"language"`
	Technology       string   `yaml:"technology" json:"technology"`
	PreferredRoots   []string `yaml:"preferred_roots" json:"preferredRoots"`
	ExpectedSymbols  []string `yaml:"expected_symbols" json:"expectedSymbols"`
	ExpectedSources  []string `yaml:"expected_sources" json:"expectedSources"`
	ForbiddenSymbols []string `yaml:"forbidden_symbols" json:"forbiddenSymbols"`
	MaxContextTokens int      `yaml:"max_context_tokens" json:"maxContextTokens"`
}

type taskFile struct {
	Tasks []taskCase `yaml:"tasks"`
}

func main() {
	dbPath := flag.String("db", "", "sqlite db (required unless -seed-demo)")
	tasksPath := flag.String("tasks", "", "YAML task file (default: embedded demo tasks)")
	seedDemo := flag.Bool("seed-demo", false, "create a temporary demo corpus and evaluate against it")
	flag.Parse()

	ctx := context.Background()
	var (
		st      *store.Store
		err     error
		tmpDB   string
		cleanup func()
	)
	if *seedDemo {
		dir, err := os.MkdirTemp("", "implcache-eval-*")
		if err != nil {
			fatal(err)
		}
		tmpDB = filepath.Join(dir, "eval.db")
		st, err = store.Open(tmpDB)
		if err != nil {
			fatal(err)
		}
		if err := seedDemoCorpus(ctx, st); err != nil {
			fatal(err)
		}
		cleanup = func() {
			st.Close()
			_ = os.RemoveAll(dir)
		}
		defer cleanup()
		fmt.Fprintf(os.Stderr, "seeded demo db at %s\n", tmpDB)
	} else {
		if strings.TrimSpace(*dbPath) == "" {
			fatal(fmt.Errorf("-db is required (or pass -seed-demo)"))
		}
		st, err = store.Open(*dbPath)
		if err != nil {
			fatal(err)
		}
		defer st.Close()
	}

	cases, err := loadTasks(*tasksPath)
	if err != nil {
		fatal(err)
	}

	type row struct {
		ID                 string `json:"id"`
		Coverage           string `json:"coverage"`
		EstimatedTokens    int    `json:"estimatedTokens"`
		Top1Symbol         bool   `json:"top1Symbol"`
		Top3Symbol         bool   `json:"top3Symbol"`
		ExpectedSourceHit  bool   `json:"expectedSourceHit"`
		ForbiddenHit       bool   `json:"forbiddenHit"`
		DuplicateExcerpts  int    `json:"duplicateExcerpts"`
		ContextFingerprint string `json:"contextFingerprint"`
		LatencyMS          int64  `json:"latencyMs"`
		Error              string `json:"error,omitempty"`
	}

	var rows []row
	var top1, top3, srcHit, srcDeclared, n int
	var tokenSum int
	var latencies []int64
	for _, tc := range cases {
		n++
		start := time.Now()
		res, err := implctx.Get(ctx, st, implctx.Request{
			Task:             tc.Task,
			Language:         tc.Language,
			Technology:       tc.Technology,
			PreferredRoots:   tc.PreferredRoots,
			MaxContextTokens: tc.MaxContextTokens,
		})
		r := row{ID: tc.ID, LatencyMS: time.Since(start).Milliseconds()}
		latencies = append(latencies, r.LatencyMS)
		if err != nil {
			r.Error = err.Error()
			rows = append(rows, r)
			continue
		}
		r.Coverage = res.Coverage
		r.EstimatedTokens = res.EstimatedTokens
		r.ContextFingerprint = res.ContextFingerprint
		tokenSum += res.EstimatedTokens

		syms := append([]string{}, res.RequiredAPIs...)
		for _, s := range res.RelevantSymbols {
			syms = append(syms, s.Name, s.UnqualifiedName)
		}
		r.Top1Symbol = symbolIn(syms, tc.ExpectedSymbols, 1)
		r.Top3Symbol = symbolIn(syms, tc.ExpectedSymbols, 3)
		if r.Top1Symbol {
			top1++
		}
		if r.Top3Symbol {
			top3++
		}
		blob := strings.ToLower(strings.Join(syms, " ") + " " + res.Summary)
		for _, bad := range tc.ForbiddenSymbols {
			if strings.Contains(blob, strings.ToLower(bad)) {
				r.ForbiddenHit = true
			}
		}
		if len(tc.ExpectedSources) > 0 {
			srcDeclared++
			uris := collectResponseURIs(res)
			if sourceHit(uris, tc.ExpectedSources) {
				r.ExpectedSourceHit = true
				srcHit++
			}
		}
		seen := map[string]int{}
		for _, e := range res.Examples {
			seen[e.Excerpt]++
			if seen[e.Excerpt] > 1 {
				r.DuplicateExcerpts++
			}
		}
		rows = append(rows, r)
	}

	summary := map[string]any{
		"taskCount":            n,
		"top1SymbolRecall":     ratio(top1, n),
		"top3SymbolRecall":     ratio(top3, n),
		"expectedSourceTasks":  srcDeclared,
		"expectedSourceRecall": ratio(srcHit, srcDeclared),
		"avgEstimatedTokens":   avgInt(tokenSum, n),
		"medianLatencyMs":      percentile(latencies, 0.5),
		"p95LatencyMs":         percentile(latencies, 0.95),
		"tasks":                rows,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(summary)
}

func loadTasks(path string) ([]taskCase, error) {
	candidates := []string{}
	if strings.TrimSpace(path) != "" {
		candidates = append(candidates, path)
	}
	candidates = append(candidates,
		filepath.Join("testdata", "eval", "tasks.yaml"),
		filepath.Join("..", "..", "testdata", "eval", "tasks.yaml"),
	)
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var tf taskFile
		if err := yaml.Unmarshal(data, &tf); err != nil {
			return nil, err
		}
		if len(tf.Tasks) == 0 {
			return nil, fmt.Errorf("no tasks in %s", p)
		}
		return tf.Tasks, nil
	}
	return nil, fmt.Errorf("task file not found (pass -tasks or use testdata/eval/tasks.yaml)")
}

func seedDemoCorpus(ctx context.Context, st *store.Store) error {
	docs := []store.UpsertInput{
		{
			URI: "project://example-plugin-app/src/commands.cpp", Title: "commands",
			SourceType: store.SourceSource, Path: "src/commands.cpp", RootName: "example-plugin-app",
			Authority: store.AuthorityCurrentProject, Hash: "p1",
			Chunks: chunkBody("RegisterCommand then AddMenuItem in plugin init"),
			Symbols: []store.SymbolInput{
				{Name: "RegisterCommand", Kind: "function", Language: "cpp", StartLine: 10, EndLine: 20},
				{Name: "AddMenuItem", Kind: "function", Language: "cpp", StartLine: 30, EndLine: 40},
			},
		},
		{
			URI: "project://example-plugin-sdk/api.md", Title: "Plugin API",
			SourceType: store.SourceMarkdown, Path: "api.md", RootName: "example-plugin-sdk",
			Authority: store.AuthorityOfficialDocs, Hash: "p2",
			Chunks: chunkBody("RegisterCommand(name, handler) AddMenuItem(menu, command)"),
			Symbols: []store.SymbolInput{
				{Name: "RegisterCommand", Kind: "api", Language: "cpp"},
				{Name: "AddMenuItem", Kind: "api", Language: "cpp"},
			},
		},
		{
			URI: "project://example-network-sdk/retry.h", Title: "retry",
			SourceType: store.SourceSource, Path: "retry.h", RootName: "example-network-sdk",
			Authority: store.AuthorityOfficialDocs, Hash: "n1",
			Chunks: chunkBody("RetryPolicy Connect Disconnect reconnect backoff"),
			Symbols: []store.SymbolInput{
				{Name: "RetryPolicy", Kind: "type", Language: "cpp"},
				{Name: "Client.Connect", Kind: "function", Language: "cpp"},
				{Name: "Disconnect", Kind: "function", Language: "cpp"},
			},
		},
		{
			URI: "project://example-network-service/client.cpp", Title: "client",
			SourceType: store.SourceSource, Path: "client.cpp", RootName: "example-network-service",
			Authority: store.AuthorityCurrentProject, Hash: "n2",
			Chunks: chunkBody("network client uses Connect and RetryPolicy"),
			Symbols: []store.SymbolInput{
				{Name: "Connect", Kind: "function", Language: "cpp"},
			},
		},
		{
			URI: "project://example-database-tool/migrate.go", Title: "migrate",
			SourceType: store.SourceSource, Path: "migrate.go", RootName: "example-database-tool",
			Authority: store.AuthorityCurrentProject, Hash: "d1",
			Chunks: chunkBody("PRAGMA user_version BeginTx schema migration"),
			Symbols: []store.SymbolInput{
				{Name: "BeginTx", Kind: "function", Language: "go"},
				{Name: "user_version", Kind: "api", Language: "sql"},
			},
		},
		{
			URI: "project://demo-embedded-project/drivers/gpio.cpp", Title: "gpio",
			SourceType: store.SourceSource, Path: "drivers/gpio.cpp", RootName: "demo-embedded-project",
			Authority: store.AuthorityCurrentProject, Hash: "e1",
			Chunks: chunkBody("SpiTransfer ConfigurePin GPIO expander"),
			Symbols: []store.SymbolInput{
				{Name: "SpiTransfer", Kind: "function", Language: "cpp"},
				{Name: "ConfigurePin", Kind: "function", Language: "cpp"},
			},
		},
		{
			URI: "project://example-device-sdk/spi.md", Title: "SPI",
			SourceType: store.SourceMarkdown, Path: "spi.md", RootName: "example-device-sdk",
			Authority: store.AuthorityOfficialDocs, Hash: "e2",
			Chunks: chunkBody("SpiTransfer bytes ConfigurePin mode"),
			Symbols: []store.SymbolInput{
				{Name: "SpiTransfer", Kind: "api", Language: "cpp"},
			},
		},
		{
			URI: "project://example-mcp-server/main.go", Title: "server",
			SourceType: store.SourceSource, Path: "main.go", RootName: "example-mcp-server",
			Authority: store.AuthorityCurrentProject, Hash: "m1",
			Chunks: chunkBody("mcp.AddTool returns CallToolResult"),
			Symbols: []store.SymbolInput{
				{Name: "AddTool", Kind: "function", Language: "go"},
				{Name: "CallToolResult", Kind: "type", Language: "go"},
			},
		},
		docSyms("example-device-sdk", "session.h", "OpenSession CloseSession device session",
			[]store.SymbolInput{{Name: "OpenSession", Kind: "function"}, {Name: "CloseSession", Kind: "function"}}),
		docSyms("example-control-app", "app.cpp", "control app OpenSession logging workers",
			[]store.SymbolInput{{Name: "OpenSession", Kind: "function"}}),
		docSyms("example-logging-sdk", "log.go", "SetLogLevel AttachSink structured logging",
			[]store.SymbolInput{{Name: "SetLogLevel", Kind: "function"}, {Name: "AttachSink", Kind: "function"}}),
		docSyms("example-config-sdk", "config.go", "LoadConfig ValidateSchema application config",
			[]store.SymbolInput{{Name: "LoadConfig", Kind: "function"}, {Name: "ValidateSchema", Kind: "function"}}),
		docSyms("example-concurrency-sdk", "pool.h", "SubmitJob ShutdownPool worker pool",
			[]store.SymbolInput{{Name: "SubmitJob", Kind: "function"}, {Name: "ShutdownPool", Kind: "function"}}),
		docSyms("example-http-sdk", "http.go", "HandlePath WriteJSON bind route",
			[]store.SymbolInput{{Name: "HandlePath", Kind: "function"}, {Name: "WriteJSON", Kind: "function"}}),
		docSyms("example-http-service", "routes.go", "service HandlePath WriteJSON",
			[]store.SymbolInput{{Name: "HandlePath", Kind: "function"}}),
		docSyms("example-cache-sdk", "cache.h", "PutEntry GetEntry persist cache",
			[]store.SymbolInput{{Name: "PutEntry", Kind: "function"}, {Name: "GetEntry", Kind: "function"}}),
		docSyms("example-protocol-sdk", "frame.h", "DecodeFrame EncodeFrame protocol frame",
			[]store.SymbolInput{{Name: "DecodeFrame", Kind: "function"}, {Name: "EncodeFrame", Kind: "function"}}),
	}
	for _, d := range docs {
		if _, err := st.UpsertDocument(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

func chunkBody(body string) []store.Chunk {
	return []store.Chunk{{Heading: "Overview", Body: body, StartLine: 1, EndLine: 20}}
}

func docSyms(root, path, body string, syms []store.SymbolInput) store.UpsertInput {
	for i := range syms {
		if syms[i].Language == "" {
			syms[i].Language = "cpp"
		}
		if syms[i].StartLine == 0 {
			syms[i].StartLine = i + 1
		}
	}
	return store.UpsertInput{
		URI: "project://" + root + "/" + path, Title: path,
		SourceType: store.SourceSource, Path: path, RootName: root,
		Authority: store.AuthorityOfficialDocs, Hash: root + "-" + path,
		Chunks: chunkBody(body), Symbols: syms,
	}
}

func collectResponseURIs(res *implctx.Response) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(u string) {
		u = strings.TrimSpace(u)
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	for _, c := range res.Citations {
		add(c.URI)
	}
	for _, e := range res.Examples {
		add(e.URI)
	}
	for _, s := range res.RelevantSymbols {
		add(s.URI)
	}
	return out
}

func sourceHit(have, want []string) bool {
	if len(want) == 0 {
		return true
	}
	for _, w := range want {
		for _, h := range have {
			if h == w {
				return true
			}
		}
	}
	return false
}

func symbolIn(have, want []string, topN int) bool {
	if len(want) == 0 {
		return true
	}
	if topN > len(have) {
		topN = len(have)
	}
	head := have[:topN]
	for _, w := range want {
		wl := strings.ToLower(w)
		for _, h := range head {
			if strings.EqualFold(h, w) || strings.HasSuffix(strings.ToLower(h), "."+wl) || strings.HasSuffix(strings.ToLower(h), "::"+wl) {
				return true
			}
		}
	}
	return false
}

func ratio(hit, n int) float64 {
	if n == 0 {
		return 0
	}
	return float64(hit) / float64(n)
}

func avgInt(sum, n int) float64 {
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n)
}

func percentile(xs []int64, p float64) int64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]int64{}, xs...)
	for i := 0; i < len(cp); i++ {
		for j := i + 1; j < len(cp); j++ {
			if cp[j] < cp[i] {
				cp[i], cp[j] = cp[j], cp[i]
			}
		}
	}
	idx := int(float64(len(cp)-1) * p)
	return cp[idx]
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
