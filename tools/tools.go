// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"implcache-mcp/gitrepo"
	"implcache-mcp/implctx"
	"implcache-mcp/ingest"
	"implcache-mcp/librarian"
	"implcache-mcp/pdf"
	"implcache-mcp/store"
	"implcache-mcp/vomit"
	"implcache-mcp/web"

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
	"ingest_url",
	"add_web_source",
	"ingest_site",
	"refresh_web_source",
	"list_web_sources",
	"remove_web_source",
	"prune_web_source",
	"inspect_pdf",
	"ingest_pdf",
	"remove_pdf",
	"inspect_repo",
	"add_repo_source",
	"ingest_repo",
	"refresh_repo_source",
	"list_repo_sources",
	"remove_repo_source",
	"list_documents",
	"delete_document",
	"delete_by_uri_prefix",
	"list_sources",
	"get_source",
	"source_health",
	"recent_source_errors",
	"preview_document",
	"search_playground",
	"get_operation",
	"list_operations",
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
			MaxResults:       opt.MaxResults,
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
		syms, err := st.FindSymbols(ctx, args.Name, roots, store.ClampSearchLimit(args.Limit, opt.MaxResults))
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

		mcp.AddTool(server, &mcp.Tool{
			Name:        "ingest_url",
			Description: "Fetch and ingest one approved documentation URL (admin-only; SSRF-safe; no link following)",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args ingestURLArgs) (*mcp.CallToolResult, web.URLIngestResult, error) {
			if !opt.AllowIngest {
				return nil, web.URLIngestResult{}, deny("ingest")
			}
			res, err := web.IngestURL(ctx, st, web.IngestURLOptions{
				URL:               args.URL,
				RootName:          args.RootName,
				Authority:         args.Authority,
				Product:           args.Product,
				Version:           args.Version,
				Target:            args.Target,
				Language:          args.Language,
				Profile:           args.Profile,
				AllowInsecureHTTP: args.AllowInsecureHTTP,
				MaxBytes:          opt.MaxDocumentBytes,
			})
			if err != nil {
				return nil, web.URLIngestResult{}, err
			}
			payload, _ := json.MarshalIndent(res, "", "  ")
			return textResult(string(payload)), *res, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "add_web_source",
			Description: "Register an approved documentation site for controlled crawling",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args addWebSourceArgs) (*mcp.CallToolResult, store.WebSource, error) {
			if !opt.AllowIngest {
				return nil, store.WebSource{}, deny("ingest")
			}
			prefixes := args.AllowedPrefixes
			if len(prefixes) == 0 {
				prefixes = []string{args.StartURL}
			}
			id, err := st.UpsertWebSource(ctx, store.WebSource{
				Name: args.Name, RootName: args.RootName, StartURL: args.StartURL,
				Profile: args.Profile, AllowedPrefixes: prefixes, Authority: args.Authority,
				Product: args.Product, DeclaredVersion: args.Version, Target: args.Target,
				Language: args.Language, Enabled: true,
			})
			if err != nil {
				return nil, store.WebSource{}, err
			}
			ws, err := st.GetWebSourceByID(ctx, id)
			if err != nil {
				return nil, store.WebSource{}, err
			}
			payload, _ := json.MarshalIndent(ws, "", "  ")
			return textResult(string(payload)), *ws, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "ingest_site",
			Description: "Crawl a registered web source within allowed URL prefixes",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args siteCrawlArgs) (*mcp.CallToolResult, web.CrawlReport, error) {
			if !opt.AllowIngest {
				return nil, web.CrawlReport{}, deny("ingest")
			}
			rep, err := runTrackedCrawl(ctx, st, args, opt.MaxDocumentBytes, false)
			if err != nil {
				return nil, web.CrawlReport{}, err
			}
			payload, _ := json.MarshalIndent(rep, "", "  ")
			return textResult(string(payload)), *rep, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "refresh_web_source",
			Description: "Refresh a registered web source using conditional requests when possible",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args siteCrawlArgs) (*mcp.CallToolResult, web.CrawlReport, error) {
			if !opt.AllowIngest {
				return nil, web.CrawlReport{}, deny("ingest")
			}
			rep, err := runTrackedCrawl(ctx, st, args, opt.MaxDocumentBytes, true)
			if err != nil {
				return nil, web.CrawlReport{}, err
			}
			payload, _ := json.MarshalIndent(rep, "", "  ")
			return textResult(string(payload)), *rep, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "list_web_sources",
			Description: "List registered documentation web sources",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listWebSourcesResult, error) {
			list, err := st.ListWebSources(ctx)
			if err != nil {
				return nil, listWebSourcesResult{}, err
			}
			out := listWebSourcesResult{Sources: list, Count: len(list)}
			payload, _ := json.MarshalIndent(out, "", "  ")
			return textResult(string(payload)), out, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "remove_web_source",
			Description: "Remove a web source and delete its mirrored documents",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args nameArgs) (*mcp.CallToolResult, deleteDocumentResult, error) {
			if !opt.AllowDelete {
				return nil, deleteDocumentResult{}, deny("delete")
			}
			ok, err := st.DeleteWebSource(ctx, args.Name)
			if err != nil {
				return nil, deleteDocumentResult{}, err
			}
			out := deleteDocumentResult{Deleted: ok, URI: args.Name}
			return textResult(fmt.Sprintf("deleted=%v name=%s", ok, args.Name)), out, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "prune_web_source",
			Description: "Delete mirrored pages missing for N successful crawl generations",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args pruneArgs) (*mcp.CallToolResult, pruneResult, error) {
			if !opt.AllowDelete {
				return nil, pruneResult{}, deny("delete")
			}
			ws, err := st.GetWebSourceByName(ctx, args.Name)
			if err != nil {
				return nil, pruneResult{}, err
			}
			n, err := st.PruneWebPages(ctx, ws.ID, args.Threshold)
			if err != nil {
				return nil, pruneResult{}, err
			}
			out := pruneResult{Name: args.Name, Deleted: n}
			payload, _ := json.MarshalIndent(out, "", "  ")
			return textResult(string(payload)), out, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "inspect_pdf",
			Description: "Inspect a local PDF (metadata, classification, bookmarks) without writing to the database",
		}, func(_ context.Context, _ *mcp.CallToolRequest, args pdfInspectArgs) (*mcp.CallToolResult, pdf.InspectReport, error) {
			if !opt.AllowIngest {
				return nil, pdf.InspectReport{}, deny("ingest")
			}
			rep, err := pdf.InspectPDF(args.Path, pdf.InspectOptions{
				MaxFileBytes: opt.MaxDocumentBytes,
				MaxPages:     args.MaxPages,
				PageStart:    args.PageStart,
				PageEnd:      args.PageEnd,
			})
			if err != nil {
				return nil, pdf.InspectReport{}, err
			}
			payload, _ := json.MarshalIndent(rep, "", "  ")
			return textResult(string(payload)), *rep, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "ingest_pdf",
			Description: "Ingest a local text PDF with page-cited chunks (admin-only; OCR not supported in Stage 1)",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args pdfIngestArgs) (*mcp.CallToolResult, pdf.IngestResult, error) {
			if !opt.AllowIngest {
				return nil, pdf.IngestResult{}, deny("ingest")
			}
			res, err := pdf.IngestPDF(ctx, st, pdf.IngestOptions{
				Path:         args.Path,
				RootName:     args.RootName,
				Authority:    args.Authority,
				Product:      args.Product,
				Version:      args.Version,
				Language:     args.Language,
				OCRMode:      args.OCRMode,
				PageStart:    args.PageStart,
				PageEnd:      args.PageEnd,
				MaxFileBytes: opt.MaxDocumentBytes,
				MaxPages:     args.MaxPages,
				Force:        args.Force,
			})
			if err != nil {
				return nil, pdf.IngestResult{}, err
			}
			payload, _ := json.MarshalIndent(res, "", "  ")
			return textResult(string(payload)), *res, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "remove_pdf",
			Description: "Remove an ingested PDF document by URI (pdf://…)",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args deleteDocumentArgs) (*mcp.CallToolResult, deleteDocumentResult, error) {
			if !opt.AllowDelete {
				return nil, deleteDocumentResult{}, deny("delete")
			}
			ok, err := pdf.RemovePDF(ctx, st, args.URI)
			if err != nil {
				return nil, deleteDocumentResult{}, err
			}
			out := deleteDocumentResult{Deleted: ok, URI: args.URI}
			return textResult(fmt.Sprintf("deleted=%v uri=%s", ok, args.URI)), out, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "inspect_repo",
			Description: "Inspect a Git remote or local checkout (no ingest; admin-only)",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args repoInspectArgs) (*mcp.CallToolResult, gitrepo.InspectReport, error) {
			if !opt.AllowIngest {
				return nil, gitrepo.InspectReport{}, deny("ingest")
			}
			rep, err := gitrepo.InspectRepo(ctx, gitrepo.InspectOptions{
				RemoteURL: args.RemoteURL, LocalPath: args.LocalPath, Ref: args.Ref,
			})
			if err != nil {
				return nil, gitrepo.InspectReport{}, err
			}
			payload, _ := json.MarshalIndent(rep, "", "  ")
			return textResult(string(payload)), *rep, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "add_repo_source",
			Description: "Register a Git repository source configuration",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args repoAddArgs) (*mcp.CallToolResult, store.RepoSource, error) {
			if !opt.AllowIngest {
				return nil, store.RepoSource{}, deny("ingest")
			}
			rs, err := gitrepo.AddRepoSource(ctx, st, gitrepo.IngestOptions{
				Name: args.Name, RemoteURL: args.RemoteURL, LocalPath: args.LocalPath,
				RootName: args.RootName, AcquisitionMode: args.AcquisitionMode, Ref: args.Ref,
				Authority: args.Authority, Product: args.Product, Version: args.Version,
				CredentialRef: args.CredentialReference, IncludePatterns: args.IncludePatterns,
				ExcludePatterns: args.ExcludePatterns, SparsePaths: args.SparsePaths,
				SubmodulePolicy: args.SubmodulePolicy, SymlinkPolicy: args.SymlinkPolicy,
				WorkingTreeMode: args.WorkingTreeMode, CloneDepth: args.CloneDepth,
				PartialCloneFilter: args.PartialCloneFilter,
			})
			if err != nil {
				return nil, store.RepoSource{}, err
			}
			payload, _ := json.MarshalIndent(rs, "", "  ")
			return textResult(string(payload)), *rs, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "ingest_repo",
			Description: "Acquire and ingest a Git repository into a versioned knowledge root (git:// URIs)",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args repoAddArgs) (*mcp.CallToolResult, gitrepo.IngestReport, error) {
			if !opt.AllowIngest {
				return nil, gitrepo.IngestReport{}, deny("ingest")
			}
			mode := args.AcquisitionMode
			if mode == "" {
				if args.LocalPath != "" {
					mode = "local_checkout"
				} else {
					mode = "snapshot"
				}
			}
			res, err := gitrepo.IngestRepo(ctx, st, gitrepo.IngestOptions{
				Name: args.Name, RemoteURL: args.RemoteURL, LocalPath: args.LocalPath,
				RootName: args.RootName, AcquisitionMode: mode, Ref: args.Ref,
				Authority: args.Authority, Product: args.Product, Version: args.Version,
				CredentialRef: args.CredentialReference, IncludePatterns: args.IncludePatterns,
				ExcludePatterns: args.ExcludePatterns, SparsePaths: args.SparsePaths,
				SubmodulePolicy: args.SubmodulePolicy, SymlinkPolicy: args.SymlinkPolicy,
				WorkingTreeMode: args.WorkingTreeMode, CloneDepth: args.CloneDepth,
				PartialCloneFilter: args.PartialCloneFilter, PersistSource: true,
				MaxFiles: opt.MaxIngestFiles, MaxDocumentBytes: opt.MaxDocumentBytes,
				CacheRoot: gitrepo.CacheRootForDB(""),
			})
			if err != nil {
				return nil, gitrepo.IngestReport{}, err
			}
			payload, _ := json.MarshalIndent(res, "", "  ")
			return textResult(string(payload)), *res, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "refresh_repo_source",
			Description: "Fetch and incrementally reindex a registered Git repository source",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args nameArgs) (*mcp.CallToolResult, gitrepo.IngestReport, error) {
			if !opt.AllowIngest {
				return nil, gitrepo.IngestReport{}, deny("ingest")
			}
			res, err := gitrepo.RefreshRepoSource(ctx, st, args.Name, gitrepo.CacheRootForDB(""), nil)
			if err != nil {
				return nil, gitrepo.IngestReport{}, err
			}
			payload, _ := json.MarshalIndent(res, "", "  ")
			return textResult(string(payload)), *res, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "list_repo_sources",
			Description: "List registered Git repository sources",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listRepoSourcesResult, error) {
			list, err := st.ListRepoSources(ctx)
			if err != nil {
				return nil, listRepoSourcesResult{}, err
			}
			out := listRepoSourcesResult{Sources: list, Count: len(list)}
			payload, _ := json.MarshalIndent(out, "", "  ")
			return textResult(string(payload)), out, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "remove_repo_source",
			Description: "Remove a Git repository source; optionally delete indexed content and managed clone",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args repoRemoveArgs) (*mcp.CallToolResult, deleteDocumentResult, error) {
			if !opt.AllowDelete {
				return nil, deleteDocumentResult{}, deny("delete")
			}
			ok, err := gitrepo.RemoveRepoSource(ctx, st, args.Name, args.RemoveIndex || !args.ConfigOnly, args.RemoveClone)
			if err != nil {
				return nil, deleteDocumentResult{}, err
			}
			out := deleteDocumentResult{Deleted: ok, URI: args.Name}
			return textResult(fmt.Sprintf("deleted=%v name=%s", ok, args.Name)), out, nil
		})
	}

	mcp.AddTool(server, &mcp.Tool{
		Name: "search_knowledge",
		Description: "Full-text search the knowledge base (FTS5 with snippets). " +
			"Infers knowledge root from the query when possible; if ambiguous, returns needsChoice " +
			"with availableRoots — ask the user and re-run with rootName.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args searchArgs) (*mcp.CallToolResult, searchResult, error) {
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
			Query:      args.Query,
			Limit:      args.Limit,
			MaxResults: opt.MaxResults,
			Roots:      inf.Roots,
			Semantic:   opt.EnableSemantic || args.Semantic,
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
			Description: "List ingested documents, optionally filtered by sourceType (markdown|source|web|pdf|git)",
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
			Name:        "list_sources",
			Description: "Unified inventory of web, PDF, Git, and local document roots for the Librarian GUI",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listSourcesResult, error) {
			list, err := librarian.ListSources(ctx, st)
			if err != nil {
				return nil, listSourcesResult{}, err
			}
			out := listSourcesResult{Sources: list, Count: len(list)}
			b, _ := json.MarshalIndent(out, "", "  ")
			return textResult(string(b)), out, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "get_source",
			Description: "Inspect one source by kind (web|pdf|repo|local) and id (name, pdf URI, or rootName)",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args sourceRefArgs) (*mcp.CallToolResult, librarian.SourceSummary, error) {
			sum, err := librarian.GetSource(ctx, st, librarian.SourceKind(args.Kind), args.ID)
			if err != nil {
				return nil, librarian.SourceSummary{}, err
			}
			b, _ := json.MarshalIndent(sum, "", "  ")
			return textResult(string(b)), *sum, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "source_health",
			Description: "Health/status snapshot for one source (counts, state, recent errors)",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args sourceRefArgs) (*mcp.CallToolResult, librarian.SourceHealth, error) {
			h, err := librarian.GetSourceHealth(ctx, st, librarian.SourceKind(args.Kind), args.ID)
			if err != nil {
				return nil, librarian.SourceHealth{}, err
			}
			b, _ := json.MarshalIndent(h, "", "  ")
			return textResult(string(b)), *h, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "recent_source_errors",
			Description: "Recent errors for one source (web page failures, repo/PDF status)",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args recentErrorsArgs) (*mcp.CallToolResult, recentErrorsResult, error) {
			limit := args.Limit
			if limit <= 0 {
				limit = 20
			}
			errs, err := librarian.RecentErrors(ctx, st, librarian.SourceKind(args.Kind), args.ID, limit)
			if err != nil {
				return nil, recentErrorsResult{}, err
			}
			out := recentErrorsResult{Kind: args.Kind, ID: args.ID, Errors: errs, Count: len(errs)}
			b, _ := json.MarshalIndent(out, "", "  ")
			return textResult(string(b)), out, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "preview_document",
			Description: "Bounded document/chunk preview for the Librarian GUI",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args previewDocumentArgs) (*mcp.CallToolResult, librarian.PreviewResult, error) {
			res, err := librarian.PreviewDocument(ctx, st, librarian.PreviewOptions{
				URI: args.URI, ID: args.ID, MaxChunks: args.MaxChunks, MaxChars: args.MaxChars,
				IncludeBody: args.IncludeBody,
			})
			if err != nil {
				return nil, librarian.PreviewResult{}, err
			}
			b, _ := json.MarshalIndent(res, "", "  ")
			return textResult(string(b)), *res, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "search_playground",
			Description: "Admin search playground with optional EXPLAIN QUERY PLAN",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args searchPlaygroundArgs) (*mcp.CallToolResult, librarian.SearchPlaygroundResult, error) {
			res, err := librarian.SearchPlayground(ctx, st, librarian.SearchPlaygroundOptions{
				Query: args.Query, Roots: args.Roots, RootName: args.RootName,
				Limit: args.Limit, Semantic: args.Semantic, Explain: args.Explain,
			})
			if err != nil {
				return nil, librarian.SearchPlaygroundResult{}, err
			}
			b, _ := json.MarshalIndent(res, "", "  ")
			return textResult(string(b)), *res, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "get_operation",
			Description: "Poll in-process ingest/crawl operation progress by opId",
		}, func(_ context.Context, _ *mcp.CallToolRequest, args opIDArgs) (*mcp.CallToolResult, librarian.Operation, error) {
			op, ok := librarian.DefaultTracker.Get(args.OpID)
			if !ok {
				return nil, librarian.Operation{}, fmt.Errorf("operation %q not found", args.OpID)
			}
			b, _ := json.MarshalIndent(op, "", "  ")
			return textResult(string(b)), *op, nil
		})

		mcp.AddTool(server, &mcp.Tool{
			Name:        "list_operations",
			Description: "List recent in-process admin operations (progress/status)",
		}, func(_ context.Context, _ *mcp.CallToolRequest, args listOpsArgs) (*mcp.CallToolResult, listOperationsResult, error) {
			ops := librarian.DefaultTracker.List(args.Limit)
			out := listOperationsResult{Operations: ops, Count: len(ops)}
			b, _ := json.MarshalIndent(out, "", "  ")
			return textResult(string(b)), out, nil
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

type ingestURLArgs struct {
	URL               string `json:"url" jsonschema:"Approved documentation URL to fetch (https preferred)"`
	RootName          string `json:"rootName,omitempty" jsonschema:"Root name for project:// URIs"`
	Authority         string `json:"authority,omitempty" jsonschema:"Authority class (default official_documentation)"`
	Product           string `json:"product,omitempty"`
	Version           string `json:"version,omitempty"`
	Target            string `json:"target,omitempty" jsonschema:"Hardware/product target (e.g. esp32)"`
	Language          string `json:"language,omitempty"`
	Profile           string `json:"profile,omitempty" jsonschema:"Extraction profile: generic|sphinx|doxygen"`
	AllowInsecureHTTP bool   `json:"allowInsecureHTTP,omitempty" jsonschema:"Permit http:// URLs (default false)"`
}

type addWebSourceArgs struct {
	Name            string   `json:"name" jsonschema:"Unique source name"`
	StartURL        string   `json:"startUrl" jsonschema:"Crawl start URL"`
	RootName        string   `json:"rootName" jsonschema:"Root name for project:// URIs"`
	Profile         string   `json:"profile,omitempty" jsonschema:"generic|sphinx|doxygen"`
	AllowedPrefixes []string `json:"allowedPrefixes,omitempty" jsonschema:"URL prefixes that may be crawled"`
	Authority       string   `json:"authority,omitempty"`
	Product         string   `json:"product,omitempty"`
	Version         string   `json:"version,omitempty"`
	Target          string   `json:"target,omitempty"`
	Language        string   `json:"language,omitempty"`
}

type siteCrawlArgs struct {
	Name              string `json:"name" jsonschema:"Registered web source name"`
	MaxPages          int    `json:"maxPages,omitempty"`
	MaxDepth          int    `json:"maxDepth,omitempty"`
	AllowInsecureHTTP bool   `json:"allowInsecureHTTP,omitempty"`
}

type listWebSourcesResult struct {
	Sources []store.WebSource `json:"sources"`
	Count   int               `json:"count"`
}

type nameArgs struct {
	Name string `json:"name" jsonschema:"Web source name"`
}

type pruneArgs struct {
	Name      string `json:"name" jsonschema:"Web source name"`
	Threshold int    `json:"threshold,omitempty" jsonschema:"Missing-generation threshold (default 2)"`
}

type pruneResult struct {
	Name    string `json:"name"`
	Deleted int64  `json:"deleted"`
}

type pdfInspectArgs struct {
	Path      string `json:"path" jsonschema:"Local filesystem path to a .pdf file"`
	PageStart int    `json:"pageStart,omitempty" jsonschema:"1-based start page (optional)"`
	PageEnd   int    `json:"pageEnd,omitempty" jsonschema:"1-based end page (optional)"`
	MaxPages  int    `json:"maxPages,omitempty" jsonschema:"Maximum allowed page count"`
}

type pdfIngestArgs struct {
	Path      string `json:"path" jsonschema:"Local filesystem path to a .pdf file"`
	RootName  string `json:"rootName,omitempty" jsonschema:"Root name for pdf:// URIs"`
	Authority string `json:"authority,omitempty"`
	Product   string `json:"product,omitempty"`
	Version   string `json:"version,omitempty"`
	Language  string `json:"language,omitempty"`
	OCRMode   string `json:"ocrMode,omitempty" jsonschema:"Must be off in Stage 1"`
	PageStart int    `json:"pageStart,omitempty"`
	PageEnd   int    `json:"pageEnd,omitempty"`
	MaxPages  int    `json:"maxPages,omitempty"`
	Force     bool   `json:"force,omitempty" jsonschema:"Reingest even if file hash unchanged"`
}

type repoInspectArgs struct {
	RemoteURL string `json:"remoteUrl,omitempty" jsonschema:"Git remote or GitHub URL"`
	LocalPath string `json:"localPath,omitempty" jsonschema:"Existing local checkout"`
	Ref       string `json:"ref,omitempty" jsonschema:"Branch, tag, or commit"`
}

type repoAddArgs struct {
	Name                string   `json:"name" jsonschema:"Unique source name"`
	RemoteURL           string   `json:"remoteUrl,omitempty"`
	LocalPath           string   `json:"localPath,omitempty"`
	RootName            string   `json:"rootName,omitempty"`
	AcquisitionMode     string   `json:"acquisitionMode,omitempty" jsonschema:"snapshot|managed_clone|local_checkout"`
	Ref                 string   `json:"ref,omitempty"`
	Authority           string   `json:"authority,omitempty"`
	Product             string   `json:"product,omitempty"`
	Version             string   `json:"version,omitempty"`
	CredentialReference string   `json:"credentialReference,omitempty"`
	IncludePatterns     []string `json:"includePatterns,omitempty"`
	ExcludePatterns     []string `json:"excludePatterns,omitempty"`
	SparsePaths         []string `json:"sparsePaths,omitempty"`
	SubmodulePolicy     string   `json:"submodulePolicy,omitempty"`
	SymlinkPolicy       string   `json:"symlinkPolicy,omitempty"`
	WorkingTreeMode     string   `json:"workingTreeMode,omitempty" jsonschema:"HEAD|working_tree"`
	CloneDepth          int      `json:"cloneDepth,omitempty"`
	PartialCloneFilter  string   `json:"partialCloneFilter,omitempty"`
}

type repoRemoveArgs struct {
	Name        string `json:"name"`
	RemoveIndex bool   `json:"removeIndex,omitempty"`
	RemoveClone bool   `json:"removeClone,omitempty"`
	ConfigOnly  bool   `json:"configOnly,omitempty"`
}

type listRepoSourcesResult struct {
	Sources []store.RepoSource `json:"sources"`
	Count   int                `json:"count"`
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
	SourceType string `json:"sourceType,omitempty" jsonschema:"Optional filter: markdown, source, web, pdf, or git"`
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

func runTrackedCrawl(ctx context.Context, st *store.Store, args siteCrawlArgs, maxBytes int64, refresh bool) (*web.CrawlReport, error) {
	src := librarian.SourceRef{Kind: librarian.KindWeb, ID: args.Name}
	if ws, err := st.GetWebSourceByName(ctx, args.Name); err == nil {
		src.RootName = ws.RootName
		src.Title = ws.StartURL
	}
	phase := "crawl"
	if refresh {
		phase = "refresh"
	}
	opID := librarian.DefaultTracker.Start(src, phase)
	rep, err := web.CrawlSite(ctx, st, web.CrawlOptions{
		SourceName:        args.Name,
		MaxPages:          args.MaxPages,
		MaxDepth:          args.MaxDepth,
		AllowInsecureHTTP: args.AllowInsecureHTTP,
		MaxResponseBytes:  maxBytes,
		RefreshOnly:       refresh,
		Progress: func(done, total int, bytes int64, currentURL, message string) {
			librarian.DefaultTracker.Update(opID, librarian.ProgressEvent{
				Source: src, Phase: phase, Done: done, Total: total, Bytes: bytes,
				Current: currentURL, Message: message, UpdatedAt: time.Now().Unix(),
			})
		},
	})
	state := "ok"
	var errs []string
	report := map[string]any{}
	if err != nil {
		state = "failed"
		errs = append(errs, err.Error())
	}
	if rep != nil {
		report["new"] = rep.New
		report["changed"] = rep.Changed
		report["failed"] = rep.Failed
		report["bytesDownloaded"] = rep.Bytes
		report["limitReached"] = rep.LimitReached
		report["durationMs"] = rep.DurationMS
		errs = append(errs, rep.PageErrors...)
		if rep.FatalError != "" {
			state = "failed"
			errs = append(errs, rep.FatalError)
		} else if rep.Failed > 0 || rep.LimitReached != "" {
			if state == "ok" {
				state = "ok"
			}
		}
	}
	librarian.DefaultTracker.Finish(opID, state, report, errs)
	if rep != nil {
		rep.OpID = opID
	}
	return rep, err
}

type listSourcesResult struct {
	Sources []librarian.SourceSummary `json:"sources"`
	Count   int                       `json:"count"`
}

type sourceRefArgs struct {
	Kind string `json:"kind" jsonschema:"Source kind: web, pdf, repo, or local"`
	ID   string `json:"id" jsonschema:"Source id: web/repo name, pdf documentUri, or local rootName"`
}

type recentErrorsArgs struct {
	Kind  string `json:"kind" jsonschema:"Source kind: web, pdf, repo, or local"`
	ID    string `json:"id" jsonschema:"Source id"`
	Limit int    `json:"limit,omitempty" jsonschema:"Max errors to return (default 20)"`
}

type recentErrorsResult struct {
	Kind   string   `json:"kind"`
	ID     string   `json:"id"`
	Errors []string `json:"errors"`
	Count  int      `json:"count"`
}

type previewDocumentArgs struct {
	URI         string `json:"uri,omitempty" jsonschema:"Document URI"`
	ID          int64  `json:"id,omitempty" jsonschema:"Numeric document id"`
	MaxChunks   int    `json:"maxChunks,omitempty" jsonschema:"Max chunks to return (default 3)"`
	MaxChars    int    `json:"maxChars,omitempty" jsonschema:"Max chars per chunk body (default 2000)"`
	IncludeBody bool   `json:"includeBody,omitempty" jsonschema:"Concatenate preview chunks into body"`
}

type searchPlaygroundArgs struct {
	Query    string   `json:"query" jsonschema:"Search query"`
	Roots    []string `json:"roots,omitempty" jsonschema:"Optional rootName filters"`
	RootName string   `json:"rootName,omitempty" jsonschema:"Optional single rootName filter"`
	Limit    int      `json:"limit,omitempty" jsonschema:"Max hits (default 10)"`
	Semantic bool     `json:"semantic,omitempty" jsonschema:"Enable sparse semantic supplement"`
	Explain  bool     `json:"explain,omitempty" jsonschema:"Include EXPLAIN QUERY PLAN"`
}

type opIDArgs struct {
	OpID string `json:"opId" jsonschema:"Operation id from ingest/crawl tracking"`
}

type listOpsArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"Max operations to list (default 20)"`
}

type listOperationsResult struct {
	Operations []librarian.Operation `json:"operations"`
	Count      int                   `json:"count"`
}
