// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package vomit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"implcache-mcp/store"
)

func TestGenerateWritesPlaybook(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	body := `
#include <DeviceSdk.h>
#include <MenuBar.h>
#include <TestError.h>

int RegisterHandler(int argc, char *argv[], char *version, char *build)
{
  CmdId cmd_id;
  FileName msgfile;
  ToWideString(msgfile, "app.txt");
  AddMenuItem("AppMenu", "AppLabel", "Help", TRUE, msgfile);
  RegisterCommand("App.Hello", (CmdActFn)Hello, Immediate, NULL, TRUE, TRUE, &cmd_id);
  AddMenuItem("AppMenu", "App.Hello", "HelloBtn", "HelloHelp", NULL, TRUE, cmd_id, msgfile);
  return 0;
}
`
	_, err = st.UpsertDocument(ctx, store.UpsertInput{
		URI:        "project://demo/samples/UgMain_c.html",
		Title:      "UgMain.c",
		SourceType: store.SourceMarkdown,
		Path:       "samples/UgMain_c.html",
		RootName:   "demo",
		Hash:       "h1",
		Chunks: []store.Chunk{
			{Body: body, StartLine: 1, EndLine: 20},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "out", "menu.md")
	res, err := Generate(ctx, st, Request{
		Subject:    "RegisterHandler menubar pushbutton",
		OutPath:    filepath.Base(out),
		OutputRoot: filepath.Dir(out),
		AllowWrite: true,
		Limit:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.SourceCount < 1 {
		t.Fatalf("sourceCount=%d", res.SourceCount)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"# Implementation Playbook:",
		"## 1. Goal",
		"## 2. Prerequisites",
		"## 3. Minimal call sequence",
		"## 4. Focused pattern excerpts",
		"## 5. Common pitfalls",
		"## 6. Checklist",
		"## 7. Citations",
		"AddMenuItem",
		"project://demo/samples/UgMain_c.html",
		"#include <DeviceSdk.h>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	pre := text
	if i := strings.Index(text, "## 2. Prerequisites"); i >= 0 {
		pre = text[i:]
		if j := strings.Index(pre, "## 3."); j >= 0 {
			pre = pre[:j]
		}
	}
	if strings.Contains(pre, "TestError.h") {
		t.Fatal("demo TestError.h should be filtered from prerequisites")
	}
	// Should not dump the entire file as a "step".
	if strings.Contains(text, "## 3. Implementation steps") {
		t.Fatal("old dump format still present")
	}
}

func TestSlugify(t *testing.T) {
	if slugify("RetryPolicy Connect!") != "retrypolicy-connect" {
		t.Fatalf("got %q", slugify("RetryPolicy Connect!"))
	}
}

func TestCleanupPreservesIncludes(t *testing.T) {
	in := "/* headers */\n#include <DeviceSdk.h>\n#include <MenuBar.h>\n\n<p>hi</p>\nint RegisterHandler(void);\n"
	out := cleanupBody(in)
	if !strings.Contains(out, "#include <DeviceSdk.h>") {
		t.Fatalf("lost include: %q", out)
	}
	if strings.Contains(out, "<p>") {
		t.Fatalf("html leaked: %q", out)
	}
}

func TestCleanupCollapsesBareIncludes(t *testing.T) {
	in := "#include \n#include \n#include \nint x;\n"
	out := cleanupBody(in)
	if strings.Count(out, "#include") > 1 {
		t.Fatalf("bare includes not collapsed: %q", out)
	}
}

func TestGenerateControlAppPlaybook(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	body := `# Download a controller configuration to the controller

To download:

1. Build the demo controller project.
2. Connect to the demo controller.
3. Download the controller configuration to the controller.

Also see:
- [Create a project](107849.htm)
- [Build a demo controller project](107898.htm)
`
	_, err = st.UpsertDocument(ctx, store.UpsertInput{
		URI:        "project://example-control-app/107900.htm",
		Title:      "Download a controller configuration to the controller",
		SourceType: store.SourceMarkdown,
		Path:       "107900.htm",
		RootName:   "example-control-app",
		Hash:       "ctrl1",
		Chunks:     []store.Chunk{{Body: body, StartLine: 1, EndLine: 20}},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "control.md")
	_, err = Generate(ctx, st, Request{
		Subject:    "example-control-app download program",
		OutPath:    filepath.Base(out),
		OutputRoot: filepath.Dir(out),
		AllowWrite: true,
		Limit:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(mustRead(t, out))
	for _, want := range []string{
		"## 3. Minimal workflow",
		"example-control-app",
		"project://example-control-app/107900.htm",
		"plugin.dat",
	} {
		if want == "plugin.dat" {
			if strings.Contains(text, want) {
				t.Fatalf("device-sdk boilerplate leaked into control-app playbook")
			}
			continue
		}
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestGenerateNetBurnerEFFSPlaybook(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	// Simulate OCR PDF: long intro, then the real API section later.
	intro := strings.Repeat("EFFS introduction hardware setup NetBurner platforms SD MMC. ", 200)
	apis := `
3.1 Common EFFS Function Calls
Create/delete working directory for current task priority:
int f_enterFS( void );
void f_releaseFS(void );
Mount/dismount a flash card:
int f_mountfat(MMC_DRV_NUM, mmc_initfunc, F_MMC_DRIVE0);
int f_delvolume(int drivenum)
Open/Close a file
F_FILE *f_open(const char *filename, const char *mode);
int f_close(F_FILE *filehandle)
long f_write(const void *buf, long size,long size_st, F_FILE *filehandle)
long f_read( void *buf, long size,long size_st, F_FILE *filehandle)
`
	body := intro + apis
	_, err = st.UpsertDocument(ctx, store.UpsertInput{
		URI:        "pdf://NetBurner/EFFS-ProgrammersGuide.pdf",
		Title:      "EFFS Programmers Guide",
		SourceType: store.SourceMarkdown,
		Path:       "EFFS-ProgrammersGuide.pdf",
		RootName:   "NetBurner",
		Hash:       "effs1",
		Chunks:     []store.Chunk{{Body: body, StartLine: 1, EndLine: 40}},
	})
	if err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "effs.md")
	res, err := Generate(ctx, st, Request{
		Subject:    "EFFS setup",
		OutPath:    filepath.Base(out),
		OutputRoot: filepath.Dir(out),
		AllowWrite: true,
		RootNames:  []string{"NetBurner"},
		Limit:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(mustRead(t, out))
	for _, want := range []string{
		"f_enterFS",
		"f_mountfat",
		"pdf://NetBurner/EFFS-ProgrammersGuide.pdf",
		"NetBurner",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	for _, bad := range []string{
		"creotk.dat",
		"ProMenubar",
		"user_initialize",
		"text/usascii",
		"Toolkit vs OTK",
		"message-file labels and register the DLL",
		"init/menu registration block",
	} {
		if strings.Contains(text, bad) {
			t.Fatalf("Creo boilerplate leaked (%q) into NetBurner playbook:\n%s", bad, text)
		}
	}
	if res.SourceCount < 1 {
		t.Fatalf("sourceCount=%d", res.SourceCount)
	}
}

func TestGeneratePromptsWhenRootAmbiguous(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	for _, root := range []string{"example-control-app", "example-device-sdk"} {
		_, err := st.UpsertDocument(ctx, store.UpsertInput{
			URI:        "project://" + root + "/x.md",
			Title:      "x",
			SourceType: store.SourceMarkdown,
			Path:       "x.md",
			RootName:   root,
			Hash:       root,
			Chunks:     []store.Chunk{{Body: "create a project download program", StartLine: 1, EndLine: 1}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = Generate(ctx, st, Request{
		Subject: "create a project",
		OutPath: filepath.Join(dir, "out.md"),
	})
	if err == nil {
		t.Fatal("expected ErrNeedsRoot")
	}
	if _, ok := err.(*store.ErrNeedsRoot); !ok {
		t.Fatalf("got %T: %v", err, err)
	}
}
