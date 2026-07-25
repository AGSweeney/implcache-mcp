// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package web

import "testing"

func TestNormalizeDocTitleSphinx(t *testing.T) {
	in := "App Image Format - ESP32 -  &mdash; ESP-IDF Programming Guide v6.0.2 documentation"
	got := NormalizeDocTitle(in, ProfileSphinx)
	if got != "App Image Format - ESP32" {
		t.Fatalf("got %q", got)
	}
	if ver := DetectDocVersion(in, ProfileSphinx); ver != "6.0.2" {
		t.Fatalf("version=%q", ver)
	}
}

func TestNormalizeDocTitleDoxygen(t *testing.T) {
	in := "NetBurner 3.5.8: DHCP Namespace Reference"
	got := NormalizeDocTitle(in, ProfileDoxygen)
	if got != "DHCP Namespace Reference" {
		t.Fatalf("got %q", got)
	}
	if ver := DetectDocVersion(in, ProfileDoxygen); ver != "3.5.8" {
		t.Fatalf("version=%q", ver)
	}
}
