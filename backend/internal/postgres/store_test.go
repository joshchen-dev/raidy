package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/joshchen-dev/raidy/internal/team"
)

func TestSchedulingLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema, err := os.ReadFile("../../migrations/001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE teams CASCADE`); err != nil {
		t.Fatal(err)
	}
	store := &Store{pool: pool}

	if _, err := store.CreateTeam(ctx, "guild", "leader", "Too Big", "Asia/Tokyo", []string{"1", "2", "3", "4", "5", "6", "7", "8"}); err == nil {
		t.Fatal("expected eight-member limit error")
	}
	created, err := store.CreateTeam(ctx, "guild", "leader", "The Echo", "Asia/Tokyo", []string{"a", "b"})
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
	}
	if err := store.SaveSchedule(ctx, draft); err != nil {
		t.Fatal(err)
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
	if err := store.ReplaceRoster(ctx, created.ID, "leader", []string{"new"}); err != nil {
		t.Fatal(err)
	}
	view, err := store.PollView(ctx, poll.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Members) != 3 || !containsID(view.Members, "a") || containsID(view.Members, "new") {
		t.Fatalf("poll roster was not snapshotted: %v", view.Members)
	}
	occurrence := view.Poll.Occurrences[0]
	now := occurrence.StartsAt.Add(-time.Hour)
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
	for _, member := range view.Members {
		if err := store.SetAvailability(ctx, poll.ID, member, []int64{occurrence.ID}, now); err != nil {
			t.Fatal(err)
		}
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
