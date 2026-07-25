// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarian

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Tracker holds in-process operation status for GUI polling.
// Pre-1.0: not persisted across process restarts.
type Tracker struct {
	mu      sync.Mutex
	ops     map[string]*Operation
	maxKeep int
}

// DefaultTracker is the process-wide operation registry.
var DefaultTracker = NewTracker(64)

// NewTracker creates an operation tracker that retains at most maxKeep ops.
func NewTracker(maxKeep int) *Tracker {
	if maxKeep <= 0 {
		maxKeep = 64
	}
	return &Tracker{ops: map[string]*Operation{}, maxKeep: maxKeep}
}

// Start registers a new running operation and returns its id.
func (t *Tracker) Start(source SourceRef, phase string) string {
	id := newOpID()
	now := time.Now().Unix()
	op := &Operation{
		OpID: id, Source: source, State: "running", StartedAt: now,
		Progress: ProgressEvent{
			OpID: id, Source: source, Phase: phase, UpdatedAt: now,
		},
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ops[id] = op
	t.trimLocked()
	return id
}

// Update applies a progress event to an operation.
func (t *Tracker) Update(opID string, ev ProgressEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()
	op, ok := t.ops[opID]
	if !ok {
		return
	}
	ev.OpID = opID
	if ev.Source.Kind == "" {
		ev.Source = op.Source
	}
	if ev.UpdatedAt == 0 {
		ev.UpdatedAt = time.Now().Unix()
	}
	op.Progress = ev
	op.State = "running"
}

// Finish marks an operation terminal.
func (t *Tracker) Finish(opID, state string, report map[string]any, errors []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	op, ok := t.ops[opID]
	if !ok {
		return
	}
	if state == "" {
		state = "ok"
	}
	op.State = state
	op.FinishedAt = time.Now().Unix()
	op.Report = report
	op.Errors = append([]string{}, errors...)
	op.Progress.UpdatedAt = op.FinishedAt
	if op.Progress.Message == "" {
		op.Progress.Message = state
	}
}

// Get returns a copy of one operation.
func (t *Tracker) Get(opID string) (*Operation, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	op, ok := t.ops[opID]
	if !ok {
		return nil, false
	}
	cp := *op
	cp.Errors = append([]string{}, op.Errors...)
	return &cp, true
}

// List returns recent operations, newest first.
func (t *Tracker) List(limit int) []Operation {
	t.mu.Lock()
	defer t.mu.Unlock()
	if limit <= 0 {
		limit = 20
	}
	out := make([]Operation, 0, len(t.ops))
	for _, op := range t.ops {
		cp := *op
		cp.Errors = append([]string{}, op.Errors...)
		out = append(out, cp)
	}
	// Newest started first (simple insertion order is map-random; sort by StartedAt).
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].StartedAt > out[i].StartedAt {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (t *Tracker) trimLocked() {
	for len(t.ops) > t.maxKeep {
		var oldestID string
		var oldestAt int64 = 1<<63 - 1
		for id, op := range t.ops {
			key := op.FinishedAt
			if key == 0 {
				key = op.StartedAt
			}
			if key < oldestAt {
				oldestAt = key
				oldestID = id
			}
		}
		if oldestID == "" {
			break
		}
		delete(t.ops, oldestID)
	}
}

func newOpID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
