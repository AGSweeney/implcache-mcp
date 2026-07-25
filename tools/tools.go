// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"implcache-mcp/implctx"
	"implcache-mcp/ingest"
	"implcache-mcp/store"
	"implcache-mcp/vomit"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AgentTools are registered in agent (default) mode.
var AgentTools = []string{
	"get_implementation_context",
	"find_symbol",
	"search_knowledge",
	"get_document",
	"list_roots",
}

// AdminOnlyTools are registered only in admin mode (or -enable-admin-tools).
var AdminOnlyTools = []string{
	"ingest_markdown",
	"ingest_project",
	"list_documents",
	"delete_document",
	"delete_by_uri_prefix",
	"vomit",
}

// PlannedTools returns the tool names that RegisterWithOptions would expose.
func PlannedTools(opt Options) []string {
	names := append([]string{}, AgentTools...)
	if opt.AdminEnabled() {
		names = append(names, AdminOnlyTools...)
	}
	return names
}

// Register adds tools with admin mode enabled (legacy helper for tests/CLIs).
func Register(server *mcp.Server, st *store.Store) []string {
	return RegisterWithOptions(server, st, Options{
		Mode:             ModeAdmin,
		AllowIngest:      true,
		AllowDelete:      true,
		AllowOutputWrite: true,
		OutputRoot:       "",
		MaxResults:       store.DefaultSearchLimit,
		MaxIngestFiles:   50000,
		MaxDocumentBytes: 8 << 20,
	})
}

