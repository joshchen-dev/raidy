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

func (b *Bot) publishUnpublished(ctx context.Context) error {
	b.publishMu.Lock()
	defer b.publishMu.Unlock()
	polls, err := b.Store.UnpublishedPolls(ctx)
	if err != nil {
		return err
	}
	var publicationErr error
	for _, poll := range polls {
		view, err := b.Store.PollView(ctx, poll.ID)
		if err != nil {
			b.Log.Error("poll load failed", "poll_id", poll.ID, "team_id", poll.TeamID, "error", err)
			publicationErr = errors.Join(publicationErr, err)
			continue
		}
		embeds, components := renderPoll(view, true)
		message, err := b.Session.ChannelMessageSendComplex(poll.ChannelID, &discordgo.MessageSend{
			Embeds:          embeds,
			Components:      components,
			AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
		})
		if err != nil {
			b.Log.Error("poll publication failed", "poll_id", poll.ID, "team_id", poll.TeamID, "channel_id", poll.ChannelID, "error", err)
			publicationErr = errors.Join(publicationErr, err)
			continue
		}
		if err := b.Store.SetPollMessage(ctx, poll.ID, message.ID); err != nil {
			b.Log.Error("poll publication persistence failed", "poll_id", poll.ID, "team_id", poll.TeamID, "message_id", message.ID, "error", err)
			publicationErr = errors.Join(publicationErr, err)
			continue
		}
		b.Log.Info("poll published", "poll_id", poll.ID, "team_id", poll.TeamID, "message_id", message.ID)
	}
	return publicationErr
}

func (b *Bot) editPoll(ctx context.Context, pollID int64, active bool) error {
	view, err := b.Store.PollView(ctx, pollID)
	if err != nil {
		return err
	}
	if view.Poll.MessageID == "" {
		return nil
	}
	embeds, components := renderPoll(view, active)
	_, err = b.Session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:              view.Poll.MessageID,
		Channel:         view.Poll.ChannelID,
		Embeds:          &embeds,
		Components:      &components,
		AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
	})
	return err
}

func (b *Bot) RefreshTeamPoll(ctx context.Context, teamID int64) error {
	view, err := b.Store.LatestPoll(ctx, teamID)
	if errors.Is(err, postgres.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return b.editPoll(ctx, view.Poll.ID, true)
}

func renderPoll(view team.PollView, active bool) ([]*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	fields := make([]*discordgo.MessageEmbedField, 0, len(view.Poll.Occurrences))
	for _, occurrence := range view.Poll.Occurrences {
		available, unavailable := 0, 0
		var availableMembers, unavailableMembers, pendingMembers []string
		for _, memberID := range view.Members {
			choice, submitted := view.Availability[occurrence.ID][memberID]
			if !submitted {
				pendingMembers = append(pendingMembers, memberID)
				continue
			}
			if choice {
				available++
				availableMembers = append(availableMembers, memberID)
			} else {
				unavailable++
				unavailableMembers = append(unavailableMembers, memberID)
			}
		}
		pending := len(view.Members) - available - unavailable
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  fmt.Sprintf("%s <t:%d:F>", statusIcon(occurrence.Status), occurrence.StartsAt.Unix()),
			Value: fmt.Sprintf("Available **%d** · Unavailable **%d** · Pending **%d**\nStatus: %s\nAvailable: %s\nUnavailable: %s\nPending: %s", available, unavailable, pending, statusLabel(occurrence.Status), memberMentions(availableMembers), memberMentions(unavailableMembers), memberMentions(pendingMembers)),
		})
	}
	embed := &discordgo.MessageEmbed{
		Title:       view.Poll.TeamName + " raid timetable",
		Description: fmt.Sprintf("%s – %s · `%s`", view.Poll.PeriodStart.Format("2006-01-02"), view.Poll.PeriodEnd.AddDate(0, 0, -1).Format("2006-01-02"), view.Poll.Timezone),
		Fields:      fields,
	}
	if !active {
		embed.Footer = &discordgo.MessageEmbedFooter{Text: "This timetable is closed and kept as history."}
		return []*discordgo.MessageEmbed{embed}, nil
	}
	id := strconv.FormatInt(view.Poll.ID, 10)
	components := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: "availability:" + id, Label: "Set availability", Style: discordgo.PrimaryButton},
		discordgo.Button{CustomID: "poll_manage:" + id, Label: "Manage dates", Style: discordgo.SecondaryButton},
	}}}
	return []*discordgo.MessageEmbed{embed}, components
}

func memberMentions(ids []string) string {
	if len(ids) == 0 {
		return "—"
	}
	mentions := make([]string, len(ids))
	for index, id := range ids {
		mentions[index] = "<@" + id + ">"
	}
	return strings.Join(mentions, ", ")
}

func statusIcon(status string) string {
	return map[string]string{
		"proposed":           "🗓️",
		"confirmed":          "✅",
		"cancelled":          "❌",
		"attention_required": "⚠️",
	}[status]
}

func statusLabel(status string) string {
	if status == "attention_required" {
		return "Attention required"
	}
	if status == "" {
		return "Unknown"
	}
	return strings.ToUpper(status[:1]) + status[1:]
}

