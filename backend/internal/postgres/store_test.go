package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/joshchen-dev/raidy/internal/team"
	"github.com/joshchen-dev/raidy/migrations"
)

func openTestStore(t *testing.T) (*Store, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := migrations.Up(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE teams CASCADE`); err != nil {
		t.Fatal(err)
	}
	return &Store{pool: pool}, pool
}

func TestForcedPublishDoesNotSkipPeriods(t *testing.T) {
	store, pool := openTestStore(t)
	ctx := context.Background()
	created, err := store.CreateTeam(ctx, "guild", "Raid Night", "leader", "The Echo", "Asia/Tokyo", nil)
	if err != nil {
		t.Fatal(err)
	}
	location, _ := time.LoadLocation("Asia/Tokyo")
	periodStart := time.Date(2026, 10, 12, 0, 0, 0, 0, location)
	now := periodStart.Add(-5 * 24 * time.Hour)
	if err := store.SaveSchedule(ctx, team.ScheduleDraft{
		TeamID: created.ID, UserID: "leader", CadenceDays: 7, Weekdays: []time.Weekday{time.Wednesday},
		StartMinutes: 21 * 60, EndMinutes: 23 * 60, FirstPeriodStart: periodStart,
		FirstPublishAt: periodStart.Add(-3 * 24 * time.Hour), ChannelID: "channel",
	}); err != nil {
		t.Fatal(err)
	}
	if _, created, err := store.PublishNext(ctx, created.ID, true, now); err != nil || !created {
		t.Fatalf("first forced PublishNext() created=%v err=%v", created, err)
	}
	if _, _, err := store.PublishNext(ctx, created.ID, true, now); !errors.Is(err, ErrAlreadyPublished) {
		t.Fatalf("second forced PublishNext() error = %v, want ErrAlreadyPublished", err)
	}
	var polls int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedule_polls WHERE team_id=$1`, created.ID).Scan(&polls); err != nil || polls != 1 {
		t.Fatalf("poll count=%d err=%v, want 1", polls, err)
	}
	schedule, err := store.Schedule(ctx, created.ID)
	if err != nil || schedule.NextPeriodStart.Format("2006-01-02") != "2026-10-19" {
		t.Fatalf("next period=%v err=%v, want one cadence after the published period", schedule.NextPeriodStart, err)
	}
}

