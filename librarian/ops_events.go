// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarian

import (
	"context"
)

type opRuntime struct {
	cancel context.CancelFunc
	subs   map[chan ProgressEvent]struct{}
}

// SetCancel attaches a cancel func for a running job.
func (t *Tracker) SetCancel(opID string, cancel context.CancelFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.runtime == nil {
		t.runtime = map[string]*opRuntime{}
	}
	rt := t.runtime[opID]
	if rt == nil {
		rt = &opRuntime{subs: map[chan ProgressEvent]struct{}{}}
		t.runtime[opID] = rt
	}
	rt.cancel = cancel
}

// Cancel requests cancellation of a running job.
func (t *Tracker) Cancel(opID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	rt := t.runtime[opID]
	if rt == nil || rt.cancel == nil {
		return false
	}
	rt.cancel()
	if op, ok := t.ops[opID]; ok && op.State == "running" {
		op.State = "cancelling"
	}
	return true
}

// Subscribe receives progress events for an operation until it finishes or the channel closes.
func (t *Tracker) Subscribe(opID string, buffer int) (<-chan ProgressEvent, func()) {
	if buffer <= 0 {
		buffer = 16
	}
	ch := make(chan ProgressEvent, buffer)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.runtime == nil {
		t.runtime = map[string]*opRuntime{}
	}
	rt := t.runtime[opID]
	if rt == nil {
		rt = &opRuntime{subs: map[chan ProgressEvent]struct{}{}}
		t.runtime[opID] = rt
	}
	rt.subs[ch] = struct{}{}
	// Send current snapshot.
	if op, ok := t.ops[opID]; ok {
		select {
		case ch <- op.Progress:
		default:
		}
	}
	unsub := func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if rt := t.runtime[opID]; rt != nil {
			if _, ok := rt.subs[ch]; ok {
				delete(rt.subs, ch)
				close(ch)
			}
		}
	}
	return ch, unsub
}

func (t *Tracker) publishLocked(opID string, ev ProgressEvent) {
	rt := t.runtime[opID]
	if rt == nil {
		return
	}
	for ch := range rt.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// finishRuntimeLocked cleans runtime after finish (call under lock from Finish).
func (t *Tracker) finishRuntimeLocked(opID string) {
	rt := t.runtime[opID]
	if rt == nil {
		return
	}
	for ch := range rt.subs {
		close(ch)
		delete(rt.subs, ch)
	}
	delete(t.runtime, opID)
}
