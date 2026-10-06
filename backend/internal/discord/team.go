package discord

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/joshchen-dev/raidy/internal/team"
)

func (b *Bot) openTeamSetup(i *discordgo.Interaction) error {
	return b.modal(i, "team_setup", "Create a static team",
		discordgo.TextInput{CustomID: "name", Label: "Team name", Style: discordgo.TextInputShort, Required: true, MaxLength: 80},
	)
}

func (b *Bot) submitTeamSetup(i *discordgo.Interaction, values map[string]string) error {
	name := team.DisplayName(values["name"])
	if name == "" {
		return errors.New("team name is required")
	}
	token, err := b.Drafts.NewTeam(team.TeamDraft{GuildID: i.GuildID, UserID: userID(i), Name: name, Timezone: "Asia/Tokyo"})
	if err != nil {
		return err
	}
	return b.ephemeral(i, "Choose the team's timezone. `Asia/Tokyo` is selected by default.", timezoneComponents(token))
}

func timezoneComponents(token string) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{CustomID: "team_timezone:" + token, Placeholder: "Choose a timezone", MinValues: intPtr(1), MaxValues: 1, Options: timezoneOptions()},
		}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "team_timezone_default:" + token, Label: "Use Asia/Tokyo", Style: discordgo.PrimaryButton},
			discordgo.Button{CustomID: "team_cancel:" + token, Label: "Cancel", Style: discordgo.SecondaryButton},
		}},
	}
}

func timezoneOptions() []discordgo.SelectMenuOption {
	zones := []struct{ label, value string }{
		{"UTC", "UTC"},
		{"Pacific Time — Los Angeles", "America/Los_Angeles"},
		{"Mountain Time — Denver", "America/Denver"},
		{"Central Time — Chicago", "America/Chicago"},
		{"Eastern Time — New York", "America/New_York"},
		{"Toronto", "America/Toronto"},
		{"Vancouver", "America/Vancouver"},
		{"São Paulo", "America/Sao_Paulo"},
		{"London", "Europe/London"},
		{"Paris", "Europe/Paris"},
		{"Berlin", "Europe/Berlin"},
		{"Amsterdam", "Europe/Amsterdam"},
		{"Madrid", "Europe/Madrid"},
		{"Rome", "Europe/Rome"},
		{"Warsaw", "Europe/Warsaw"},
		{"Tokyo", "Asia/Tokyo"},
		{"Seoul", "Asia/Seoul"},
		{"Shanghai", "Asia/Shanghai"},
		{"Hong Kong", "Asia/Hong_Kong"},
		{"Taipei", "Asia/Taipei"},
		{"Singapore", "Asia/Singapore"},
		{"Bangkok", "Asia/Bangkok"},
		{"Sydney", "Australia/Sydney"},
		{"Auckland", "Pacific/Auckland"},
		{"Other timezone…", "other"},
	}
	options := make([]discordgo.SelectMenuOption, 0, len(zones))
	for _, zone := range zones {
		options = append(options, discordgo.SelectMenuOption{Label: zone.label, Value: zone.value, Default: zone.value == "Asia/Tokyo"})
	}
	return options
}

func (b *Bot) selectTeamTimezone(i *discordgo.Interaction, token string, values []string) error {
	if len(values) != 1 {
		return errors.New("choose one timezone")
	}
	if values[0] == "other" {
		if _, err := b.Drafts.Team(token, i.GuildID, userID(i)); err != nil {
			return err
		}
		return b.modal(i, "team_timezone_custom:"+token, "Custom timezone",
			discordgo.TextInput{CustomID: "timezone", Label: "IANA timezone", Style: discordgo.TextInputShort, Required: true, Placeholder: "America/Phoenix", MaxLength: 80},
		)
	}
	return b.saveTeamTimezone(i, token, values[0], true)
}

func (b *Bot) submitCustomTimezone(i *discordgo.Interaction, token string, values map[string]string) error {
	return b.saveTeamTimezone(i, token, values["timezone"], false)
}

func (b *Bot) saveTeamTimezone(i *discordgo.Interaction, token, timezone string, update bool) error {
	draft, err := b.Drafts.Team(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	timezone = strings.TrimSpace(timezone)
	if _, err := time.LoadLocation(timezone); err != nil {
		return fmt.Errorf("unknown timezone %q", timezone)
	}
	draft.Timezone = timezone
	b.Drafts.SaveTeam(token, draft)
	content := "Select up to seven teammates. You are included automatically."
	if update {
		return b.update(i, content, rosterComponents(token, nil))
	}
	return b.ephemeral(i, content, rosterComponents(token, nil))
}

func (b *Bot) openTeamManage(ctx context.Context, i *discordgo.Interaction) error {
	teams, err := b.Store.TeamsLedBy(ctx, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	if len(teams) == 0 {
		return errors.New("you do not lead a team; run /team setup first")
	}
	if len(teams) == 1 {
		return b.beginRosterManagement(i, teams[0], false)
	}
	components := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.SelectMenu{CustomID: "team_manage_pick", Placeholder: "Choose a team", Options: teamOptions(teams), MinValues: intPtr(1), MaxValues: 1},
	}}}
	return b.ephemeral(i, "Choose the team to manage.", components)
}

