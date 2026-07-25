// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"encoding/json"
	"net/http"
)

// APIError is the standard error envelope returned by every endpoint on failure.
type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Detail    string `json:"detail,omitempty"`
	Retryable bool   `json:"retryable"`
	SourceID  string `json:"sourceId,omitempty"`
	JobID     string `json:"jobId,omitempty"`
}

// WriteJSON writes v as an indented JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// WriteError writes a simple APIError with the given code and message.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteAPIError(w, status, APIError{Code: code, Message: message})
}

// WriteAPIError writes a fully populated APIError (detail/sourceId/jobId/retryable).
func WriteAPIError(w http.ResponseWriter, status int, apiErr APIError) {
	if apiErr.Code == "" {
		apiErr.Code = "error"
	}
	WriteJSON(w, status, apiErr)
}
