package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/joshchen-dev/raidy/internal/postgres"
	"github.com/joshchen-dev/raidy/internal/team"
)

func TestFailMapsErrorsToStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		status   int
		contains string
	}{
		{"already published", postgres.ErrAlreadyPublished, http.StatusConflict, "Republish"},
		{"duplicate team", postgres.ErrDuplicateTeam, http.StatusConflict, "already exists"},
		{"validation", fmt.Errorf("save: %w", postgres.ValidationError{Message: "select at least one weekday"}), http.StatusBadRequest, "select at least one weekday"},
		{"timeout", context.DeadlineExceeded, http.StatusGatewayTimeout, "timed out"},
		{"database", &pgconn.PgError{Code: "57P01", Message: "terminating connection"}, http.StatusInternalServerError, "internal server error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &Handler{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
			response := httptest.NewRecorder()
			h.fail(response, httptest.NewRequest(http.MethodPost, "/api/teams", nil), test.err)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.contains) {
				t.Fatalf("fail(%v) = %d %s, want %d containing %q", test.err, response.Code, response.Body.String(), test.status, test.contains)
			}
			if test.status == http.StatusInternalServerError && strings.Contains(response.Body.String(), "terminating") {
				t.Fatal("internal error details leaked to the client")
			}
		})
	}
	if !errors.Is(fmt.Errorf("wrapped: %w", postgres.ErrDuplicateTeam), postgres.ErrDuplicateTeam) {
		t.Fatal("ErrDuplicateTeam must survive wrapping")
	}
}

func TestPollJSONCountsVotesPerOccurrence(t *testing.T) {
	start := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	view := team.PollView{
		Poll: team.Poll{ID: 9, PeriodStart: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), PeriodEnd: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Tokyo",
			Occurrences: []team.Occurrence{{ID: 1, StartsAt: start, EndsAt: start.Add(2 * time.Hour), Status: "proposed"}}},
		Members:      []string{"a", "b", "c"},
		Availability: map[int64]map[string]bool{1: {"a": true, "b": false}},
	}
	got := pollJSON(view)
	if got.ID != 9 || got.PeriodEnd != "2026-10-11" {
		t.Fatalf("poll = %+v, want id 9 ending on the inclusive last day", got)
	}
	occurrence := got.Occurrences[0]
	if occurrence.Available != 1 || occurrence.Unavailable != 1 || occurrence.Pending != 1 || len(occurrence.Votes) != 3 {
		t.Fatalf("occurrence counts = %+v", occurrence)
	}
}

type countingTransport struct{ calls int }

func (c *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	c.calls++
	return nil, errors.New("network disabled in tests")
}

func TestTeamResponseUsesCachedMembers(t *testing.T) {
	session, err := discordgo.New("Bot test")
	if err != nil {
		t.Fatal(err)
	}
	transport := &countingTransport{}
	session.Client = &http.Client{Transport: transport}
	session.State.User = &discordgo.User{ID: "bot"}
	if err := session.State.GuildAdd(&discordgo.Guild{ID: "guild", Members: []*discordgo.Member{
		{GuildID: "guild", Nick: "Alice", User: &discordgo.User{ID: "a", Username: "alice"}},
	}}); err != nil {
		t.Fatal(err)
	}
	h := &Handler{discord: session}
	response := h.teamResponse(context.Background(), team.Team{ID: 1, GuildID: "guild", LeaderID: "a", MemberIDs: []string{"a", "missing"}}, "a")
	if transport.calls != 1 {
		t.Fatalf("REST calls = %d, want 1 (only the uncached member)", transport.calls)
	}
	if response.Members[0].Name != "Alice" || response.Members[1].Name != "missing" {
		t.Fatalf("members = %+v", response.Members)
	}
}
