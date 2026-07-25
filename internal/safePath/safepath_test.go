package safePath

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveUnderRootOK(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveUnderRoot(root, filepath.Join("a", "b.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "a", "b.md")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveUnderRootRejectsEscape(t *testing.T) {
	root := t.TempDir()
	if _, err := ResolveUnderRoot(root, filepath.Join("..", "outside.md")); err == nil {
		t.Fatal("expected escape rejection")
	}
	if _, err := ResolveUnderRoot(root, "/etc/passwd"); err == nil {
		t.Fatal("expected absolute rejection")
	}
	if runtime.GOOS == "windows" {
		if _, err := ResolveUnderRoot(root, `C:\Windows\notepad.exe`); err == nil {
			t.Fatal("expected drive rejection")
		}
		if _, err := ResolveUnderRoot(root, `\\server\share\x`); err == nil {
			t.Fatal("expected UNC rejection")
		}
	}
}
