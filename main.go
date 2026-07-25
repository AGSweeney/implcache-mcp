package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"implcache-mcp/store"
	"implcache-mcp/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	dbPath := flag.String("db", "./implcache.db", "path to SQLite ImplCache database")
	httpAddr := flag.String("http", "", "if set, serve streamable HTTP (default bind rewrites bare :port to 127.0.0.1:port)")
	readOnly := flag.Bool("readonly", false, "disable ingest, delete, and filesystem vomit output; open DB read-only when possible")
	allowIngest := flag.Bool("allow-ingest", true, "allow ingest_* tools")
	allowDelete := flag.Bool("allow-delete", true, "allow delete_* tools")
	allowOutput := flag.Bool("allow-output-write", true, "allow vomit to write files under -output-root")
	outputRoot := flag.String("output-root", "./vomit", "directory that confines vomit output paths")
	maxResults := flag.Int("max-results", store.DefaultSearchLimit, "default/max search results per query")
	maxIngestFiles := flag.Int("max-ingest-files", 50000, "max files per ingest operation")
	maxDocBytes := flag.Int64("max-document-bytes", 8<<20, "max bytes per ingested file")
	flag.Parse()

	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	absOut, err := filepath.Abs(*outputRoot)
	if err != nil {
		log.Fatalf("output-root: %v", err)
	}

	st, err := store.OpenWithOptions(*dbPath, store.OpenOptions{ReadOnly: *readOnly})
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "implcache-mcp",
		Version: "v1.1.0",
	}, nil)

	toolOpt := tools.Options{
		ReadOnly:         *readOnly,
		AllowIngest:      *allowIngest && !*readOnly,
		AllowDelete:      *allowDelete && !*readOnly,
		AllowOutputWrite: *allowOutput && !*readOnly,
		OutputRoot:       absOut,
		MaxResults:       *maxResults,
		MaxIngestFiles:   *maxIngestFiles,
		MaxDocumentBytes: *maxDocBytes,
	}
	tools.RegisterWithOptions(server, st, toolOpt)

	if *httpAddr != "" {
		addr := normalizeHTTPAddr(*httpAddr)
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return server
		}, nil)
		srv := &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
			MaxHeaderBytes:    1 << 20,
		}
		log.Printf("implcache-mcp HTTP at %s (db=%s readonly=%v output-root=%s)", addr, *dbPath, *readOnly, absOut)

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

	log.Printf("implcache-mcp stdio (db=%s readonly=%v output-root=%s)", *dbPath, *readOnly, absOut)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

// normalizeHTTPAddr rewrites bare ":8080" to loopback to avoid accidental LAN exposure.
func normalizeHTTPAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "127.0.0.1:8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// Allow "8080" shorthand.
		if !strings.Contains(addr, ":") {
			return net.JoinHostPort("127.0.0.1", addr)
		}
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		log.Printf("warning: binding %q rewritten to 127.0.0.1:%s (pass an explicit non-loopback host to override)", addr, port)
		return net.JoinHostPort("127.0.0.1", port)
	}
	return addr
}
