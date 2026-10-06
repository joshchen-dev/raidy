package discord

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/joshchen-dev/raidy/internal/postgres"
	"github.com/joshchen-dev/raidy/internal/team"
)

func (b *Bot) openScheduleSetup(ctx context.Context, i *discordgo.Interaction) error {
	teams, err := b.Store.TeamsLedBy(ctx, i.GuildID, userID(i))
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

func (b *Bot) pickScheduleTeam(ctx context.Context, i *discordgo.Interaction, token string, values []string) error {
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
	value, err := b.ownsTeam(ctx, i.GuildID, userID(i), id)
	if err != nil {
		return err
	}
	draft.TeamID = value.ID
	draft.TeamName = value.Name
	draft.Timezone = value.Timezone
	if err := setScheduleDefaults(&draft, now()); err != nil {
		return err
	}
	b.Drafts.SaveSchedule(token, draft)
	return b.update(i, "Choose how much time each timetable covers.", cadenceComponents(token))
}

func (b *Bot) beginScheduleForTeam(ctx context.Context, i *discordgo.Interaction, teamID int64) error {
	if err := invalidID(teamID); err != nil {
		return err
	}
	value, err := b.ownsTeam(ctx, i.GuildID, userID(i), teamID)
	if err != nil {
		return err
	}
	return b.startScheduleDraft(i, value, true)
}

func (b *Bot) startScheduleDraft(i *discordgo.Interaction, value team.Team, update bool) error {
	draft := team.ScheduleDraft{GuildID: i.GuildID, UserID: userID(i), TeamID: value.ID, TeamName: value.Name, Timezone: value.Timezone, ChannelID: i.ChannelID}
	if err := setScheduleDefaults(&draft, now()); err != nil {
		return err
	}
	token, err := b.Drafts.NewSchedule(draft)
	if err != nil {
		return err
	}
	content := "Setting up **" + value.Name + "**. Choose how much time each timetable covers."
	if update {
		return b.update(i, content, cadenceComponents(token))
	}
	return b.ephemeral(i, content, cadenceComponents(token))
}

func setScheduleDefaults(draft *team.ScheduleDraft, at time.Time) error {
	today, err := localDate(draft.Timezone, at)
	if err != nil {
		return err
	}
	daysUntilMonday := (int(time.Monday) - int(today.Weekday()) + 7) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}
	draft.StartMinutes = 21 * 60
	draft.EndMinutes = 23 * 60
	draft.PublishLeadDays = 3
	draft.DatePageStart = today
	draft.FirstPeriodStart = today.AddDate(0, 0, daysUntilMonday)
	return nil
}

func localDate(timezone string, at time.Time) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, err
	}
	year, month, day := at.In(loc).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, loc), nil
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
	return b.update(i, scheduleTimeContent(draft), scheduleTimeComponents(token, draft))
}

func scheduleTimeContent(draft team.ScheduleDraft) string {
	return fmt.Sprintf("Choose the shared raid time. Current selection: **%s–%s** (`%s`).", team.FormatClock(draft.StartMinutes), team.FormatClock(draft.EndMinutes), draft.Timezone)
}

func scheduleTimeComponents(token string, draft team.ScheduleDraft) []discordgo.MessageComponent {
	selectRow := func(customID, placeholder string, options []discordgo.SelectMenuOption) discordgo.MessageComponent {
		return discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{CustomID: customID + ":" + token, Placeholder: placeholder, MinValues: intPtr(1), MaxValues: 1, Options: options},
		}}
	}
	return []discordgo.MessageComponent{
		selectRow("schedule_start_hour", "Start hour", clockOptions(24, 1, draft.StartMinutes/60)),
		selectRow("schedule_start_minute", "Start minute", clockOptions(60, 15, draft.StartMinutes%60)),
		selectRow("schedule_end_hour", "End hour", clockOptions(24, 1, draft.EndMinutes/60)),
		selectRow("schedule_end_minute", "End minute", clockOptions(60, 15, draft.EndMinutes%60)),
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "schedule_time_continue:" + token, Label: "Continue", Style: discordgo.PrimaryButton},
			discordgo.Button{CustomID: "schedule_cancel:" + token, Label: "Cancel", Style: discordgo.SecondaryButton},
		}},
	}
}

func clockOptions(limit, step, selected int) []discordgo.SelectMenuOption {
	options := make([]discordgo.SelectMenuOption, 0, limit/step)
	for value := 0; value < limit; value += step {
		label := fmt.Sprintf("%02d", value)
		options = append(options, discordgo.SelectMenuOption{Label: label, Value: strconv.Itoa(value), Default: value == selected})
	}
	return options
}

