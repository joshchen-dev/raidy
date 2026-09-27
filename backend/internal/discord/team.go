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
		discordgo.TextInput{CustomID: "timezone", Label: "IANA timezone", Style: discordgo.TextInputShort, Required: true, Value: "Asia/Tokyo", MaxLength: 80},
	)
}

func (b *Bot) submitTeamSetup(i *discordgo.Interaction, values map[string]string) error {
	name := team.DisplayName(values["name"])
	if name == "" {
		return errors.New("team name is required")
	}
	zone := strings.TrimSpace(values["timezone"])
	if _, err := time.LoadLocation(zone); err != nil {
		return fmt.Errorf("unknown timezone %q", zone)
	}
	token, err := b.Drafts.NewTeam(team.TeamDraft{GuildID: i.GuildID, UserID: userID(i), Name: name, Timezone: zone})
	if err != nil {
		return err
	}
	return b.ephemeral(i, "Select up to seven teammates. You are included automatically.", rosterComponents(token, nil))
}

func (b *Bot) openTeamManage(i *discordgo.Interaction) error {
	teams, err := b.Store.TeamsLedBy(context.Background(), i.GuildID, userID(i))
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

func (b *Bot) pickManagedTeam(i *discordgo.Interaction, values []string) error {
	if len(values) != 1 {
		return errors.New("choose one team")
	}
	id, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		return errors.New("invalid team")
	}
	value, err := b.ownsTeam(context.Background(), i.GuildID, userID(i), id)
	if err != nil {
		return err
	}
	return b.beginRosterManagement(i, value, true)
}

func (b *Bot) beginRosterManagement(i *discordgo.Interaction, value team.Team, update bool) error {
	draft := team.TeamDraft{TeamID: value.ID, GuildID: value.GuildID, UserID: value.LeaderID, Name: value.Name, Timezone: value.Timezone, MemberIDs: value.MemberIDs}
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

func (b *Bot) commitTeam(i *discordgo.Interaction, token string) error {
	draft, err := b.Drafts.Team(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	if draft.TeamID == 0 {
		created, err := b.Store.CreateTeam(context.Background(), draft.GuildID, draft.UserID, draft.Name, draft.Timezone, draft.MemberIDs)
		if err != nil {
			return err
		}
		draft.TeamID = created.ID
	} else if err := b.Store.ReplaceRoster(context.Background(), draft.TeamID, draft.UserID, draft.MemberIDs); err != nil {
		return err
	}
	b.Drafts.DeleteTeam(token)
	return b.update(i, "Saved **"+draft.Name+"**.", nil)
}

func (b *Bot) confirmDeleteTeam(i *discordgo.Interaction, teamID int64) error {
	if err := invalidID(teamID); err != nil {
		return err
	}
	value, err := b.ownsTeam(context.Background(), i.GuildID, userID(i), teamID)
	if err != nil {
		return err
	}
	components := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: "team_delete_confirm:" + strconv.FormatInt(teamID, 10), Label: "Permanently delete", Style: discordgo.DangerButton},
	}}}
	return b.update(i, "Delete **"+value.Name+"** and all of its timetable history? This cannot be undone.", components)
}

func (b *Bot) deleteTeam(i *discordgo.Interaction, teamID int64) error {
	if err := invalidID(teamID); err != nil {
		return err
	}
	if _, err := b.ownsTeam(context.Background(), i.GuildID, userID(i), teamID); err != nil {
		return err
	}
	if err := b.Store.DeleteTeam(context.Background(), teamID, userID(i)); err != nil {
		return err
	}
	return b.update(i, "Team and stored timetable data deleted.", nil)
}