func (b *Bot) openAvailability(ctx context.Context, i *discordgo.Interaction, pollID int64) error {
	if err := invalidID(pollID); err != nil {
		return err
	}
	view, err := b.Store.PollView(ctx, pollID)
	if err != nil {
		return err
	}
	memberID := userID(i)
	if !contains(view.Members, memberID) {
		return postgres.ErrForbidden
	}
	if view.Poll.Expired(now()) {
		return postgres.ErrExpired
	}
	options := make([]discordgo.SelectMenuOption, 0, len(view.Poll.Occurrences))
	loc := mustLocation(view.Poll.Timezone)
	for _, occurrence := range view.Poll.Occurrences {
		if !occurrence.StartsAt.After(now()) {
			continue
		}
		options = append(options, discordgo.SelectMenuOption{
			Label:       occurrence.StartsAt.In(loc).Format("Mon, Jan 2 15:04"),
			Value:       strconv.FormatInt(occurrence.ID, 10),
			Default:     view.Availability[occurrence.ID][memberID],
			Description: statusLabel(occurrence.Status),
		})
	}
	if len(options) == 0 {
		return postgres.ErrExpired
	}
	id := strconv.FormatInt(pollID, 10)
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{CustomID: "availability_set:" + id, Placeholder: "Select every date you can attend", MinValues: intPtr(0), MaxValues: len(options), Options: options},
		}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "availability_none:" + id, Label: "Unavailable for all", Style: discordgo.DangerButton},
		}},
	}
	return b.ephemeral(i, "Selected dates become Available; every omitted date becomes Unavailable.", components)
}

func (b *Bot) saveAvailability(ctx context.Context, i *discordgo.Interaction, pollID int64, values []string) error {
	if err := invalidID(pollID); err != nil {
		return err
	}
	ids := partInt64s(values)
	if len(ids) != len(values) {
		return errors.New("invalid occurrence")
	}
	if err := b.Store.SetAvailability(ctx, pollID, userID(i), ids, now()); err != nil {
		return err
	}
	if err := b.update(i, "Availability saved.", nil); err != nil {
		return err
	}
	b.refresher.queue(pollID)
	return nil
}

// SubmitAvailability records a member's answers made outside Discord: listed
// dates become Available and every other upcoming date Unavailable. The
// Discord announcement is refreshed in the background.
func (b *Bot) SubmitAvailability(ctx context.Context, pollID int64, memberID string, available []int64) error {
	if err := b.Store.SetAvailability(ctx, pollID, memberID, available, now()); err != nil {
		return err
	}
	b.refresher.queue(pollID)
	return nil
}

func (b *Bot) openPollManage(ctx context.Context, i *discordgo.Interaction, pollID int64) error {
	if err := invalidID(pollID); err != nil {
		return err
	}
	view, err := b.Store.PollView(ctx, pollID)
	if err != nil {
		return err
	}
	if view.Poll.LeaderID != userID(i) {
		return postgres.ErrForbidden
	}
	if view.Poll.Expired(now()) {
		return postgres.ErrExpired
	}
	options := make([]discordgo.SelectMenuOption, 0, len(view.Poll.Occurrences))
	loc := mustLocation(view.Poll.Timezone)
	for _, occurrence := range view.Poll.Occurrences {
		if occurrence.StartsAt.After(now()) {
			options = append(options, discordgo.SelectMenuOption{
				Label:       occurrence.StartsAt.In(loc).Format("Mon, Jan 2 15:04"),
				Value:       strconv.FormatInt(occurrence.ID, 10),
				Description: statusLabel(occurrence.Status),
			})
		}
	}
	if len(options) == 0 {
		return postgres.ErrExpired
	}
	components := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.SelectMenu{CustomID: "occurrence_select", Placeholder: "Choose a raid date", MinValues: intPtr(1), MaxValues: 1, Options: options},
	}}}
	return b.ephemeral(i, "Choose the date to manage.", components)
}

func (b *Bot) selectOccurrence(i *discordgo.Interaction, ids []int64) error {
	if len(ids) != 1 {
		return errors.New("choose one occurrence")
	}
	id := strconv.FormatInt(ids[0], 10)
	components := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: "occurrence_action:" + id + ":confirm", Label: "Confirm", Style: discordgo.SuccessButton},
		discordgo.Button{CustomID: "occurrence_action:" + id + ":cancel", Label: "Cancel", Style: discordgo.DangerButton},
		discordgo.Button{CustomID: "occurrence_action:" + id + ":reopen", Label: "Reopen", Style: discordgo.SecondaryButton},
	}}}
	return b.update(i, "Choose the new state for this raid date.", components)
}

func (b *Bot) changeOccurrence(ctx context.Context, i *discordgo.Interaction, occurrenceID int64, action string) error {
	if err := invalidID(occurrenceID); err != nil {
		return err
	}
	if err := b.Store.SetOccurrenceStatus(ctx, occurrenceID, userID(i), action, now()); err != nil {
		return err
	}
	pollID, err := b.Store.PollIDForOccurrence(ctx, occurrenceID)
	if err != nil {
		return err
	}
	if err := b.update(i, "Raid date updated.", nil); err != nil {
		return err
	}
	b.refresher.queue(pollID)
	return nil
}

// ChangeOccurrence applies a leader's confirm, cancel, or reopen decision made
// outside Discord and refreshes the published timetable message.
func (b *Bot) ChangeOccurrence(ctx context.Context, occurrenceID int64, leaderID, action string) error {
	if err := b.Store.SetOccurrenceStatus(ctx, occurrenceID, leaderID, action, now()); err != nil {
		return err
	}
	pollID, err := b.Store.PollIDForOccurrence(ctx, occurrenceID)
	if err != nil {
		return err
	}
	b.refresher.queue(pollID)
	return nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func mustLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return location
}
