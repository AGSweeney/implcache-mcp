// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package embedui embeds the Librarian frontend production build.
package embedui

import (
	"embed"
	"io/fs"
)

// Dist holds the Vite production assets under frontend/dist.
//
//go:embed all:dist
var Dist embed.FS

// FS returns the static file root (contents of dist/).
func FS() (fs.FS, error) {
	return fs.Sub(Dist, "dist")
}
