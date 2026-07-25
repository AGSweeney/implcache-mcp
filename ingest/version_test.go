// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import "testing"

func TestInferProductVersion(t *testing.T) {
	cases := []struct {
		root, path, body, want string
	}{
		{"example-device-sdk", "docs/v3.2/api.md", "", "3.2"},
		{"sdk", "release/1.0.0/readme.md", "", "1.0.0"},
		{"sdk", "guide.md", "Version: 2.1\n", "2.1"},
		{"sdk", "guide.md", "This document covers SDK 4.x features.\n", "4.x"},
		{"sdk", "plain.md", "no version here", ""},
	}
	for _, tc := range cases {
		got := InferProductVersion(tc.root, tc.path, tc.body)
		if got != tc.want {
			t.Fatalf("root=%s path=%s got %q want %q", tc.root, tc.path, got, tc.want)
		}
	}
}