// RegisterWithOptions registers tools with explicit safety/limit options.
// Agent mode (default) omits administrative schemas entirely.
func RegisterWithOptions(server *mcp.Server, st *store.Store, opt Options) []string {
	opt = opt.Normalize()
	if opt.MaxResults <= 0 {
		opt.MaxResults = store.DefaultSearchLimit
	}
	if opt.MaxIngestFiles <= 0 {
		opt.MaxIngestFiles = 50000
	}
	if opt.MaxDocumentBytes <= 0 {
		opt.MaxDocumentBytes = 8 << 20
	}

	deny := func(action string) error {
		return fmt.Errorf("%s disabled (read-only or permission flags)", action)
	}
	registered := PlannedTools(opt)

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_implementation_context",
		Description: "Primary tool for coding agents: return a compact, cited implementation package " +
			"(APIs, sequence, examples, constraints, pitfalls) within a token budget. " +
			"Prefer this over dumping search hits or full documents.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args implContextArgs) (*mcp.CallToolResult, implctx.Response, error) {
		projectRoot := args.ProjectRoot
		if strings.TrimSpace(projectRoot) == "" {
			projectRoot = opt.DefaultProjectRoot
		}
		preferred := args.PreferredRoots
		if len(preferred) == 0 {
			preferred = append([]string{}, opt.DefaultPreferredRoots...)
		}
		res, err := implctx.Get(ctx, st, implctx.Request{
			Task:             args.Task,
			Language:         args.Language,
			Technology:       args.Technology,
			ProjectRoot:      projectRoot,
			PreferredRoots:   preferred,
			RootGroup:        args.RootGroup,
			MaxContextTokens: args.MaxContextTokens,
			Semantic:         opt.EnableSemantic || args.Semantic,
		})
		if err != nil {
			var need *store.ErrNeedsRoot
			if asNeedsRoot(err, &need) {
				payload, _ := json.MarshalIndent(need.Inference, "", "  ")
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}},
					IsError: true,
				}, implctx.Response{}, nil
			}
			return nil, implctx.Response{}, err
		}
		payload, _ := json.MarshalIndent(res, "", "  ")
		return textResult(string(payload)), *res, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "find_symbol",
		Description: "Symbol lookup with staged exact/normalized/qualified/prefix/suffix matching within preferred roots",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args findSymbolArgs) (*mcp.CallToolResult, findSymbolResult, error) {
		var roots []string
		if r := strings.TrimSpace(args.RootName); r != "" {
			roots = []string{r}
		} else if len(args.PreferredRoots) > 0 {
			roots = args.PreferredRoots
		} else if len(opt.DefaultPreferredRoots) > 0 {
			roots = opt.DefaultPreferredRoots
		}
		limit := store.ClampSearchLimit(args.Limit, opt.MaxResults)
		syms, err := st.FindSymbols(ctx, args.Name, roots, limit)
		if err != nil {
			return nil, findSymbolResult{}, err
		}
		out := findSymbolResult{Symbols: syms, Count: len(syms)}
		payload, _ := json.MarshalIndent(out, "", "  ")
		return textResult(string(payload)), out, nil
	})

	if !opt.AdminEnabled() {
		// Agent retrieval tools continue below; admin tools are not registered.
	} else {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "ingest_markdown",
			Description: "Ingest Markdown or HTML (HTML→Markdown) from a file or directory using portable project://{rootName}/{rel} URIs",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args ingestMarkdownArgs) (*mcp.CallToolResult, ingest.MarkdownResult, error) {
			if !opt.AllowIngest {
				return nil, ingest.MarkdownResult{}, deny("ingest")
			}
			res, err := ingest.IngestMarkdownOpts(ctx, st, ingest.MarkdownOptions{
				Path:             args.Path,
				Recursive:        args.Recursive,
				RootName:         args.RootName,
				MaxFiles:         opt.MaxIngestFiles,
				MaxDocumentBytes: opt.MaxDocumentBytes,
			})
			if err != nil {
				return nil, ingest.MarkdownResult{}, err
			}
			return textResult(fmt.Sprintf("root=%s ingested=%d skipped=%d errors=%d", res.RootName, res.Ingested, res.Skipped, len(res.Errors))), *res, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "ingest_project",
			Description: "Walk a project source tree and ingest text-like files (project:// URIs)",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args ingestProjectArgs) (*mcp.CallToolResult, ingest.ProjectResult, error) {
			if !opt.AllowIngest {
				return nil, ingest.ProjectResult{}, deny("ingest")
			}
			res, err := ingest.IngestProjectOpts(ctx, st, ingest.ProjectOptions{
				Path:             args.Path,
				RootName:         args.RootName,
				MaxFiles:         opt.MaxIngestFiles,
				MaxDocumentBytes: opt.MaxDocumentBytes,
			})
			if err != nil {
				return nil, ingest.ProjectResult{}, err
			}
			return textResult(fmt.Sprintf("root=%s ingested=%d skipped=%d errors=%d", res.RootName, res.Ingested, res.Skipped, len(res.Errors))), *res, nil
		})
	}

	mcp.AddTool(server, &mcp.Tool{
		Name: "search_knowledge",
		Description: "Full-text search the knowledge base (FTS5 with snippets). " +
			"Infers knowledge root from the query when possible; if ambiguous, returns needsChoice " +
			"with availableRoots — ask the user and re-run with rootName.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args searchArgs) (*mcp.CallToolResult, searchResult, error) {
		limit := store.ClampSearchLimit(args.Limit, opt.MaxResults)
		var explicit []string
		if r := strings.TrimSpace(args.RootName); r != "" {
			explicit = []string{r}
		}
		inf, err := st.ResolveRoots(ctx, args.Query, explicit)
		if err != nil {
			return nil, searchResult{}, err
		}
		if inf.NeedsChoice {
			out := searchResult{
				NeedsChoice:    true,
				Message:        inf.Message,
				AvailableRoots: inf.AvailableRoots,
				MatchedHints:   inf.MatchedHints,
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
				IsError: true,
			}, out, nil
		}
		hits, err := st.SearchOpts(ctx, store.SearchOptions{
			Query:    args.Query,
			Limit:    limit,
			Roots:    inf.Roots,
			Semantic: opt.EnableSemantic || args.Semantic,
		})
		if err != nil {
			return nil, searchResult{}, err
		}
		out := searchResult{
			Hits:         hits,
			Count:        len(hits),
			Roots:        inf.Roots,
			MatchedHints: inf.MatchedHints,
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return textResult(string(b)), out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_roots",
		Description: "List distinct knowledge rootName values currently in the database",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listRootsResult, error) {
		roots, err := st.ListRootNames(ctx)
		if err != nil {
			return nil, listRootsResult{}, err
		}
		out := listRootsResult{Roots: roots, Count: len(roots)}
		b, _ := json.MarshalIndent(out, "", "  ")
		return textResult(string(b)), out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_document",
		Description: "Fetch a document by uri (project://…) or numeric id",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getDocumentArgs) (*mcp.CallToolResult, documentResult, error) {
		var (
			doc    *store.Document
			chunks []store.Chunk
			err    error
		)
		switch {
		case args.ID > 0:
			doc, chunks, err = st.GetDocumentByID(ctx, args.ID)
		case strings.TrimSpace(args.URI) != "":
			doc, chunks, err = st.GetDocumentByURI(ctx, args.URI)
		default:
			return nil, documentResult{}, fmt.Errorf("uri or id is required")
		}
		if err != nil {
			return nil, documentResult{}, err
		}
		out := documentResult{Document: *doc, Chunks: chunks}
		if args.IncludeBody {
			var b strings.Builder
			for i, c := range chunks {
				if i > 0 {
					b.WriteString("\n\n")
				}
				if c.Heading != "" {
					b.WriteString("# ")
					b.WriteString(c.Heading)
					b.WriteString("\n\n")
				}
				b.WriteString(c.Body)
			}
			out.Body = b.String()
		}
		payload, _ := json.MarshalIndent(out, "", "  ")
		return textResult(string(payload)), out, nil
	})

	if opt.AdminEnabled() {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "delete_by_uri_prefix",
			Description: "Delete all documents whose URI starts with the given prefix (e.g. file:///)",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args deletePrefixArgs) (*mcp.CallToolResult, deletePrefixResult, error) {
			if !opt.AllowDelete {
				return nil, deletePrefixResult{}, deny("delete")
			}
			n, err := st.DeleteDocumentsByURIPrefix(ctx, args.Prefix)
			if err != nil {
				return nil, deletePrefixResult{}, err
			}
			out := deletePrefixResult{Deleted: n, Prefix: args.Prefix}
			return textResult(fmt.Sprintf("deleted=%d prefix=%s", n, args.Prefix)), out, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "list_documents",
			Description: "List ingested documents, optionally filtered by sourceType (markdown|source)",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args listDocumentsArgs) (*mcp.CallToolResult, listDocumentsResult, error) {
			docs, err := st.ListDocuments(ctx, args.SourceType)
			if err != nil {
				return nil, listDocumentsResult{}, err
			}
			out := listDocumentsResult{Documents: docs, Count: len(docs)}
			b, _ := json.MarshalIndent(out, "", "  ")
			return textResult(string(b)), out, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "delete_document",
			Description: "Delete a document and its chunks by uri",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args deleteDocumentArgs) (*mcp.CallToolResult, deleteDocumentResult, error) {
			if !opt.AllowDelete {
				return nil, deleteDocumentResult{}, deny("delete")
			}
			ok, err := st.DeleteDocument(ctx, args.URI)
			if err != nil {
				return nil, deleteDocumentResult{}, err
			}
			out := deleteDocumentResult{Deleted: ok, URI: args.URI}
			msg := "not found"
			if ok {
				msg = "deleted"
			}
			return textResult(msg), out, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name: "vomit",
			Description: "Compile a source-grounded implementation recipe/playbook from local knowledge. " +
				"Returns body to the agent; optionally writes under -output-root and/or saves as a generated knowledge_entry. " +
				"Generated recipes keep source lineage and are ranked below human-reviewed recipes.",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args vomitArgs) (*mcp.CallToolResult, vomit.Result, error) {
			if !opt.AllowOutputWrite && !args.ReturnBody && !args.SaveRecipe {
				return nil, vomit.Result{}, deny("vomit filesystem output")
			}
			var roots []string
			if r := strings.TrimSpace(args.RootName); r != "" {
				roots = []string{r}
			}
			res, err := vomit.Generate(ctx, st, vomit.Request{
				Subject:          args.Subject,
				OutPath:          args.OutPath,
				Limit:            args.Limit,
				MaxCharsPerDoc:   args.MaxCharsPerDoc,
				RootNames:        roots,
				OutputRoot:       opt.OutputRoot,
				AllowWrite:       opt.AllowOutputWrite,
				ReturnBody:       true, // always return recipe body to the agent
				MaxPlaybookBytes: 2 << 20,
				SaveRecipe:       args.SaveRecipe,
				Technology:       args.Technology,
				Language:         args.Language,
			})
			if err != nil {
				var need *store.ErrNeedsRoot
				if asNeedsRoot(err, &need) {
					payload, _ := json.MarshalIndent(need.Inference, "", "  ")
					return &mcp.CallToolResult{
						Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}},
						IsError: true,
					}, vomit.Result{}, nil
				}
				return nil, vomit.Result{}, err
			}
			payload, _ := json.MarshalIndent(res, "", "  ")
			return textResult(string(payload)), *res, nil
		})
	}

	return registered
}

