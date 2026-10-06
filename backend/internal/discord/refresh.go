package discord

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// refreshDelay groups rapid changes (several leader clicks, a burst of votes)
// into one Discord message edit, which keeps us under Discord's per-channel
// edit rate limit.
const refreshDelay = 750 * time.Millisecond

// pollRefresher updates published timetable messages in the background so
// web and Discord interactions respond as soon as the database commit lands.
type pollRefresher struct {
	mu      sync.Mutex
	pending map[int64]bool
	delay   time.Duration
	log     *slog.Logger
	refresh func(context.Context, int64) error
}

func newPollRefresher(delay time.Duration, log *slog.Logger, refresh func(context.Context, int64) error) *pollRefresher {
	return &pollRefresher{pending: make(map[int64]bool), delay: delay, log: log, refresh: refresh}
}

// queue schedules a refresh of the poll's message. Calls made while a refresh
// is already waiting are absorbed by it, since it renders the latest state.
func (r *pollRefresher) queue(pollID int64) {
	r.mu.Lock()
	if r.pending[pollID] {
		r.mu.Unlock()
		return
	}
	r.pending[pollID] = true
	r.mu.Unlock()

	time.AfterFunc(r.delay, func() {
		r.mu.Lock()
		delete(r.pending, pollID)
		r.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := r.refresh(ctx, pollID); err != nil {
			r.log.Error("poll refresh failed", "poll_id", pollID, "error", err)
		}
	})
}
