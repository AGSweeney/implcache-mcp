// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"context"
	"net/http"
	"time"

	"implcache-mcp/librarian"
	"implcache-mcp/store"
	"implcache-mcp/web"
)

func (h *handler) handleUpsertWebSource(w http.ResponseWriter, r *http.Request) {
	if !h.allowMutation(w, r, "ingest") {
		return
	}
	var ws store.WebSource
	if err := decodeJSON(r, &ws); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	id, err := h.opt.Store.UpsertWebSource(r.Context(), ws)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	out, err := h.opt.Store.GetWebSourceByID(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

type webCrawlRequest struct {
	MaxPages          int  `json:"maxPages,omitempty"`
	MaxDepth          int  `json:"maxDepth,omitempty"`
	AllowInsecureHTTP bool `json:"allowInsecureHttp,omitempty"`
}

func (h *handler) handleWebIngest(w http.ResponseWriter, r *http.Request) {
	h.startWebCrawl(w, r, false)
}

func (h *handler) handleWebRefresh(w http.ResponseWriter, r *http.Request) {
	h.startWebCrawl(w, r, true)
}

// startWebCrawl mirrors tools.runTrackedCrawl: register a tracked operation,
// run the crawl in a detached goroutine (own cancellable context, independent
// of the HTTP request lifetime), and return the opId immediately.
func (h *handler) startWebCrawl(w http.ResponseWriter, r *http.Request, refresh bool) {
	if !h.allowMutation(w, r, "ingest") {
		return
	}
	name := r.PathValue("name")
	var req webCrawlRequest
	if err := decodeJSONOptional(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	ws, err := h.opt.Store.GetWebSourceByName(r.Context(), name)
	if err != nil {
		WriteError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}

	src := librarian.SourceRef{Kind: librarian.KindWeb, ID: name, RootName: ws.RootName, Title: ws.StartURL}
	phase := "crawl"
	if refresh {
		phase = "refresh"
	}
	opID := h.opt.Tracker.Start(src, phase)
	jobCtx, cancel := context.WithCancel(context.Background())
	h.opt.Tracker.SetCancel(opID, cancel)

	go func() {
		defer cancel()
		rep, err := web.CrawlSite(jobCtx, h.opt.Store, web.CrawlOptions{
			SourceName:        name,
			MaxPages:          req.MaxPages,
			MaxDepth:          req.MaxDepth,
			AllowInsecureHTTP: req.AllowInsecureHTTP,
			MaxResponseBytes:  h.opt.MaxDocumentBytes,
			RefreshOnly:       refresh,
			Progress: func(done, total int, bytes int64, currentURL, message string) {
				h.opt.Tracker.Update(opID, librarian.ProgressEvent{
					Source: src, Phase: phase, Done: done, Total: total, Bytes: bytes,
					Current: currentURL, Message: message, UpdatedAt: time.Now().Unix(),
				})
			},
		})
		state := "ok"
		var errs []string
		report := map[string]any{}
		if err != nil {
			state = "failed"
			errs = append(errs, err.Error())
		}
		if rep != nil {
			report["new"] = rep.New
			report["changed"] = rep.Changed
			report["failed"] = rep.Failed
			report["bytesDownloaded"] = rep.Bytes
			report["limitReached"] = rep.LimitReached
			report["durationMs"] = rep.DurationMS
			errs = append(errs, rep.PageErrors...)
			if rep.FatalError != "" {
				state = "failed"
				errs = append(errs, rep.FatalError)
			}
		}
		h.opt.Tracker.Finish(opID, state, report, errs)
	}()

	WriteJSON(w, http.StatusAccepted, map[string]any{"opId": opID})
}

// handleDeleteWebSource mirrors store.DeleteWebSource, which also removes
// mirrored documents under the source's root.
func (h *handler) handleDeleteWebSource(w http.ResponseWriter, r *http.Request) {
	if !h.allowMutation(w, r, "delete") {
		return
	}
	name := r.PathValue("name")
	ok, err := h.opt.Store.DeleteWebSource(r.Context(), name)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"name": name, "deleted": ok})
}
