package discord

import (
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/joshchen-dev/raidy/internal/team"
)

func sampleView() team.PollView {
	return team.PollView{
		Poll: team.Poll{
			ID: 1, TeamID: 42, TeamName: "The Echo", Timezone: "Asia/Tokyo",
			PeriodStart: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
			PeriodEnd:   time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
			Occurrences: []team.Occurrence{
				{ID: 2, StartsAt: time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC), Status: "confirmed"},
				{ID: 3, StartsAt: time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC), Status: "proposed"},
			},
		},
		Members: []string{"a", "b", "c"},
		Availability: map[int64]map[string]bool{
			2: {"a": true, "b": true, "c": true},
			3: {"a": true, "b": false},
		},
	}
}

func TestAnnouncementShowsLocalTimesCountsAndWebLink(t *testing.T) {
	embeds, components := renderAnnouncement(sampleView(), true, "https://raidy.example")
	embed := embeds[0]
	if len(embed.Fields) != 2 {
		t.Fatalf("fields = %d, want one per date", len(embed.Fields))
	}
	// Discord timestamps render in every viewer's own timezone.
	if want := "<t:1791374400:F>"; !strings.Contains(embed.Fields[0].Name, want) {
		t.Fatalf("field name %q does not contain %q", embed.Fields[0].Name, want)
	}
	if got := embed.Fields[0].Value; !strings.Contains(got, "3/3 available") || !strings.Contains(got, "Confirmed") {
		t.Fatalf("confirmed date = %q", got)
	}
	if got := embed.Fields[1].Value; !strings.Contains(got, "1/3 available") || !strings.Contains(got, "Proposed") {
		t.Fatalf("proposed date = %q", got)
	}
	// Announcement only: no member mentions and no interactive buttons.
	for _, field := range embed.Fields {
		if strings.Contains(field.Value, "<@") {
			t.Fatalf("announcement mentions members: %q", field.Value)
		}
	}
	button := components[0].(discordgo.ActionsRow).Components[0].(discordgo.Button)
	if button.Style != discordgo.LinkButton || button.URL != "https://raidy.example/app?team=42" || button.CustomID != "" {
		t.Fatalf("button = %+v, want a link to the team page", button)
	}
}

func TestClosedAnnouncementKeepsHistoryWithoutButtons(t *testing.T) {
	embeds, components := renderAnnouncement(sampleView(), false, "https://raidy.example")
	if components != nil {
		t.Fatalf("closed announcement components = %v, want none", components)
	}
	if embeds[0].Footer == nil || !strings.Contains(embeds[0].Footer.Text, "closed") {
		t.Fatalf("closed announcement footer = %+v", embeds[0].Footer)
	}
}

func TestRaidyCommandLinksToTheMembersTeam(t *testing.T) {
	teams := []team.Team{
		{ID: 7, GuildID: "other", Name: "Elsewhere"},
		{ID: 9, GuildID: "guild", Name: "The Echo"},
	}
	if got := raidyReply("https://raidy.example/", "guild", teams); !strings.Contains(got, "https://raidy.example/app?team=9") || !strings.Contains(got, "The Echo") {
		t.Fatalf("reply = %q, want a link to the team in this server", got)
	}
	if got := raidyReply("https://raidy.example", "guild", nil); !strings.Contains(got, "https://raidy.example/app") {
		t.Fatalf("reply without teams = %q, want the app link", got)
	}
	if got := raidyReply("https://raidy.example", "guild", nil); !strings.Contains(got, "create") {
		t.Fatalf("reply without teams = %q, want a hint to create a team", got)
	}
}

func TestCommandsAreOnlyTheWebLink(t *testing.T) {
	commands := Commands()
	if len(commands) != 1 || commands[0].Name != "raidy" {
		t.Fatalf("commands = %+v, want only /raidy", commands)
	}
}
