// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"implcache-mcp/embedui"
	"implcache-mcp/httpapi"
	"implcache-mcp/manifest"
	"implcache-mcp/store"
	"implcache-mcp/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is "dev" for local builds; override at build time via:
//
//	go build -ldflags "-X main.version=$(git describe --tags --always)"
var version = "dev"

func main() {
	dbPath := flag.String("db", "./implcache.db", "path to SQLite ImplCache database")
	httpAddr := flag.String("http", "", "if set, serve HTTP (MCP at /mcp; Librarian API at /api/v1)")
	mode := flag.String("mode", "agent", "tool surface: agent (retrieval only) or admin (includes ingest/delete/vomit)")
	enableAdmin := flag.Bool("enable-admin-tools", false, "register administrative tools even when -mode=agent")
	readOnly := flag.Bool("readonly", false, "disable ingest, delete, and filesystem vomit output; open DB read-only when possible")
	allowIngest := flag.Bool("allow-ingest", true, "allow ingest_* tools when admin tools are enabled")
	allowDelete := flag.Bool("allow-delete", true, "allow delete_* tools when admin tools are enabled")
	allowOutput := flag.Bool("allow-output-write", true, "allow vomit to write files under -output-root when admin tools are enabled")
	enableHTTPMutations := flag.Bool("enable-http-mutations", false, "when serving -http, allow ingest/delete/output writes (default: mutations off over HTTP)")
	allowRemoteHTTP := flag.Bool("allow-remote-http", false, "allow binding HTTP to a non-loopback address")
	enableLibrarian := flag.Bool("enable-librarian", false, "serve Librarian UI and /api/v1 admin REST (requires -http)")
	librarianBase := flag.String("librarian-base-path", "/", "URL base path for embedded Librarian UI")
	librarianToken := flag.String("librarian-token", "", "if set, require Authorization: Bearer for /api/v1 (administrator)")
	librarianViewerToken := flag.String("librarian-viewer-token", "", "optional Bearer token with viewer (read-only) role for /api/v1")
	uploadDir := flag.String("upload-dir", "", "directory for Librarian uploads (default: <db-dir>/uploads)")
	workspace := flag.String("workspace", "", "optional workspace directory; loads .implcache.yaml for default project roots")
	projectRoot := flag.String("project-root", "", "default knowledge root treated as current_project (overrides manifest rootName)")
	outputRoot := flag.String("output-root", "./vomit-output", "directory that confines vomit output paths")
	maxResults := flag.Int("max-results", store.DefaultSearchLimit, "default/max search results per query")
	maxIngestFiles := flag.Int("max-ingest-files", 50000, "max files per ingest operation")
	maxDocBytes := flag.Int64("max-document-bytes", 8<<20, "max bytes per ingested file")
	enableSemantic := flag.Bool("enable-semantic", false, "supplement FTS with optional sparse term-vector similarity (not embeddings)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	absOut, err := filepath.Abs(*outputRoot)
	if err != nil {
		log.Fatalf("output-root: %v", err)
	}

	toolMode, err := parseToolMode(*mode)
	if err != nil {
		log.Fatal(err)
	}

	defaultProject := strings.TrimSpace(*projectRoot)
	var defaultPreferred []string
	if ws := strings.TrimSpace(*workspace); ws != "" {
		m, err := manifest.LoadFromDir(ws)
		if err != nil {
			log.Fatalf("workspace manifest: %v", err)
		}
		if m != nil {
			if defaultProject == "" {
				defaultProject = m.RootName
			}
			defaultPreferred = m.PreferredRoots()
			log.Printf("loaded %s rootName=%s related=%v", manifest.DefaultFilename, m.RootName, m.RelatedRoots)
		}
	}

	allowIngestEff, allowDeleteEff, allowOutputEff := resolveMutationFlags(
		*allowIngest, *allowDelete, *allowOutput, *readOnly, *httpAddr != "", *enableHTTPMutations,
	)
	if *httpAddr != "" && !*enableHTTPMutations && (toolMode == tools.ModeAdmin || *enableAdmin || *enableLibrarian) {
		log.Printf("HTTP: mutations disabled (pass -enable-http-mutations to allow ingest/delete/writes)")
	}
	if *enableLibrarian && *httpAddr == "" {
		log.Fatal("-enable-librarian requires -http")
	}

	st, err := store.OpenWithOptions(*dbPath, store.OpenOptions{ReadOnly: *readOnly})
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "implcache-mcp",
		Version: version,
	}, nil)

	toolOpt := tools.Options{
		Mode:                  toolMode,
		EnableAdminTools:      *enableAdmin,
		ReadOnly:              *readOnly,
		AllowIngest:           allowIngestEff,
		AllowDelete:           allowDeleteEff,
		AllowOutputWrite:      allowOutputEff,
		OutputRoot:            absOut,
		DefaultProjectRoot:    defaultProject,
		DefaultPreferredRoots: defaultPreferred,
		MaxResults:            *maxResults,
		MaxIngestFiles:        *maxIngestFiles,
		MaxDocumentBytes:      *maxDocBytes,
		EnableSemantic:        *enableSemantic,
	}
	registered := tools.RegisterWithOptions(server, st, toolOpt)
	log.Printf("implcache-mcp %s mode=%s tools=%v", version, toolOpt.EffectiveMode(), registered)

	if *httpAddr != "" {
		addr, err := normalizeHTTPAddr(*httpAddr, *allowRemoteHTTP)
		if err != nil {
			log.Fatal(err)
		}

		mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return server
		}, nil)

		uploads := strings.TrimSpace(*uploadDir)
		if uploads == "" {
			uploads = filepath.Join(filepath.Dir(*dbPath), "uploads")
		}
		_ = os.MkdirAll(uploads, 0o755)

		apiOpt := httpapi.Options{
			Store:             st,
			DBPath:            *dbPath,
			ServerVersion:     version,
			ReadOnly:          *readOnly,
			AllowIngest:       allowIngestEff,
			AllowDelete:       allowDeleteEff,
			EnableSemantic:    *enableSemantic,
			MaxDocumentBytes:  *maxDocBytes,
			MaxIngestFiles:    *maxIngestFiles,
			LibrarianEnabled:  *enableLibrarian,
			LibrarianBasePath: *librarianBase,
			APIToken:          strings.TrimSpace(*librarianToken),
			ViewerAPIToken:    strings.TrimSpace(*librarianViewerToken),
			UploadDir:         uploads,
		}
		if *enableLibrarian {
			if sub, err := embedui.FS(); err != nil {
				log.Printf("librarian embed: %v (UI disabled)", err)
			} else {
				apiOpt.StaticFS = sub
			}
		}
		librarianHandler := httpapi.NewHandler(apiOpt)

		root := http.NewServeMux()
		root.Handle("/mcp", mcpHandler)
		root.Handle("/mcp/", mcpHandler)
		root.Handle("/", librarianHandler)

		srv := &http.Server{
			Addr:              addr,
			Handler:           root,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
			MaxHeaderBytes:    1 << 20,
		}
		log.Printf("HTTP at %s mcp=/mcp api=/api/v1 librarian=%v readonly=%v mutations=%v",
			addr, *enableLibrarian, *readOnly, *enableHTTPMutations && !*readOnly)

		errCh := make(chan error, 1)
		go func() { errCh <- srv.ListenAndServe() }()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		select {
		case err := <-errCh:
			if err != nil && err != http.ErrServerClosed {
				log.Fatal(err)
			}
		case sig := <-sigCh:
			log.Printf("shutdown signal: %v", sig)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx)
		}
		return
	}

	log.Printf("stdio (db=%s readonly=%v output-root=%s)", *dbPath, *readOnly, absOut)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

