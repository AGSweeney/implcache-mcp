// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"net/http"
	"sync"
	"time"
)

// LogLine is one retained in-process log entry for GET /api/v1/logs.
type LogLine struct {
	At      int64  `json:"at"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type logRing struct {
	mu   sync.Mutex
	buf  []LogLine
	cap  int
	next int
	full bool
}

func newLogRing(n int) *logRing {
	if n <= 0 {
		n = 200
	}
	return &logRing{buf: make([]LogLine, n), cap: n}
}

func (r *logRing) Append(level, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.next] = LogLine{At: time.Now().Unix(), Level: level, Message: message}
	r.next = (r.next + 1) % r.cap
	if r.next == 0 {
		r.full = true
	}
}

func (r *logRing) Snapshot(limit int) []LogLine {
	r.mu.Lock()
	defer r.mu.Unlock()
	var all []LogLine
	if r.full {
		all = append(all, r.buf[r.next:]...)
		all = append(all, r.buf[:r.next]...)
	} else {
		all = append(all, r.buf[:r.next]...)
	}
	if limit <= 0 || limit > len(all) {
		limit = len(all)
	}
	if limit == 0 {
		return []LogLine{}
	}
	return all[len(all)-limit:]
}

// processLogs is a small in-process ring for Librarian UI diagnostics.
// Durable/server-file log shipping is out of scope for Stage 5.
var processLogs = newLogRing(256)

// NoteLibrarian records a UI-visible diagnostic line (best-effort).
func NoteLibrarian(level, message string) {
	processLogs.Append(level, message)
}

func (h *handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 100)
	lines := nonNilSlice(processLogs.Snapshot(limit))
	WriteJSON(w, http.StatusOK, map[string]any{"lines": lines, "count": len(lines)})
}
