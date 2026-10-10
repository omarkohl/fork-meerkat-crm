package services

import (
	"context"
	"sync"
	"time"

	"meerkat/logger"
)

// Background runs the one-off tasks started at boot under one context and a
// WaitGroup, so shutdown can wait for them (bounded) before the database closes.
type Background struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	wg     sync.WaitGroup
}

// NewBackground returns a runner whose context is cancelled by Shutdown.
func NewBackground() *Background {
	ctx, cancel := context.WithCancel(context.Background())
	return &Background{ctx: ctx, cancel: cancel}
}

// Go runs fn in a goroutine. After Shutdown has begun, fn is not started.
// The tasks themselves are not interruptible; fn should check ctx before it
// begins any work.
func (b *Background) Go(name string, fn func(ctx context.Context)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ctx.Err() != nil {
		return
	}
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				logger.Error().Interface("panic", r).Str("task", name).Msg("Background task panicked")
			}
		}()
		fn(b.ctx)
	}()
}

// Shutdown cancels the context and waits up to timeout for running tasks. It
// reports whether all of them finished.
func (b *Background) Shutdown(timeout time.Duration) bool {
	b.mu.Lock()
	b.cancel()
	b.mu.Unlock()

	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}
