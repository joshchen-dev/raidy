package discord

import (
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/joshchen-dev/raidy/internal/team"
)

func TestTimezoneOptionsFitDiscordLimit(t *testing.T) {
	options := timezoneOptions()
	if len(options) != 25 {
		t.Fatalf("timezone options = %d, want 25", len(options))
	}
	defaults, other := 0, false
	for _, option := range options {
		if option.Default {
			defaults++
			if option.Value != "Asia/Tokyo" {
				t.Fatalf("default timezone = %q", option.Value)
			}
		}
		if option.Value == "other" {
			other = true
			continue
		}
		if _, err := time.LoadLocation(option.Value); err != nil {
			t.Fatalf("invalid timezone %q: %v", option.Value, err)
		}
	}
	if defaults != 1 || !other {
		t.Fatalf("defaults=%d other=%v", defaults, other)
	}
}

func TestScheduleDefaultsUseNextMonday(t *testing.T) {
	draft := team.ScheduleDraft{Timezone: "Asia/Tokyo"}
	if err := setScheduleDefaults(&draft, time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if got := draft.FirstPeriodStart.Format("2006-01-02"); got != "2026-10-05" {
		t.Fatalf("first period = %s", got)
	}
	if draft.StartMinutes != 21*60 || draft.EndMinutes != 23*60 || draft.PublishLeadDays != 3 {
		t.Fatalf("unexpected defaults: %+v", draft)
	}
}

func TestClockAndDateOptions(t *testing.T) {
	if got := len(clockOptions(24, 1, 21)); got != 24 {
		t.Fatalf("hour options = %d", got)
	}
	minutes := clockOptions(60, 15, 30)
	if len(minutes) != 4 || !minutes[2].Default {
		t.Fatalf("minute options = %+v", minutes)
	}
	location, _ := time.LoadLocation("Asia/Tokyo")
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, location)
	components := scheduleDateComponents("token", team.ScheduleDraft{Timezone: "Asia/Tokyo", DatePageStart: start, FirstPeriodStart: start.AddDate(0, 0, 6)})
	row := components[0].(discordgo.ActionsRow)
	menu := row.Components[0].(discordgo.SelectMenu)
	if len(menu.Options) != 25 || menu.Options[0].Value != "2026-10-01" || menu.Options[24].Value != "2026-10-25" || !menu.Options[6].Default {
		t.Fatalf("unexpected date page: %+v", menu.Options)
	}
}
