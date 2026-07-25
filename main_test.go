// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"implcache-mcp/tools"
)

func TestResolveMutationFlagsHTTPDefaultOff(t *testing.T) {
	in, del, out := resolveMutationFlags(true, true, true, false, true, false)
	if in || del || out {
		t.Fatalf("HTTP without enable-http-mutations must deny writes: %v %v %v", in, del, out)
	}
	in, del, out = resolveMutationFlags(true, true, true, false, true, true)
	if !in || !del || !out {
		t.Fatalf("HTTP with enable-http-mutations should allow: %v %v %v", in, del, out)
	}
	in, del, out = resolveMutationFlags(true, true, true, true, false, false)
	if in || del || out {
		t.Fatalf("readonly must deny: %v %v %v", in, del, out)
	}
	in, del, out = resolveMutationFlags(true, true, true, false, false, false)
	if !in || !del || !out {
		t.Fatalf("stdio admin defaults should allow: %v %v %v", in, del, out)
	}
}

func TestParseToolMode(t *testing.T) {
	got, err := parseToolMode("")
	if err != nil || got != tools.ModeAgent {
		t.Fatalf("empty: got %q err=%v", got, err)
	}
	got, err = parseToolMode("agent")
	if err != nil || got != tools.ModeAgent {
		t.Fatalf("agent: got %q err=%v", got, err)
	}
	got, err = parseToolMode("ADMIN")
	if err != nil || got != tools.ModeAdmin {
		t.Fatalf("admin: got %q err=%v", got, err)
	}
	_, err = parseToolMode("ops")
	if err == nil || !strings.Contains(err.Error(), "invalid -mode") {
		t.Fatalf("expected invalid mode error, got %v", err)
	}
}

func TestVersionDefaultDev(t *testing.T) {
	if version != "dev" {
		t.Fatalf("source default version=%q want dev (rebuild without -X if injected)", version)
	}
}

func TestVersionInjectedViaLdflags(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build subprocess in short mode")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "implcache-mcp")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-ldflags", "-X main.version=v0.2.0-test", "-o", bin, ".")
	build.Dir = "."
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, "-version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run -version: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != "v0.2.0-test" {
		t.Fatalf("got %q want v0.2.0-test", got)
	}
}

func TestNormalizeHTTPAddrLoopback(t *testing.T) {
	got, err := normalizeHTTPAddr(":8080", false)
	if err != nil || got != "127.0.0.1:8080" {
		t.Fatalf("got %q err=%v", got, err)
	}
	got, err = normalizeHTTPAddr("0.0.0.0:9000", false)
	if err != nil || got != "127.0.0.1:9000" {
		t.Fatalf("got %q err=%v", got, err)
	}
	got, err = normalizeHTTPAddr("127.0.0.1:8080", false)
	if err != nil || got != "127.0.0.1:8080" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestNormalizeHTTPAddrRemoteRefused(t *testing.T) {
	_, err := normalizeHTTPAddr("192.168.1.10:8080", false)
	if err == nil {
		t.Fatal("expected refusal without -allow-remote-http")
	}
	got, err := normalizeHTTPAddr("192.168.1.10:8080", true)
	if err != nil || got != "192.168.1.10:8080" {
		t.Fatalf("got %q err=%v", got, err)
	}
}
