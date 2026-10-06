package discord

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/joshchen-dev/raidy/internal/postgres"
	"github.com/joshchen-dev/raidy/internal/team"
)

// Bot announces timetables in Discord. Every decision (rosters, schedules,
// availability, and date status) is made on the web; Discord only shows the
// result and links back to it.
type Bot struct {
	Session *discordgo.Session
	Store   *postgres.Store
	Log     *slog.Logger
	// BaseURL is the public web origin that announcements and /raidy link to.
	BaseURL string
	// ponytail: process-local publication lock matches the one-replica MVP; use DB leasing before adding replicas.
	publishMu sync.Mutex
	refresher *pollRefresher
}

func New(session *discordgo.Session, store *postgres.Store, log *slog.Logger, baseURL string) *Bot {
	b := &Bot{Session: session, Store: store, Log: log, BaseURL: baseURL}
	b.refresher = newPollRefresher(refreshDelay, log, func(ctx context.Context, pollID int64) error {
		return b.editPoll(ctx, pollID, true)
	})
	return b
}

func Commands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{Name: "raidy", Description: "Open your team's Raidy page"},
	}
}

// interactionTimeout bounds the database and Discord work done for one
// interaction so a hung dependency cannot pin the handler goroutine forever.
const interactionTimeout = 10 * time.Second

func (b *Bot) Handle(_ *discordgo.Session, event *discordgo.InteractionCreate) {
	if event.GuildID == "" {
		b.respondError(event.Interaction, errors.New("Raidy commands are only available inside a server"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), interactionTimeout)
	defer cancel()
	var err error
	switch event.Type {
	case discordgo.InteractionApplicationCommand:
		err = b.handleCommand(ctx, event.Interaction)
	case discordgo.InteractionMessageComponent, discordgo.InteractionModalSubmit:
		// Buttons on timetables posted before the move to the web still exist.
		err = b.ephemeral(event.Interaction, "Raidy now runs on the web: "+appURL(b.BaseURL, 0), nil)
	}
	if err != nil {
		b.Log.Error("interaction failed", "type", event.Type.String(), "user_id", userID(event.Interaction), "error", err)
		b.respondError(event.Interaction, err)
	}
}

func (b *Bot) handleCommand(ctx context.Context, i *discordgo.Interaction) error {
	if i.ApplicationCommandData().Name != "raidy" {
		return errors.New("unknown command")
	}
	teams, err := b.Store.TeamsForUser(ctx, userID(i))
	if err != nil {
		return err
	}
	return b.ephemeral(i, raidyReply(b.BaseURL, i.GuildID, teams), nil)
}

// raidyReply links to the member's team in this server, or to the app when
// they have none here yet.
func raidyReply(baseURL, guildID string, teams []team.Team) string {
	var here []team.Team
	for _, value := range teams {
		if value.GuildID == guildID {
			here = append(here, value)
		}
	}
	if len(here) == 0 {
		return "You are not on a team in this server yet. Sign in to create one: " + appURL(baseURL, 0)
	}
	lines := make([]string, 0, len(here))
	for _, value := range here {
		lines = append(lines, "**"+value.Name+"**: "+appURL(baseURL, value.ID))
	}
	return strings.Join(lines, "\n")
}

func appURL(baseURL string, teamID int64) string {
	value := strings.TrimRight(baseURL, "/") + "/app"
	if teamID > 0 {
		value += "?team=" + strconv.FormatInt(teamID, 10)
	}
	return value
}

func userID(i *discordgo.Interaction) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func (b *Bot) ephemeral(i *discordgo.Interaction, content string, components []discordgo.MessageComponent) error {
	return b.Session.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content, Components: components, Flags: discordgo.MessageFlagsEphemeral},
	})
}

func (b *Bot) respondError(i *discordgo.Interaction, err error) {
	if responseErr := b.ephemeral(i, userMessage(err), nil); responseErr != nil {
		b.Log.Error("failed to send interaction error", "error", responseErr)
	}
}

func now() time.Time { return time.Now().UTC() }
