package discord

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
		embeds, components := renderAnnouncement(view, true, b.BaseURL)
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
	embeds, components := renderAnnouncement(view, active, b.BaseURL)
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

func renderAnnouncement(view team.PollView, active bool, baseURL string) ([]*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	fields := make([]*discordgo.MessageEmbedField, 0, len(view.Poll.Occurrences))
	for _, occurrence := range view.Poll.Occurrences {
		available := 0
		for _, memberID := range view.Members {
			if view.Availability[occurrence.ID][memberID] {
				available++
			}
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  fmt.Sprintf("%s <t:%d:F>", statusIcon(occurrence.Status), occurrence.StartsAt.Unix()),
			Value: fmt.Sprintf("**%d/%d available** · %s", available, len(view.Members), statusLabel(occurrence.Status)),
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
	embed.Footer = &discordgo.MessageEmbedFooter{Text: "Times are shown in your timezone. Vote and manage dates in Raidy."}
	components := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{Label: "Open in Raidy", Style: discordgo.LinkButton, URL: appURL(baseURL, view.Poll.TeamID)},
	}}}
	return []*discordgo.MessageEmbed{embed}, components
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

// PublishTeam publishes the team's next timetable now instead of waiting for
// its scheduled publication time.
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

func (b *Bot) SetTeamScheduleEnabled(ctx context.Context, teamID int64, leaderID string, enabled bool) error {
	return b.Store.SetScheduleEnabled(ctx, teamID, leaderID, enabled)
}

// RepublishTeam replaces the latest timetable message, for example after the
// original was deleted or the channel changed.
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