func asNeedsRoot(err error, target **store.ErrNeedsRoot) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*store.ErrNeedsRoot); ok {
		*target = e
		return true
	}
	return false
}

type implContextArgs struct {
	Task             string   `json:"task" jsonschema:"Coding task, e.g. register a plugin command handler"`
	Language         string   `json:"language,omitempty" jsonschema:"Language hint (C, C++, Go, …)"`
	Technology       string   `json:"technology,omitempty" jsonschema:"Platform/library hint (Example Plugin SDK, …)"`
	ProjectRoot      string   `json:"projectRoot,omitempty" jsonschema:"Preferred current-project knowledge root"`
	PreferredRoots   []string `json:"preferredRoots,omitempty" jsonschema:"Ordered knowledge roots to search"`
	RootGroup        string   `json:"rootGroup,omitempty" jsonschema:"Named root group with priorities"`
	MaxContextTokens int      `json:"maxContextTokens,omitempty" jsonschema:"Soft token budget (estimate; default 2500)"`
	Semantic         bool     `json:"semantic,omitempty" jsonschema:"Supplement FTS with sparse term-vector similarity (also -enable-semantic)"`
}

type findSymbolArgs struct {
	Name           string   `json:"name" jsonschema:"Symbol or API name (e.g. RegisterHandler)"`
	RootName       string   `json:"rootName,omitempty"`
	PreferredRoots []string `json:"preferredRoots,omitempty"`
	Limit          int      `json:"limit,omitempty"`
}