// resolveMutationFlags applies readonly and HTTP mutation defaults.
// HTTP without -enable-http-mutations clears all mutation permissions.
func resolveMutationFlags(allowIngest, allowDelete, allowOutput, readOnly, httpMode, enableHTTPMutations bool) (ingest, delete, output bool) {
	ingest = allowIngest && !readOnly
	delete = allowDelete && !readOnly
	output = allowOutput && !readOnly
	if httpMode && !enableHTTPMutations {
		return false, false, false
	}
	return ingest, delete, output
}

// parseToolMode accepts agent (default) or admin.
func parseToolMode(mode string) (tools.ToolMode, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "agent":
		return tools.ModeAgent, nil
	case "admin":
		return tools.ModeAdmin, nil
	default:
		return "", fmt.Errorf("invalid -mode %q (want agent or admin)", mode)
	}
}

// normalizeHTTPAddr rewrites bare ":8080" to loopback. Non-loopback binds require -allow-remote-http.
func normalizeHTTPAddr(addr string, allowRemote bool) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "127.0.0.1:8080", nil
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if !strings.Contains(addr, ":") {
			return net.JoinHostPort("127.0.0.1", addr), nil
		}
		return "", fmt.Errorf("http address: %w", err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		log.Printf("warning: binding %q rewritten to 127.0.0.1:%s", addr, port)
		return net.JoinHostPort("127.0.0.1", port), nil
	}
	if !allowRemote && !isLoopbackHost(host) {
		return "", fmt.Errorf("refusing non-loopback HTTP bind %q without -allow-remote-http (no built-in auth)", addr)
	}
	if allowRemote && !isLoopbackHost(host) {
		log.Printf("warning: binding non-loopback %s; ImplCache has no built-in HTTP authentication", addr)
	}
	return net.JoinHostPort(host, port), nil
}

func isLoopbackHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
