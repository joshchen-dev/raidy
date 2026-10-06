package postgres

import (
	"context"
	"errors"
	"fmt"
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

func TestPublishedOpenPollIDs(t *testing.T) {
	store, pool := openTestStore(t)
	ctx := context.Background()
	created, err := store.CreateTeam(ctx, "guild", "Raid Night", "leader", "The Echo", "Asia/Tokyo", nil)
	if err != nil {
		t.Fatal(err)
	}
	var published, unpublished, closed int64
	for _, row := range []struct {
		id      *int64
		start   string
		message string
		closed  bool
	}{{&published, "2026-10-05", "m1", false}, {&unpublished, "2026-10-12", "", false}, {&closed, "2026-09-28", "m0", true}} {
		if err := pool.QueryRow(ctx, `
			INSERT INTO schedule_polls (team_id, period_start, period_end, channel_id, message_id, closed_at)
			VALUES ($1, $2::date, $2::date + 7, 'channel', $3, CASE WHEN $4 THEN now() END) RETURNING id`,
			created.ID, row.start, row.message, row.closed).Scan(row.id); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := store.PublishedOpenPollIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != published {
		t.Fatalf("PublishedOpenPollIDs() = %v, want only %d (unpublished %d and closed %d excluded)", ids, published, unpublished, closed)
	}
}

func TestCatchUpSkipsElapsedPeriods(t *testing.T) {
	store, pool := openTestStore(t)
	ctx := context.Background()
	created, err := store.CreateTeam(ctx, "guild", "Raid Night", "leader", "The Echo", "Asia/Tokyo", nil)
	if err != nil {
		t.Fatal(err)
	}
	location, _ := time.LoadLocation("Asia/Tokyo")
	// The bot was offline for three weeks: periods starting Sep 7, 14, and 21
	// are over, and the period starting Sep 28 still has Wednesday ahead.
	firstPeriod := time.Date(2026, 9, 7, 0, 0, 0, 0, location)
	if err := store.SaveSchedule(ctx, team.ScheduleDraft{
		TeamID: created.ID, UserID: "leader", CadenceDays: 7, Weekdays: []time.Weekday{time.Wednesday},
		StartMinutes: 21 * 60, EndMinutes: 23 * 60, FirstPeriodStart: firstPeriod,
		FirstPublishAt: firstPeriod.Add(-3 * 24 * time.Hour), ChannelID: "channel",
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, location)
	for i := 0; i < 10; i++ {
		_, advanced, err := store.PublishNext(ctx, created.ID, false, now)
		if err != nil {
			t.Fatal(err)
		}
		if !advanced {
			break
		}
	}
	rows, err := pool.Query(ctx, `SELECT period_start FROM schedule_polls WHERE team_id=$1 ORDER BY period_start`, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	var periods []string
	for rows.Next() {
		var start time.Time
		if err := rows.Scan(&start); err != nil {
			t.Fatal(err)
		}
		periods = append(periods, start.Format("2006-01-02"))
	}
	if len(periods) != 1 || periods[0] != "2026-09-28" {
		t.Fatalf("polls created while catching up = %v, want only [2026-09-28]", periods)
	}
	schedule, err := store.Schedule(ctx, created.ID)
	if err != nil || schedule.NextPeriodStart.Format("2006-01-02") != "2026-10-05" {
		t.Fatalf("next period = %v err=%v, want 2026-10-05", schedule.NextPeriodStart, err)
	}
}

func TestWebSessionsPersistAndExpire(t *testing.T) {
	store, pool := openTestStore(t)
	ctx := context.Background()
	// The web package's session tests share this table, so use unique keys
	// instead of truncating it.
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	key := func(name string) []byte {
		return []byte(fmt.Sprintf("%s-%s-%d", t.Name(), name, time.Now().UnixNano()))
	}
	live, expired, another := key("live"), key("expired"), key("another")
	if err := store.PutSession(ctx, live, []byte(`{"user":{"id":"u1"}}`), now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if err := store.PutSession(ctx, expired, []byte(`{}`), now.Add(-time.Minute), now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	data, err := store.Session(ctx, live, now)
	if err != nil || string(data) != `{"user": {"id": "u1"}}` {
		t.Fatalf("Session(live) = %s, %v", data, err)
	}
	if _, err := store.Session(ctx, expired, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Session(expired) error = %v, want ErrNotFound", err)
	}
	// Writing a new session prunes expired rows.
	if err := store.PutSession(ctx, another, []byte(`{}`), now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM web_sessions WHERE token_hash=$1`, expired).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("expired rows=%d err=%v, want pruned", rows, err)
	}
	if err := store.DeleteSession(ctx, live); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Session(ctx, live, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Session(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestOpenPollsListsEveryActivePeriodInOrder(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	created, err := store.CreateTeam(ctx, "guild", "Raid Night", "leader", "The Echo", "Asia/Tokyo", nil)
	if err != nil {
		t.Fatal(err)
	}
	location, _ := time.LoadLocation("Asia/Tokyo")
	periodStart := time.Date(2026, 10, 5, 0, 0, 0, 0, location)
	publishAt := periodStart.Add(-3 * 24 * time.Hour)
	if err := store.SaveSchedule(ctx, team.ScheduleDraft{
		TeamID: created.ID, UserID: "leader", CadenceDays: 7, Weekdays: []time.Weekday{time.Wednesday},
		StartMinutes: 21 * 60, EndMinutes: 23 * 60, FirstPeriodStart: periodStart, FirstPublishAt: publishAt, ChannelID: "channel",
	}); err != nil {
		t.Fatal(err)
	}
	if _, created, err := store.PublishNext(ctx, created.ID, false, publishAt); err != nil || !created {
		t.Fatalf("scheduled PublishNext() created=%v err=%v", created, err)
	}
	// During the current period, the leader opens next week's voting early.
	duringPeriod := periodStart.Add(24 * time.Hour)
	if _, created, err := store.PublishNext(ctx, created.ID, true, duringPeriod); err != nil || !created {
		t.Fatalf("early PublishNext() created=%v err=%v", created, err)
	}
	views, err := store.OpenPolls(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 2 || views[0].Poll.PeriodStart.Format("2006-01-02") != "2026-10-05" || views[1].Poll.PeriodStart.Format("2006-01-02") != "2026-10-12" {
		t.Fatalf("OpenPolls() periods = %v, want 2026-10-05 then 2026-10-12", periodStarts(views))
	}
	if len(views[0].Poll.Occurrences) == 0 {
		t.Fatal("OpenPolls() must include occurrences")
	}
}

func periodStarts(views []team.PollView) []string {
	result := make([]string, len(views))
	for i, view := range views {
		result[i] = view.Poll.PeriodStart.Format("2006-01-02")
	}
	return result
}

func TestTeamErrorsAreTyped(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	if _, err := store.CreateTeam(ctx, "guild", "Raid Night", "leader", "The Echo", "Asia/Tokyo", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateTeam(ctx, "guild", "Raid Night", "other", " the echo ", "Asia/Tokyo", nil); !errors.Is(err, ErrDuplicateTeam) {
		t.Fatalf("duplicate CreateTeam() error = %v, want ErrDuplicateTeam", err)
	}
	second, err := store.CreateTeam(ctx, "guild", "Raid Night", "leader", "Second", "Asia/Tokyo", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTeam(ctx, second.ID, "leader", "The Echo", "Asia/Tokyo", nil); !errors.Is(err, ErrDuplicateTeam) {
		t.Fatalf("duplicate UpdateTeam() error = %v, want ErrDuplicateTeam", err)
	}
	var validation ValidationError
	if _, err := store.CreateTeam(ctx, "guild", "Raid Night", "leader", "Too Big", "Asia/Tokyo", []string{"1", "2", "3", "4", "5", "6", "7", "8"}); !errors.As(err, &validation) {
		t.Fatalf("oversized roster error = %v, want ValidationError", err)
	}
	if err := store.SaveSchedule(ctx, team.ScheduleDraft{TeamID: second.ID, UserID: "leader", CadenceDays: 3}); !errors.As(err, &validation) {
		t.Fatalf("invalid cadence error = %v, want ValidationError", err)
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
	var validation ValidationError
	if err := store.SetAvailability(ctx, poll.ID, "a", []int64{999999}, now); !errors.As(err, &validation) {
		t.Fatalf("foreign occurrence availability error = %v, want ValidationError", err)
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
