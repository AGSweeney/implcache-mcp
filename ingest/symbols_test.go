// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import "testing"

func TestExtractSymbolsGoAndDemo(t *testing.T) {
	goSrc := "package p\n\nfunc OpenStore(path string) error {\n\treturn nil\n}\n"
	syms := ExtractSymbols("store.go", goSrc)
	found := false
	for _, s := range syms {
		if s.Name == "OpenStore" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing OpenStore: %+v", syms)
	}

	cSrc := `#include <MenuBar.h>
int RegisterHandler(void) {
  RegisterCommand("x", 0, 0, 0, 0, 0, 0);
  return 0;
}
`
	syms = ExtractSymbols("main.c", cSrc)
	names := map[string]bool{}
	for _, s := range syms {
		names[s.Name] = true
	}
	if !names["RegisterCommand"] && !names["RegisterHandler"] {
		t.Fatalf("expected C symbols, got %+v", syms)
	}
}
