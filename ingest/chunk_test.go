// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import (
	"strings"
	"testing"
)

func TestDecodeTextToUTF8Windows1252NBSP(t *testing.T) {
	// Word HTML often embeds Latin-1 0xA0 instead of UTF-8 C2 A0.
	in := []byte("hello\xa0\xa0world")
	out := DecodeTextToUTF8(in)
	if !IsValidUTF8(out) {
		t.Fatalf("still invalid UTF-8: %q", out)
	}
	if string(out) != "hello\u00a0\u00a0world" {
		t.Fatalf("got %q", out)
	}
	// Valid UTF-8 must pass through unchanged (including multi-byte).
	utf := []byte("café")
	if string(DecodeTextToUTF8(utf)) != "café" {
		t.Fatalf("valid utf8 altered")
	}
}

func TestChunkMarkdownHeadings(t *testing.T) {
	md := "# Title\n\nHello world\n\n## Section\n\nMore text about SQLite\n"
	chunks := ChunkMarkdown(md)
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks, got %d: %+v", len(chunks), chunks)
	}
	if chunks[0].Heading != "Title" {
		t.Fatalf("heading=%q", chunks[0].Heading)
	}
	if !strings.Contains(chunks[1].Body, "SQLite") {
		t.Fatalf("body=%q", chunks[1].Body)
	}
}

func TestChunkSourceSize(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("line of source code number ")
		b.WriteByte(byte('0' + i%10))
		b.WriteByte('\n')
		if i%10 == 0 {
			b.WriteByte('\n')
		}
	}
	chunks := ChunkSource(b.String())
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if c.StartLine <= 0 || c.EndLine < c.StartLine {
			t.Fatalf("bad lines: %+v", c)
		}
	}
}

func TestFileAndProjectURI(t *testing.T) {
	u := FileURI(`D:\docs\guide.md`)
	if !strings.HasPrefix(u, "file:///") {
		t.Fatalf("file uri=%q", u)
	}
	p := ProjectURI("NewBrain", `cmd\app\main.go`)
	if p != "project://NewBrain/cmd/app/main.go" {
		t.Fatalf("project uri=%q", p)
	}
}
