package ingest

import "testing"

func TestExtractSymbolsGoAndPro(t *testing.T) {
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

	cSrc := `#include <ProMenuBar.h>
int user_initialize(void) {
  ProCmdActionAdd("x", 0, 0, 0, 0, 0, 0);
  return 0;
}
`
	syms = ExtractSymbols("main.c", cSrc)
	names := map[string]bool{}
	for _, s := range syms {
		names[s.Name] = true
	}
	if !names["ProCmdActionAdd"] && !names["user_initialize"] {
		t.Fatalf("expected C symbols, got %+v", syms)
	}
}
