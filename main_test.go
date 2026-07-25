// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import "testing"

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

func TestVersionNonEmpty(t *testing.T) {
	if version == "" {
		t.Fatal("version empty")
	}
}