func (b *Bot) pickScheduleTime(i *discordgo.Interaction, token, field string, values []string) error {
	if len(values) != 1 {
		return errors.New("choose one time value")
	}
	value, err := strconv.Atoi(values[0])
	if err != nil {
		return errors.New("invalid time value")
	}
	draft, err := b.Drafts.Schedule(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	switch field {
	case "schedule_start_hour":
		if value < 0 || value > 23 {
			return errors.New("invalid start hour")
		}
		draft.StartMinutes = value*60 + draft.StartMinutes%60
	case "schedule_start_minute":
		if value < 0 || value > 45 || value%15 != 0 {
			return errors.New("invalid start minute")
		}
		draft.StartMinutes = draft.StartMinutes/60*60 + value
	case "schedule_end_hour":
		if value < 0 || value > 23 {
			return errors.New("invalid end hour")
		}
		draft.EndMinutes = value*60 + draft.EndMinutes%60
	case "schedule_end_minute":
		if value < 0 || value > 45 || value%15 != 0 {
			return errors.New("invalid end minute")
		}
		draft.EndMinutes = draft.EndMinutes/60*60 + value
	default:
		return errors.New("invalid time field")
	}
	b.Drafts.SaveSchedule(token, draft)
	return b.update(i, scheduleTimeContent(draft), scheduleTimeComponents(token, draft))
}

func (b *Bot) continueScheduleTime(i *discordgo.Interaction, token string) error {
	draft, err := b.Drafts.Schedule(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	if draft.StartMinutes == draft.EndMinutes {
		return errors.New("start and end time must differ")
	}
	return b.update(i, publishLeadContent(draft), publishLeadComponents(token, draft))
}

func publishLeadContent(draft team.ScheduleDraft) string {
	return fmt.Sprintf("Choose when voting opens. Current selection: **%d days before the first raid**.", draft.PublishLeadDays)
}

func publishLeadComponents(token string, draft team.ScheduleDraft) []discordgo.MessageComponent {
	leadDays := []int{1, 2, 3, 5, 7, 10, 14}
	options := make([]discordgo.SelectMenuOption, 0, len(leadDays))
	for _, days := range leadDays {
		options = append(options, discordgo.SelectMenuOption{Label: fmt.Sprintf("%d days before", days), Value: strconv.Itoa(days), Default: days == draft.PublishLeadDays})
	}
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{CustomID: "schedule_lead:" + token, Placeholder: "Voting lead time", MinValues: intPtr(1), MaxValues: 1, Options: options},
		}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "schedule_lead_continue:" + token, Label: "Continue", Style: discordgo.PrimaryButton},
			discordgo.Button{CustomID: "schedule_cancel:" + token, Label: "Cancel", Style: discordgo.SecondaryButton},
		}},
	}
}

func validPublishLead(days int) bool {
	for _, allowed := range []int{1, 2, 3, 5, 7, 10, 14} {
		if days == allowed {
			return true
		}
	}
	return false
}

func (b *Bot) pickPublishLead(i *discordgo.Interaction, token string, values []string) error {
	if len(values) != 1 {
		return errors.New("choose one lead time")
	}
	days, err := strconv.Atoi(values[0])
	if err != nil || !validPublishLead(days) {
		return errors.New("invalid publication lead time")
	}
	draft, err := b.Drafts.Schedule(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	draft.PublishLeadDays = days
	b.Drafts.SaveSchedule(token, draft)
	return b.update(i, publishLeadContent(draft), publishLeadComponents(token, draft))
}

func (b *Bot) openScheduleDate(i *discordgo.Interaction, token string) error {
	draft, err := b.Drafts.Schedule(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	return b.update(i, scheduleDateContent(draft), scheduleDateComponents(token, draft))
}

func scheduleDateContent(draft team.ScheduleDraft) string {
	return "Choose the first period date. Current selection: **" + draft.FirstPeriodStart.Format("Mon, 2006-01-02") + "**."
}

func scheduleDateComponents(token string, draft team.ScheduleDraft) []discordgo.MessageComponent {
	options := make([]discordgo.SelectMenuOption, 0, 25)
	for day := 0; day < 25; day++ {
		date := draft.DatePageStart.AddDate(0, 0, day)
		options = append(options, discordgo.SelectMenuOption{
			Label: date.Format("Mon, Jan 2, 2006"), Value: date.Format("2006-01-02"), Default: sameDate(date, draft.FirstPeriodStart),
		})
	}
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{CustomID: "schedule_date:" + token, Placeholder: "First period date", MinValues: intPtr(1), MaxValues: 1, Options: options},
		}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "schedule_date_page:" + token + ":-1", Label: "Previous 25", Style: discordgo.SecondaryButton, Disabled: !draft.DatePageStart.After(localDateUnchecked(draft.Timezone, now()))},
			discordgo.Button{CustomID: "schedule_date_page:" + token + ":1", Label: "Next 25", Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: "schedule_date_use:" + token, Label: "Use date", Style: discordgo.PrimaryButton},
			discordgo.Button{CustomID: "schedule_cancel:" + token, Label: "Cancel", Style: discordgo.SecondaryButton},
		}},
	}
}

