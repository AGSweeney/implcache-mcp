// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

// TestConcurrentReads exercise the single-connection store under concurrent readers.
// This complements go test -race (which requires CGO) on no-CGO hosts.
func TestConcurrentReads(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		_, err := st.UpsertDocument(ctx, UpsertInput{
			URI: fmt.Sprintf("project://example-control-app/f%d.md", i), Title: "f",
			SourceType: SourceMarkdown, Path: fmt.Sprintf("f%d.md", i), RootName: "example-control-app",
			Hash:    fmt.Sprintf("h%d", i),
			Chunks:  []Chunk{{Body: "HandlePath WriteJSON concurrent read fixture", StartLine: 1, EndLine: 2}},
			Symbols: []SymbolInput{{Name: "HandlePath", Kind: "function", Language: "go", StartLine: 1}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 32)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := st.SearchOpts(ctx, SearchOptions{Query: "HandlePath", Limit: 5, Roots: []string{"example-control-app"}}); err != nil {
				errCh <- err
				return
			}
			if _, err := st.FindSymbols(ctx, "HandlePath", []string{"example-control-app"}, 5); err != nil {
				errCh <- err
				return
			}
			if _, err := st.ListRootNames(ctx); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}
