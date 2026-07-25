// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// mkemptydb creates a sanitized empty ImplCache database (schema only, no corpora).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"implcache-mcp/store"
)

func main() {
	out := flag.String("o", "", "output database path (required)")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "usage: mkemptydb -o path/to/implcache.db")
		os.Exit(2)
	}
	abs, err := filepath.Abs(*out)
	if err != nil {
		fatal(err)
	}
	for _, side := range []string{abs, abs + "-wal", abs + "-shm"} {
		_ = os.Remove(side)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		fatal(err)
	}

	st, err := store.Open(abs)
	if err != nil {
		fatal(err)
	}
	counts, err := st.CountLibrary(context.Background())
	if err != nil {
		_ = st.Close()
		fatal(err)
	}
	if counts.Documents != 0 || counts.Chunks != 0 || counts.Symbols != 0 || counts.Recipes != 0 {
		_ = st.Close()
		fatal(fmt.Errorf(
			"refusing to ship non-empty DB: documents=%d chunks=%d symbols=%d recipes=%d",
			counts.Documents, counts.Chunks, counts.Symbols, counts.Recipes,
		))
	}
	if err := st.FinalizeEmptyPackage(); err != nil {
		_ = st.Close()
		fatal(err)
	}
	if err := st.Close(); err != nil {
		fatal(err)
	}
	_ = os.Remove(abs + "-wal")
	_ = os.Remove(abs + "-shm")

	fi, err := os.Stat(abs)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("wrote empty schema DB %s (%d bytes)\n", abs, fi.Size())
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