func localDateUnchecked(timezone string, at time.Time) time.Time {
	date, _ := localDate(timezone, at)
	return date
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func (b *Bot) pickScheduleDate(i *discordgo.Interaction, token string, values []string) error {
	if len(values) != 1 {
		return errors.New("choose one period date")
	}
	draft, err := b.Drafts.Schedule(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(draft.Timezone)
	if err != nil {
		return err
	}
	date, err := time.ParseInLocation("2006-01-02", values[0], loc)
	if err != nil || values[0] != date.Format("2006-01-02") {
		return errors.New("invalid period date")
	}
	today, err := localDate(draft.Timezone, now())
	if err != nil {
		return err
	}
	if date.Before(today) {
		return errors.New("period date cannot be in the past")
	}
	draft.FirstPeriodStart = date
	b.Drafts.SaveSchedule(token, draft)
	return b.update(i, scheduleDateContent(draft), scheduleDateComponents(token, draft))
}

func (b *Bot) moveScheduleDatePage(i *discordgo.Interaction, token string, direction int) error {
	if direction != -1 && direction != 1 {
		return errors.New("invalid date page")
	}
	draft, err := b.Drafts.Schedule(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	today, err := localDate(draft.Timezone, now())
	if err != nil {
		return err
	}
	draft.DatePageStart = draft.DatePageStart.AddDate(0, 0, direction*25)
	if draft.DatePageStart.Before(today) {
		draft.DatePageStart = today
	}
	pageEnd := draft.DatePageStart.AddDate(0, 0, 25)
	if draft.FirstPeriodStart.Before(draft.DatePageStart) || !draft.FirstPeriodStart.Before(pageEnd) {
		draft.FirstPeriodStart = draft.DatePageStart
	}
	b.Drafts.SaveSchedule(token, draft)
	return b.update(i, scheduleDateContent(draft), scheduleDateComponents(token, draft))
}

func (b *Bot) reviewSchedule(i *discordgo.Interaction, token string) error {
	draft, err := b.Drafts.Schedule(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	if draft.StartMinutes == draft.EndMinutes {
		return errors.New("start and end time must differ")
	}
	if !validPublishLead(draft.PublishLeadDays) {
		return errors.New("invalid publication lead time")
	}
	occurrences, err := team.GenerateOccurrences(team.Schedule{Timezone: draft.Timezone, CadenceDays: draft.CadenceDays, StartMinutes: draft.StartMinutes, EndMinutes: draft.EndMinutes, Weekdays: draft.Weekdays, NextPeriodStart: draft.FirstPeriodStart})
	if err != nil {
		return err
	}
	if len(occurrences) == 0 {
		return errors.New("schedule produced no occurrences")
	}
	draft.FirstPublishAt, err = team.FirstPublication(occurrences[0].StartsAt, draft.Timezone, draft.PublishLeadDays)
	if err != nil {
		return err
	}
	b.Drafts.SaveSchedule(token, draft)
	var lines []string
	for _, o := range occurrences {
		lines = append(lines, "• <t:"+strconv.FormatInt(o.StartsAt.Unix(), 10)+":F>–<t:"+strconv.FormatInt(o.EndsAt.Unix(), 10)+":t>")
	}
	publishText := "<t:" + strconv.FormatInt(draft.FirstPublishAt.Unix(), 10) + ":F>"
	if !draft.FirstPublishAt.After(now()) {
		publishText += " — **immediately after activation**"
	}
	content := fmt.Sprintf("**%s** — %d-day timetable\nTimezone: `%s`\nRaid time: **%s–%s**\nFirst period: **%s**\nVoting opens: **%d days before the first raid**\nChannel: <#%s>\nPublishes: %s\n%s",
		draft.TeamName, draft.CadenceDays, draft.Timezone, team.FormatClock(draft.StartMinutes), team.FormatClock(draft.EndMinutes),
		draft.FirstPeriodStart.Format("2006-01-02"), draft.PublishLeadDays, draft.ChannelID, publishText, strings.Join(lines, "\n"))
	components := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: "schedule_commit:" + token, Label: "Activate", Style: discordgo.SuccessButton},
		discordgo.Button{CustomID: "schedule_date_back:" + token, Label: "Change date", Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: "schedule_cancel:" + token, Label: "Cancel", Style: discordgo.SecondaryButton},
	}}}
	return b.update(i, content, components)
}

