// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"net/http"
	"strings"

	"implcache-mcp/usage"
)

func (h *handler) recordUsage(r *http.Request, ev usage.RequestEvent) {
	defer func() { _ = recover() }()
	if h == nil || h.opt.Usage == nil {
		return
	}
	if r != nil {
		if sid := strings.TrimSpace(r.Header.Get("X-ImplCache-Session")); sid != "" {
			ev.SessionHash = h.opt.Usage.SessionHash(r.Context(), sid)
		}
		if c := strings.TrimSpace(r.Header.Get("X-ImplCache-Client")); c != "" {
			ev.ClientName = c
		}
	}
	h.opt.Usage.Record(ev)
}
