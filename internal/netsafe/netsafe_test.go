// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package netsafe

import (
	"net"
	"testing"
)

func TestValidateURLSchemes(t *testing.T) {
	if _, err := ValidateURL("https://docs.example.com/a", Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateURL("http://docs.example.com/a", Options{}); err == nil {
		t.Fatal("expected http blocked without allowInsecureHTTP")
	}
	if _, err := ValidateURL("http://docs.example.com/a", Options{AllowInsecureHTTP: true}); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"file:///etc/passwd", "ftp://x", "data:text/plain,hi", "javascript:alert(1)"} {
		if _, err := ValidateURL(raw, Options{AllowInsecureHTTP: true}); err == nil {
			t.Fatalf("expected reject %s", raw)
		}
	}
}

func TestBlockedIPs(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "172.16.0.1", "169.254.169.254", "::1", "fc00::1"} {
		ip := net.ParseIP(s)
		if !IsBlockedIP(ip) {
			t.Fatalf("%s should be blocked", s)
		}
	}
	if IsBlockedIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public IP should not be blocked")
	}
}

func TestValidateURLBlocksLocalhost(t *testing.T) {
	if _, err := ValidateURL("https://localhost/x", Options{}); err == nil {
		t.Fatal("expected localhost blocked")
	}
}

func TestPrefixAllowed(t *testing.T) {
	allowed := []string{"https://docs.example.com/v1/"}
	if !PrefixAllowed("https://docs.example.com/v1/api.html", allowed) {
		t.Fatal("expected allowed")
	}
	if PrefixAllowed("https://docs.example.com/v2/api.html", allowed) {
		t.Fatal("expected denied")
	}
}
