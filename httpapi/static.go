// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"net/http"
	"strings"
)

const spaIndex = "index.html"

// serveStatic serves the bundled Librarian SPA from opt.StaticFS. It strips
// opt.LibrarianBasePath from the request path, serves the matching file when
// present, and otherwise falls back to index.html so client-side routing works.
func (h *handler) serveStatic(w http.ResponseWriter, r *http.Request) {
	base := h.opt.LibrarianBasePath
	if base == "" {
		base = "/"
	}
	p := r.URL.Path
	if !strings.HasPrefix(p, base) {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(p, base)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		rel = spaIndex
	}

	if f, err := h.opt.StaticFS.Open(rel); err == nil {
		_ = f.Close()
		http.ServeFileFS(w, r, h.opt.StaticFS, rel)
		return
	}
	http.ServeFileFS(w, r, h.opt.StaticFS, spaIndex)
}
