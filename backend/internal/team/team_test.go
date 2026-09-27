package team

import (
	"strings"
	"testing"
	"time"
)

func TestDraftOwnershipAndExpiry(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	drafts := NewDrafts()
	drafts.now = func() time.Time { return now }
	token, err := drafts.NewTeam(TeamDraft{GuildID: "guild", UserID: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := drafts.Team(token, "guild", "other"); err == nil || !strings.Contains(err.Error(), "another user") {
		t.Fatalf("expected ownership error, got %v", err)
	}
	now = now.Add(16 * time.Minute)
	if _, err := drafts.Team(token, "guild", "owner"); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expiry error, got %v", err)
	}
}

func TestNameAndWeekdayNormalization(t *testing.T) {
	if got := NormalizeName("  The   Echo  "); got != "the echo" {
		t.Fatalf("NormalizeName() = %q", got)
	}
	days, err := ParseWeekdays([]string{"5", "1", "5"})
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 2 || days[0] != time.Monday || days[1] != time.Friday {
		t.Fatalf("unexpected weekdays: %v", days)
	}
}

func TestGenerateOccurrences(t *testing.T) {
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	schedule := Schedule{
		Timezone:        "Asia/Tokyo",
		CadenceDays:     14,
		StartMinutes:    21 * 60,
		EndMinutes:      1 * 60,
		Weekdays:        []time.Weekday{time.Monday},
		NextPeriodStart: time.Date(2026, 10, 5, 0, 0, 0, 0, location),
	}
	occurrences, err := GenerateOccurrences(schedule)
	if err != nil {
		t.Fatal(err)
	}
	if len(occurrences) != 2 {
		t.Fatalf("got %d occurrences, want 2", len(occurrences))
	}
	if got := occurrences[0].StartsAt.Format(time.RFC3339); got != "2026-10-05T12:00:00Z" {
		t.Fatalf("unexpected UTC start: %s", got)
	}
	if occurrences[0].EndsAt.Sub(occurrences[0].StartsAt) != 4*time.Hour {
		t.Fatalf("overnight duration = %v", occurrences[0].EndsAt.Sub(occurrences[0].StartsAt))
	}
}

func TestGenerateOccurrencesRejectsDSTGap(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("timezone database unavailable")
	}
	_, err = GenerateOccurrences(Schedule{
		Timezone:        "America/New_York",
		CadenceDays:     7,
		StartMinutes:    2*60 + 30,
		EndMinutes:      4 * 60,
		Weekdays:        []time.Weekday{time.Sunday},
		NextPeriodStart: time.Date(2026, 3, 8, 0, 0, 0, 0, location),
	})
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected DST gap error, got %v", err)
	}
}

func TestParseSetupTimesRejectsDSTGap(t *testing.T) {
	if _, err := time.LoadLocation("America/New_York"); err != nil {
		t.Skip("timezone database unavailable")
	}
	_, _, err := ParseSetupTimes("America/New_York", "2026-03-08", "2026-03-08 02:30")
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected DST gap error, got %v", err)
	}
}

func TestPollExpirationBoundary(t *testing.T) {
	start := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	poll := Poll{Occurrences: []Occurrence{{StartsAt: start}}}
	if poll.Expired(start.Add(-time.Nanosecond)) {
		t.Fatal("poll expired before its final occurrence")
	}
	if !poll.Expired(start) {
		t.Fatal("poll must expire when its final occurrence starts")
	}
}
