package discord

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/joshchen-dev/raidy/internal/team"
)

func (b *Bot) openScheduleSetup(i *discordgo.Interaction) error {
	teams, err := b.Store.TeamsLedBy(context.Background(), i.GuildID, userID(i))
	if err != nil {
		return err
	}
	if len(teams) == 0 {
		return errors.New("you do not lead a team; run /team setup first")
	}
	if len(teams) == 1 {
		return b.startScheduleDraft(i, teams[0], false)
	}
	token, err := b.Drafts.NewSchedule(team.ScheduleDraft{GuildID: i.GuildID, UserID: userID(i), ChannelID: i.ChannelID})
	if err != nil {
		return err
	}
	components := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.SelectMenu{CustomID: "schedule_team:" + token, Placeholder: "Choose a team", MinValues: intPtr(1), MaxValues: 1, Options: teamOptions(teams)},
	}}}
	return b.ephemeral(i, "Choose the team to schedule.", components)
}

func (b *Bot) pickScheduleTeam(i *discordgo.Interaction, token string, values []string) error {
	if len(values) != 1 {
		return errors.New("choose one team")
	}
	draft, err := b.Drafts.Schedule(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		return errors.New("invalid team")
	}
	value, err := b.ownsTeam(context.Background(), i.GuildID, userID(i), id)
	if err != nil {
		return err
	}
	draft.TeamID = value.ID
	draft.TeamName = value.Name
	draft.Timezone = value.Timezone
	b.Drafts.SaveSchedule(token, draft)
	return b.update(i, "Choose how much time each timetable covers.", cadenceComponents(token))
}

func (b *Bot) beginScheduleForTeam(i *discordgo.Interaction, teamID int64) error {
	if err := invalidID(teamID); err != nil {
		return err
	}
	value, err := b.ownsTeam(context.Background(), i.GuildID, userID(i), teamID)
	if err != nil {
		return err
	}
	return b.startScheduleDraft(i, value, true)
}

func (b *Bot) startScheduleDraft(i *discordgo.Interaction, value team.Team, update bool) error {
	token, err := b.Drafts.NewSchedule(team.ScheduleDraft{GuildID: i.GuildID, UserID: userID(i), TeamID: value.ID, TeamName: value.Name, Timezone: value.Timezone, ChannelID: i.ChannelID})
	if err != nil {
		return err
	}
	content := "Setting up **" + value.Name + "**. Choose how much time each timetable covers."
	if update {
		return b.update(i, content, cadenceComponents(token))
	}
	return b.ephemeral(i, content, cadenceComponents(token))
}

func cadenceComponents(token string) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: "schedule_cadence:" + token + ":7", Label: "Weekly", Style: discordgo.PrimaryButton},
		discordgo.Button{CustomID: "schedule_cadence:" + token + ":14", Label: "Biweekly", Style: discordgo.PrimaryButton},
		discordgo.Button{CustomID: "schedule_cancel:" + token, Label: "Cancel", Style: discordgo.SecondaryButton},
	}}}
}

func (b *Bot) pickCadence(i *discordgo.Interaction, token string, days int) error {
	if days != 7 && days != 14 {
		return errors.New("invalid cadence")
	}
	draft, err := b.Drafts.Schedule(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	draft.CadenceDays = days
	b.Drafts.SaveSchedule(token, draft)
	options := make([]discordgo.SelectMenuOption, 0, 7)
	for _, value := range team.WeekdayOptions() {
		options = append(options, discordgo.SelectMenuOption{Label: value.Label, Value: value.Value})
	}
	components := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.SelectMenu{CustomID: "schedule_weekdays:" + token, Placeholder: "Select raid weekdays", MinValues: intPtr(1), MaxValues: 7, Options: options},
	}}}
	return b.update(i, "Select every weekday on which the team may raid.", components)
}

func (b *Bot) pickWeekdays(i *discordgo.Interaction, token string, values []string) error {
	draft, err := b.Drafts.Schedule(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	draft.Weekdays, err = team.ParseWeekdays(values)
	if err != nil {
		return err
	}
	b.Drafts.SaveSchedule(token, draft)
	return b.modal(i, "schedule_times:"+token, "Timetable details",
		discordgo.TextInput{CustomID: "start", Label: "Start time (HH:MM)", Style: discordgo.TextInputShort, Required: true, Value: "21:00", MaxLength: 5},
		discordgo.TextInput{CustomID: "end", Label: "End time (HH:MM)", Style: discordgo.TextInputShort, Required: true, Value: "23:00", MaxLength: 5},
		discordgo.TextInput{CustomID: "period", Label: "First period start (YYYY-MM-DD)", Style: discordgo.TextInputShort, Required: true, Placeholder: "2026-10-05", MaxLength: 10},
		discordgo.TextInput{CustomID: "publish", Label: "First publish (YYYY-MM-DD HH:MM)", Style: discordgo.TextInputShort, Required: true, Placeholder: "2026-10-02 18:00", MaxLength: 16},
	)
}

func (b *Bot) submitScheduleTimes(i *discordgo.Interaction, token string, values map[string]string) error {
	draft, err := b.Drafts.Schedule(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	draft.StartMinutes, err = team.ParseClock(values["start"])
	if err != nil {
		return err
	}
	draft.EndMinutes, err = team.ParseClock(values["end"])
	if err != nil {
		return err
	}
	if draft.StartMinutes == draft.EndMinutes {
		return errors.New("start and end time must differ")
	}
	draft.FirstPeriodStart, draft.FirstPublishAt, err = team.ParseSetupTimes(draft.Timezone, values["period"], values["publish"])
	if err != nil {
		return err
	}
	occurrences, err := team.GenerateOccurrences(team.Schedule{Timezone: draft.Timezone, CadenceDays: draft.CadenceDays, StartMinutes: draft.StartMinutes, EndMinutes: draft.EndMinutes, Weekdays: draft.Weekdays, NextPeriodStart: draft.FirstPeriodStart})
	if err != nil {
		return err
	}
	if len(occurrences) == 0 || !draft.FirstPublishAt.Before(occurrences[0].StartsAt) {
		return errors.New("publication must be before the first raid starts")
	}
	b.Drafts.SaveSchedule(token, draft)
	var lines []string
	for _, o := range occurrences {
		lines = append(lines, "• <t:"+strconv.FormatInt(o.StartsAt.Unix(), 10)+":F>–<t:"+strconv.FormatInt(o.EndsAt.Unix(), 10)+":t>")
	}
	content := fmt.Sprintf("**%s** — %d-day timetable\nTimezone: `%s`\nChannel: <#%s>\nPublishes: <t:%d:F>\n%s", draft.TeamName, draft.CadenceDays, draft.Timezone, draft.ChannelID, draft.FirstPublishAt.Unix(), strings.Join(lines, "\n"))
	components := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: "schedule_commit:" + token, Label: "Activate", Style: discordgo.SuccessButton},
		discordgo.Button{CustomID: "schedule_cancel:" + token, Label: "Cancel", Style: discordgo.SecondaryButton},
	}}}
	return b.ephemeral(i, content, components)
}