func (b *Bot) pickManagedTeam(ctx context.Context, i *discordgo.Interaction, values []string) error {
	if len(values) != 1 {
		return errors.New("choose one team")
	}
	id, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		return errors.New("invalid team")
	}
	value, err := b.ownsTeam(ctx, i.GuildID, userID(i), id)
	if err != nil {
		return err
	}
	return b.beginRosterManagement(i, value, true)
}

func (b *Bot) beginRosterManagement(i *discordgo.Interaction, value team.Team, update bool) error {
	draft := team.TeamDraft{TeamID: value.ID, GuildID: value.GuildID, GuildName: value.GuildName, UserID: value.LeaderID, Name: value.Name, Timezone: value.Timezone, MemberIDs: value.MemberIDs}
	token, err := b.Drafts.NewTeam(draft)
	if err != nil {
		return err
	}
	defaults := make([]string, 0, len(value.MemberIDs)-1)
	for _, id := range value.MemberIDs {
		if id != value.LeaderID {
			defaults = append(defaults, id)
		}
	}
	content := "Edit **" + value.Name + "**. Select up to seven teammates; you remain the leader."
	if update {
		return b.update(i, content, rosterComponents(token, defaults))
	}
	return b.ephemeral(i, content, rosterComponents(token, defaults))
}

func rosterComponents(token string, defaults []string) []discordgo.MessageComponent {
	defaultValues := make([]discordgo.SelectMenuDefaultValue, 0, len(defaults))
	for _, id := range defaults {
		defaultValues = append(defaultValues, discordgo.SelectMenuDefaultValue{ID: id, Type: discordgo.SelectMenuDefaultValueUser})
	}
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{MenuType: discordgo.UserSelectMenu, CustomID: "team_roster:" + token,
				Placeholder: "Choose teammates", MinValues: intPtr(0), MaxValues: 7, DefaultValues: defaultValues},
		}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "team_roster_empty:" + token, Label: "Use leader only", Style: discordgo.SecondaryButton},
		}},
	}
}

func (b *Bot) selectTeamRoster(i *discordgo.Interaction, token string, selected []string) error {
	draft, err := b.Drafts.Team(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	selected = uniqueOthers(selected, draft.UserID)
	draft.MemberIDs = append([]string(nil), selected...)
	b.Drafts.SaveTeam(token, draft)
	action := "Create"
	if draft.TeamID != 0 {
		action = "Save roster"
	}
	components := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: "team_commit:" + token, Label: action, Style: discordgo.SuccessButton},
		discordgo.Button{CustomID: "team_cancel:" + token, Label: "Cancel", Style: discordgo.SecondaryButton},
	}}}
	if draft.TeamID != 0 {
		components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "team_delete:" + strconv.FormatInt(draft.TeamID, 10), Label: "Delete team", Style: discordgo.DangerButton},
		}})
	}
	members := append([]string{draft.UserID}, selected...)
	return b.update(i, fmt.Sprintf("**%s**\nTimezone: `%s`\nRoster (%d/8): %s", draft.Name, draft.Timezone, len(members), mentionList(members)), components)
}

func uniqueOthers(values []string, leaderID string) []string {
	seen := map[string]bool{leaderID: true}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func (b *Bot) commitTeam(ctx context.Context, i *discordgo.Interaction, token string) error {
	draft, err := b.Drafts.Team(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	if draft.TeamID == 0 {
		guild, guildErr := b.Session.State.Guild(draft.GuildID)
		if guildErr != nil {
			guild, guildErr = b.Session.Guild(draft.GuildID)
		}
		if guildErr != nil {
			return guildErr
		}
		created, err := b.Store.CreateTeam(ctx, draft.GuildID, guild.Name, draft.UserID, draft.Name, draft.Timezone, draft.MemberIDs)
		if err != nil {
			return err
		}
		draft.TeamID = created.ID
	} else {
		if err := b.Store.ReplaceRoster(ctx, draft.TeamID, draft.UserID, draft.MemberIDs); err != nil {
			return err
		}
		if err := b.RefreshTeamPoll(ctx, draft.TeamID); err != nil {
			b.Log.Error("poll refresh after roster update failed", "team_id", draft.TeamID, "error", err)
		}
	}
	b.Drafts.DeleteTeam(token)
	return b.update(i, "Saved **"+draft.Name+"**.", nil)
}

func (b *Bot) confirmDeleteTeam(ctx context.Context, i *discordgo.Interaction, teamID int64) error {
	if err := invalidID(teamID); err != nil {
		return err
	}
	value, err := b.ownsTeam(ctx, i.GuildID, userID(i), teamID)
	if err != nil {
		return err
	}
	components := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: "team_delete_confirm:" + strconv.FormatInt(teamID, 10), Label: "Permanently delete", Style: discordgo.DangerButton},
	}}}
	return b.update(i, "Delete **"+value.Name+"** and all of its timetable history? This cannot be undone.", components)
}

func (b *Bot) deleteTeam(ctx context.Context, i *discordgo.Interaction, teamID int64) error {
	if err := invalidID(teamID); err != nil {
		return err
	}
	if _, err := b.ownsTeam(ctx, i.GuildID, userID(i), teamID); err != nil {
		return err
	}
	if err := b.Store.DeleteTeam(ctx, teamID, userID(i)); err != nil {
		return err
	}
	return b.update(i, "Team and stored timetable data deleted.", nil)
}
