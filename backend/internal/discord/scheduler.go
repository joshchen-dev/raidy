package discord

import (
	"context"
	"time"
)

func (b *Bot) RunScheduler(ctx context.Context) {
	b.refreshOpenPolls(ctx)
	b.runSchedulerTick(ctx)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.runSchedulerTick(ctx)
		}
	}
}

func (b *Bot) runSchedulerTick(ctx context.Context) {
	ids, err := b.Store.DueTeamIDs(ctx, now())
	if err != nil {
		b.Log.Error("scheduler query failed", "error", err)
		return
	}
	for _, teamID := range ids {
		for {
			poll, created, err := b.Store.PublishNext(ctx, teamID, false, now())
			if err != nil {
				b.Log.Error("poll generation failed", "team_id", teamID, "error", err)
				break
			}
			if !created {
				break
			}
			if poll.ID == 0 {
				b.Log.Info("skipped elapsed period", "team_id", teamID)
				continue
			}
			b.Log.Info("poll cycle advanced", "poll_id", poll.ID, "team_id", teamID)
		}
	}
	expired, err := b.Store.CloseExpiredPolls(ctx, now())
	if err != nil {
		b.Log.Error("poll expiration failed", "error", err)
		return
	}
	for _, poll := range expired {
		if err := b.editPoll(ctx, poll.ID, false); err != nil {
			b.Log.Error("expired poll refresh failed", "poll_id", poll.ID, "team_id", poll.TeamID, "error", err)
		}
	}
	if err := b.publishUnpublished(ctx); err != nil {
		b.Log.Error("poll publication pass failed", "error", err)
	}
}

// refreshOpenPolls re-renders every published, open timetable at startup. A
// background refresh lost to a crash or restart leaves only a stale Discord
// message, never stale data, so re-rendering from PostgreSQL repairs it.
func (b *Bot) refreshOpenPolls(ctx context.Context) {
	ids, err := b.Store.PublishedOpenPollIDs(ctx)
	if err != nil {
		b.Log.Error("startup poll refresh query failed", "error", err)
		return
	}
	for _, id := range ids {
		b.refresher.queue(id)
	}
}