func (b *Bot) commitSchedule(i *discordgo.Interaction, token string) error {
	draft, err := b.Drafts.Schedule(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	if err := b.Store.SaveSchedule(context.Background(), draft); err != nil {
		return err
	}
	b.Drafts.DeleteSchedule(token)
	return b.update(i, "Recurring timetable activated for **"+draft.TeamName+"**.", nil)
}

func (b *Bot) openScheduleManage(i *discordgo.Interaction) error {
	teams, err := b.Store.TeamsLedBy(context.Background(), i.GuildID, userID(i))
	if err != nil {
		return err
	}
	if len(teams) == 0 {
		return errors.New("you do not lead a team")
	}
	if len(teams) == 1 {
		return b.showScheduleManage(i, teams[0], false)
	}
	components := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.SelectMenu{CustomID: "schedule_manage_pick", Placeholder: "Choose a team", MinValues: intPtr(1), MaxValues: 1, Options: teamOptions(teams)},
	}}}
	return b.ephemeral(i, "Choose the timetable to manage.", components)
}

func (b *Bot) pickScheduleManage(i *discordgo.Interaction, values []string) error {
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
	return b.showScheduleManage(i, value, true)
}

func (b *Bot) showScheduleManage(i *discordgo.Interaction, value team.Team, update bool) error {
	id := strconv.FormatInt(value.ID, 10)
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "schedule_publish:" + id, Label: "Publish next now", Style: discordgo.PrimaryButton},
			discordgo.Button{CustomID: "schedule_republish:" + id, Label: "Republish latest", Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: "schedule_replace:" + id, Label: "Replace template", Style: discordgo.SecondaryButton},
		}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "schedule_enable:" + id, Label: "Enable automation", Style: discordgo.SuccessButton},
			discordgo.Button{CustomID: "schedule_disable:" + id, Label: "Pause automation", Style: discordgo.DangerButton},
		}},
	}
	content := "Manage **" + value.Name + "**."
	if update {
		return b.update(i, content, components)
	}
	return b.ephemeral(i, content, components)
}

func (b *Bot) publishNow(i *discordgo.Interaction, teamID int64) error {
	if err := invalidID(teamID); err != nil {
		return err
	}
	value, err := b.ownsTeam(context.Background(), i.GuildID, userID(i), teamID)
	if err != nil {
		return err
	}
	poll, created, err := b.Store.PublishNext(context.Background(), teamID, true, now())
	if err != nil {
		return err
	}
	if created {
		b.Log.Info("poll cycle advanced manually", "poll_id", poll.ID, "team_id", teamID)
	}
	if err := b.publishUnpublished(context.Background()); err != nil {
		return err
	}
	return b.update(i, "Published the next timetable for **"+value.Name+"**.", nil)
}

func (b *Bot) enableSchedule(i *discordgo.Interaction, teamID int64, enabled bool) error {
	if err := invalidID(teamID); err != nil {
		return err
	}
	value, err := b.ownsTeam(context.Background(), i.GuildID, userID(i), teamID)
	if err != nil {
		return err
	}
	if err := b.Store.SetScheduleEnabled(context.Background(), teamID, userID(i), enabled); err != nil {
		return err
	}
	state := "paused"
	if enabled {
		state = "enabled"
	}
	return b.update(i, "Automation "+state+" for **"+value.Name+"**.", nil)
}

func (b *Bot) republishLatest(i *discordgo.Interaction, teamID int64) error {
	if err := invalidID(teamID); err != nil {
		return err
	}
	if _, err := b.ownsTeam(context.Background(), i.GuildID, userID(i), teamID); err != nil {
		return err
	}
	view, err := b.Store.LatestPoll(context.Background(), teamID)
	if err != nil {
		return err
	}
	if view.Poll.MessageID != "" {
		_ = b.Session.ChannelMessageDelete(view.Poll.ChannelID, view.Poll.MessageID)
	}
	if err := b.Store.ClearPollMessage(context.Background(), view.Poll.ID, userID(i)); err != nil {
		return err
	}
	if err := b.publishUnpublished(context.Background()); err != nil {
		return err
	}
	return b.update(i, "Latest timetable republished.", nil)
}
