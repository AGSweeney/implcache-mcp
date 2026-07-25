// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"encoding/json"
	"net/http"
	"time"
)

func (h *handler) handleListJobs(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 20)
	jobs := nonNilSlice(h.opt.Tracker.List(limit))
	// "jobs" is the REST name; "operations" mirrors the MCP list_operations shape.
	WriteJSON(w, http.StatusOK, map[string]any{"jobs": jobs, "operations": jobs, "count": len(jobs)})
}

func (h *handler) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	op, ok := h.opt.Tracker.Get(id)
	if !ok {
		WriteAPIError(w, http.StatusNotFound, APIError{Code: "not_found", Message: "job not found", JobID: id})
		return
	}
	WriteJSON(w, http.StatusOK, op)
}

func (h *handler) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	if !h.allowMutation(w, r, "ingest") {
		return
	}
	id := r.PathValue("id")
	if _, ok := h.opt.Tracker.Get(id); !ok {
		WriteAPIError(w, http.StatusNotFound, APIError{Code: "not_found", Message: "job not found", JobID: id})
		return
	}
	ok := h.opt.Tracker.Cancel(id)
	WriteJSON(w, http.StatusOK, map[string]any{"jobId": id, "cancelled": ok})
}

// handleJobEvents streams job progress as Server-Sent Events until the job
// finishes (subscription channel closes) or the client disconnects.
func (h *handler) handleJobEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := h.opt.Tracker.Get(id); !ok {
		WriteAPIError(w, http.StatusNotFound, APIError{Code: "not_found", Message: "job not found", JobID: id})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		WriteError(w, http.StatusInternalServerError, "internal", "streaming unsupported")
		return
	}

	ch, unsubscribe := h.opt.Tracker.Subscribe(id, 16)
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Keepalive comment so proxies/browsers don't idle-close the stream.
			if _, err := w.Write([]byte(": keepalive\n\n")); err != nil {
				return
			}
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if _, err := w.Write([]byte("event: progress\ndata: " + string(data) + "\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
