// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package usage

import (
	"context"
	"log"
	"sync"
	"time"
)

const (
	queueSize   = 256
	batchSize   = 64
	flushEvery  = 500 * time.Millisecond
)

type asyncWriter struct {
	store *Store
	ch    chan RequestEvent
	done  chan struct{}
	wg    sync.WaitGroup
}

func newAsyncWriter(s *Store) *asyncWriter {
	return &asyncWriter{
		store: s,
		ch:    make(chan RequestEvent, queueSize),
		done:  make(chan struct{}),
	}
}

func (w *asyncWriter) start() {
	w.wg.Add(1)
	go w.loop()
}

func (w *asyncWriter) stop(timeout time.Duration) {
	close(w.done)
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

func (w *asyncWriter) enqueue(ev RequestEvent) {
	if w == nil {
		return
	}
	select {
	case w.ch <- ev:
	default:
		w.store.incDrops()
	}
}

func (w *asyncWriter) loop() {
	defer w.wg.Done()
	ticker := time.NewTicker(flushEvery)
	defer ticker.Stop()
	batch := make([]RequestEvent, 0, batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := w.store.insertBatch(ctx, batch); err != nil {
			log.Printf("usage analytics write: %v", err)
		}
		cancel()
		batch = batch[:0]
	}
	for {
		select {
		case <-w.done:
			for {
				select {
				case ev := <-w.ch:
					batch = append(batch, ev)
					if len(batch) >= batchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		case ev := <-w.ch:
			batch = append(batch, ev)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
