package discord

import (
	"strings"
	"testing"
	"time"

	"github.com/joshchen-dev/raidy/internal/team"
)

func TestRenderPollShowsMemberVotes(t *testing.T) {
	view := team.PollView{
		Poll: team.Poll{
			ID: 1, TeamName: "The Echo", Timezone: "Asia/Tokyo",
			PeriodStart: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
			PeriodEnd:   time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
			Occurrences: []team.Occurrence{{ID: 2, StartsAt: time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC), Status: "proposed"}},
		},
		Members:      []string{"available", "unavailable", "pending"},
		Availability: map[int64]map[string]bool{2: {"available": true, "unavailable": false}},
	}
	embeds, _ := renderPoll(view, true)
	value := embeds[0].Fields[0].Value
	for _, want := range []string{"Available: <@available>", "Unavailable: <@unavailable>", "Pending: <@pending>"} {
		if !strings.Contains(value, want) {
			t.Fatalf("rendered poll %q does not contain %q", value, want)
		}
	}
}