type findSymbolResult struct {
	Symbols []store.Symbol `json:"symbols"`
	Count   int            `json:"count"`
}

type ingestMarkdownArgs struct {
	Path      string `json:"path" jsonschema:"Path to a .md/.html file or directory"`
	Recursive bool   `json:"recursive" jsonschema:"When path is a directory, recurse into subdirectories"`
	RootName  string `json:"rootName,omitempty" jsonschema:"Optional root name for project:// URIs (default: directory basename)"`
}

type deletePrefixArgs struct {
	Prefix string `json:"prefix" jsonschema:"URI prefix to delete (e.g. file:///)"`
}

type deletePrefixResult struct {
	Deleted int64  `json:"deleted"`
	Prefix  string `json:"prefix"`
}

type ingestProjectArgs struct {
	Path     string `json:"path" jsonschema:"Root directory of the project to ingest"`
	RootName string `json:"rootName,omitempty" jsonschema:"Optional root name for project:// URIs (default: directory basename)"`
}

type searchArgs struct {
	Query    string `json:"query" jsonschema:"Full-text search query"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Max hits to return (default 20, max 100)"`
	RootName string `json:"rootName,omitempty" jsonschema:"Optional knowledge root (e.g. example-device-sdk). If omitted, inferred from query; if ambiguous, tool asks you to choose."`
	Semantic bool   `json:"semantic,omitempty" jsonschema:"If true, also score related chunks via sparse term vectors (or enable server-wide with -enable-semantic)"`
}

type searchResult struct {
	Hits           []store.SearchHit `json:"hits,omitempty"`
	Count          int               `json:"count"`
	Roots          []string          `json:"roots,omitempty"`
	NeedsChoice    bool              `json:"needsChoice,omitempty"`
	Message        string            `json:"message,omitempty"`
	AvailableRoots []string          `json:"availableRoots,omitempty"`
	MatchedHints   []string          `json:"matchedHints,omitempty"`
}

type listRootsResult struct {
	Roots []string `json:"roots"`
	Count int      `json:"count"`
}

type getDocumentArgs struct {
	URI         string `json:"uri,omitempty" jsonschema:"Document URI (project://…)"`
	ID          int64  `json:"id,omitempty" jsonschema:"Numeric document id"`
	IncludeBody bool   `json:"includeBody,omitempty" jsonschema:"If true, concatenate chunk bodies into body"`
}

type documentResult struct {
	Document store.Document `json:"document"`
	Chunks   []store.Chunk  `json:"chunks"`
	Body     string         `json:"body,omitempty"`
}

type listDocumentsArgs struct {
	SourceType string `json:"sourceType,omitempty" jsonschema:"Optional filter: markdown or source"`
}

type listDocumentsResult struct {
	Documents []store.Document `json:"documents"`
	Count     int              `json:"count"`
}

type deleteDocumentArgs struct {
	URI string `json:"uri" jsonschema:"Document URI to delete"`
}

type deleteDocumentResult struct {
	Deleted bool   `json:"deleted"`
	URI     string `json:"uri"`
}

type vomitArgs struct {
	Subject        string `json:"subject" jsonschema:"Subject to research (e.g. RegisterHandler initialization sequence)"`
	OutPath        string `json:"outPath,omitempty" jsonschema:"Relative path under the server output root (default: {slug}.md)"`
	Limit          int    `json:"limit,omitempty" jsonschema:"Max documents to cite (default 8, max 20)"`
	MaxCharsPerDoc int    `json:"maxCharsPerDoc,omitempty" jsonschema:"Soft scan budget per source body (default 12000)"`
	RootName       string `json:"rootName,omitempty" jsonschema:"Optional knowledge root (e.g. example-plugin-sdk). If omitted, inferred from subject; if ambiguous, tool asks you to choose."`
	ReturnBody     bool   `json:"returnBody,omitempty" jsonschema:"Deprecated: body is always returned"`
	SaveRecipe     bool   `json:"saveRecipe,omitempty" jsonschema:"Persist as a generated knowledge_entry with source lineage"`
	Technology     string `json:"technology,omitempty"`
	Language       string `json:"language,omitempty"`
}

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: s},
		},
	}
}