func (b *Bot) commitSchedule(ctx context.Context, i *discordgo.Interaction, token string) error {
	draft, err := b.Drafts.Schedule(token, i.GuildID, userID(i))
	if err != nil {
		return err
	}
	channel, err := b.Session.State.Channel(draft.ChannelID)
	if err != nil {
		channel, err = b.Session.Channel(draft.ChannelID)
	}
	if err != nil {
		return err
	}
	draft.ChannelName = channel.Name
	if err := b.Store.SaveSchedule(ctx, draft); err != nil {
		return err
	}
	b.Drafts.DeleteSchedule(token)
	return b.update(i, "Recurring timetable activated for **"+draft.TeamName+"**.", nil)
}

func (b *Bot) openScheduleManage(ctx context.Context, i *discordgo.Interaction) error {
	teams, err := b.Store.TeamsLedBy(ctx, i.GuildID, userID(i))
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

func (b *Bot) pickScheduleManage(ctx context.Context, i *discordgo.Interaction, values []string) error {
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

func (b *Bot) publishNow(ctx context.Context, i *discordgo.Interaction, teamID int64) error {
	if err := invalidID(teamID); err != nil {
		return err
	}
	value, err := b.ownsTeam(ctx, i.GuildID, userID(i), teamID)
	if err != nil {
		return err
	}
	if err := b.PublishTeam(ctx, teamID, userID(i)); err != nil {
		return err
	}
	return b.update(i, "Published the next timetable for **"+value.Name+"**.", nil)
}

func (b *Bot) PublishTeam(ctx context.Context, teamID int64, leaderID string) error {
	value, err := b.Store.Team(ctx, teamID)
	if err != nil {
		return err
	}
	if value.LeaderID != leaderID {
		return postgres.ErrForbidden
	}
	poll, created, err := b.Store.PublishNext(ctx, teamID, true, now())
	if err != nil {
		return err
	}
	if created {
		b.Log.Info("poll cycle advanced manually", "poll_id", poll.ID, "team_id", teamID)
	}
	return b.publishUnpublished(ctx)
}

func (b *Bot) enableSchedule(ctx context.Context, i *discordgo.Interaction, teamID int64, enabled bool) error {
	if err := invalidID(teamID); err != nil {
		return err
	}
	value, err := b.ownsTeam(ctx, i.GuildID, userID(i), teamID)
	if err != nil {
		return err
	}
	if err := b.SetTeamScheduleEnabled(ctx, teamID, userID(i), enabled); err != nil {
		return err
	}
	state := "paused"
	if enabled {
		state = "enabled"
	}
	return b.update(i, "Automation "+state+" for **"+value.Name+"**.", nil)
}

func (b *Bot) SetTeamScheduleEnabled(ctx context.Context, teamID int64, leaderID string, enabled bool) error {
	return b.Store.SetScheduleEnabled(ctx, teamID, leaderID, enabled)
}

func (b *Bot) republishLatest(ctx context.Context, i *discordgo.Interaction, teamID int64) error {
	if err := invalidID(teamID); err != nil {
		return err
	}
	if _, err := b.ownsTeam(ctx, i.GuildID, userID(i), teamID); err != nil {
		return err
	}
	if err := b.RepublishTeam(ctx, teamID, userID(i)); err != nil {
		return err
	}
	return b.update(i, "Latest timetable republished.", nil)
}

func (b *Bot) RepublishTeam(ctx context.Context, teamID int64, leaderID string) error {
	value, err := b.Store.Team(ctx, teamID)
	if err != nil {
		return err
	}
	if value.LeaderID != leaderID {
		return postgres.ErrForbidden
	}
	view, err := b.Store.LatestPoll(ctx, teamID)
	if err != nil {
		return err
	}
	if view.Poll.MessageID != "" {
		_ = b.Session.ChannelMessageDelete(view.Poll.ChannelID, view.Poll.MessageID)
	}
	if err := b.Store.ClearPollMessage(ctx, view.Poll.ID, leaderID); err != nil {
		return err
	}
	return b.publishUnpublished(ctx)
}
