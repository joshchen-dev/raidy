package discord

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestPollRefresherCoalescesBursts(t *testing.T) {
	var mu sync.Mutex
	calls := map[int64]int{}
	done := make(chan struct{}, 10)
	r := newPollRefresher(20*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)), func(_ context.Context, pollID int64) error {
		mu.Lock()
		calls[pollID]++
		mu.Unlock()
		done <- struct{}{}
		return nil
	})

	// Five rapid leader clicks on one poll and one on another.
	for i := 0; i < 5; i++ {
		r.queue(1)
	}
	r.queue(2)
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("refresh did not run")
		}
	}
	// A change after the refresh ran must schedule another refresh.
	r.queue(1)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("later change was not refreshed")
	}
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if calls[1] != 2 || calls[2] != 1 {
		t.Fatalf("refresh calls = %v, want poll 1 twice and poll 2 once", calls)
	}
}
