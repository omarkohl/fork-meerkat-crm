package services

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBackgroundShutdownWaitsForRunningTasks(t *testing.T) {
	bg := NewBackground()
	var done atomic.Bool
	started := make(chan struct{})
	bg.Go("slow", func(ctx context.Context) {
		close(started)
		time.Sleep(100 * time.Millisecond)
		done.Store(true)
	})
	<-started

	assert.True(t, bg.Shutdown(2*time.Second))
	assert.True(t, done.Load(), "Shutdown must wait for the task")
}

func TestBackgroundShutdownIsBounded(t *testing.T) {
	bg := NewBackground()
	release := make(chan struct{})
	defer close(release)
	bg.Go("stuck", func(ctx context.Context) { <-release })

	start := time.Now()
	assert.False(t, bg.Shutdown(50*time.Millisecond), "a stuck task makes Shutdown report a timeout")
	assert.Less(t, time.Since(start), time.Second)
}

func TestBackgroundCancelsContextOnShutdown(t *testing.T) {
	bg := NewBackground()
	saw := make(chan bool, 1)
	bg.Go("waiter", func(ctx context.Context) {
		<-ctx.Done()
		saw <- true
	})
	assert.True(t, bg.Shutdown(2*time.Second))
	assert.True(t, <-saw)
}

func TestBackgroundDoesNotStartTasksAfterShutdown(t *testing.T) {
	bg := NewBackground()
	assert.True(t, bg.Shutdown(time.Second))
	var ran atomic.Bool
	bg.Go("late", func(ctx context.Context) { ran.Store(true) })
	assert.True(t, bg.Shutdown(time.Second))
	assert.False(t, ran.Load())
}