func TestSchedulingLifecycle(t *testing.T) {
	store, pool := openTestStore(t)
	ctx := context.Background()

	if _, err := store.CreateTeam(ctx, "guild", "Raid Night", "leader", "Too Big", "Asia/Tokyo", []string{"1", "2", "3", "4", "5", "6", "7", "8"}); err == nil {
		t.Fatal("expected eight-member limit error")
	}
	created, err := store.CreateTeam(ctx, "guild", "Raid Night", "leader", "The Echo", "Asia/Tokyo", []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	location, _ := time.LoadLocation("Asia/Tokyo")
	periodStart := time.Date(2026, 10, 5, 0, 0, 0, 0, location)
	publishAt := periodStart.Add(-24 * time.Hour)
	draft := team.ScheduleDraft{
		TeamID:           created.ID,
		UserID:           "leader",
		CadenceDays:      7,
		Weekdays:         []time.Weekday{time.Monday},
		StartMinutes:     21 * 60,
		EndMinutes:       23 * 60,
		FirstPeriodStart: periodStart,
		FirstPublishAt:   publishAt,
		ChannelID:        "channel",
		ChannelName:      "raid-schedule",
	}
	if err := store.SaveSchedule(ctx, draft); err != nil {
		t.Fatal(err)
	}
	savedSchedule, err := store.Schedule(ctx, created.ID)
	if err != nil || savedSchedule.PublishLeadDays != 3 || savedSchedule.ChannelName != "raid-schedule" || created.GuildName != "Raid Night" {
		t.Fatalf("saved destination guild=%q channel=%q lead=%d err=%v", created.GuildName, savedSchedule.ChannelName, savedSchedule.PublishLeadDays, err)
	}
	memberTeams, err := store.TeamsForUser(ctx, "a")
	if err != nil || len(memberTeams) != 1 || memberTeams[0].ID != created.ID {
		t.Fatalf("member teams=%+v err=%v", memberTeams, err)
	}
	var enabled bool
	if err := pool.QueryRow(ctx, `SELECT enabled FROM schedule_settings WHERE team_id=$1`, created.ID).Scan(&enabled); err != nil || !enabled {
		t.Fatalf("schedule enabled=%v err=%v", enabled, err)
	}
	poll, generated, err := store.PublishNext(ctx, created.ID, false, publishAt)
	if err != nil || !generated {
		t.Fatalf("PublishNext() generated=%v err=%v", generated, err)
	}
	if _, generated, err := store.PublishNext(ctx, created.ID, false, publishAt); err != nil || generated {
		t.Fatalf("second PublishNext() generated=%v err=%v", generated, err)
	}
	if err := store.SaveSchedule(ctx, draft); err != nil {
		t.Fatal(err)
	}
	if _, advanced, err := store.PublishNext(ctx, created.ID, false, publishAt); err != nil || !advanced {
		t.Fatalf("conflicting PublishNext() advanced=%v err=%v", advanced, err)
	}
	var pollCount int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedule_polls WHERE team_id=$1`, created.ID).Scan(&pollCount); err != nil || pollCount != 1 {
		t.Fatalf("idempotent poll count=%d err=%v", pollCount, err)
	}
	view, err := store.PollView(ctx, poll.ID)
	if err != nil {
		t.Fatal(err)
	}
	occurrence := view.Poll.Occurrences[0]
	now := occurrence.StartsAt.Add(-time.Hour)
	for _, member := range []string{"leader", "a"} {
		if err := store.SetAvailability(ctx, poll.ID, member, []int64{occurrence.ID}, now); err != nil {
			t.Fatal(err)
		}
	}
	view, err = store.PollView(ctx, poll.ID)
	if err != nil || pendingCount(view, occurrence.ID) != 1 {
		t.Fatalf("pending before roster removal=%d err=%v", pendingCount(view, occurrence.ID), err)
	}
	if err := store.UpdateTeam(ctx, created.ID, "leader", "The Echo", "Asia/Tokyo", []string{"a", "new"}); err != nil {
		t.Fatal(err)
	}
	view, err = store.PollView(ctx, poll.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Members) != 3 || !containsID(view.Members, "a") || !containsID(view.Members, "new") || containsID(view.Members, "b") || pendingCount(view, occurrence.ID) != 1 {
		t.Fatalf("active poll roster=%v pending=%d", view.Members, pendingCount(view, occurrence.ID))
	}
	if err := store.SetAvailability(ctx, poll.ID, "b", nil, now); !errors.Is(err, ErrForbidden) {
		t.Fatalf("removed member availability error = %v", err)
	}
	if err := store.SetAvailability(ctx, poll.ID, "outsider", nil, now); !errors.Is(err, ErrForbidden) {
		t.Fatalf("outsider availability error = %v", err)
	}
	if err := store.SetOccurrenceStatus(ctx, occurrence.ID, "outsider", "confirm", now); !errors.Is(err, ErrForbidden) {
		t.Fatalf("outsider status error = %v", err)
	}
	if err := store.SetOccurrenceStatus(ctx, occurrence.ID, "leader", "confirm", now); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, store, poll.ID, "attention_required")
	if err := store.SetAvailability(ctx, poll.ID, "new", []int64{occurrence.ID}, now); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, store, poll.ID, "confirmed")
	if err := store.SetAvailability(ctx, poll.ID, "a", nil, now); err != nil {
		t.Fatal(err)
	}
	view, err = store.PollView(ctx, poll.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Submitted["a"] || view.Availability[occurrence.ID]["a"] {
		t.Fatalf("omitted date must be an explicit unavailable submission: %+v", view.Availability[occurrence.ID])
	}
	assertStatus(t, store, poll.ID, "attention_required")
	if err := store.SetOccurrenceStatus(ctx, occurrence.ID, "leader", "cancel", now); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, store, poll.ID, "cancelled")
	if err := store.SetOccurrenceStatus(ctx, occurrence.ID, "leader", "reopen", now); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, store, poll.ID, "proposed")

	closed, err := store.CloseExpiredPolls(ctx, occurrence.StartsAt)
	if err != nil || len(closed) != 1 {
		t.Fatalf("CloseExpiredPolls() count=%d err=%v", len(closed), err)
	}
	if err := store.SetAvailability(ctx, poll.ID, "leader", nil, occurrence.StartsAt); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired availability error = %v", err)
	}
	if err := store.UpdateTeam(ctx, created.ID, "leader", "The Echo", "Asia/Tokyo", nil); err != nil {
		t.Fatal(err)
	}
	view, err = store.PollView(ctx, poll.ID)
	if err != nil || len(view.Members) != 3 || !containsID(view.Members, "a") || !containsID(view.Members, "new") {
		t.Fatalf("closed poll history changed: members=%v err=%v", view.Members, err)
	}
	if err := store.DeleteTeam(ctx, created.ID, "leader"); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schedule_polls WHERE team_id=$1`, created.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("poll cascade count=%d err=%v", count, err)
	}
}

func assertStatus(t *testing.T, store *Store, pollID int64, want string) {
	t.Helper()
	view, err := store.PollView(context.Background(), pollID)
	if err != nil {
		t.Fatal(err)
	}
	if got := view.Poll.Occurrences[0].Status; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func containsID(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func pendingCount(view team.PollView, occurrenceID int64) int {
	pending := len(view.Members)
	for _, memberID := range view.Members {
		if _, submitted := view.Availability[occurrenceID][memberID]; submitted {
			pending--
		}
	}
	return pending
}
