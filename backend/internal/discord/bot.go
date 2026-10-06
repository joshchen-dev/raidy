package discord

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/joshchen-dev/raidy/internal/postgres"
	"github.com/joshchen-dev/raidy/internal/team"
)

type Bot struct {
	Session *discordgo.Session
	Store   *postgres.Store
	Drafts  *team.Drafts
	Log     *slog.Logger
	// ponytail: process-local publication lock matches the one-replica MVP; use DB leasing before adding replicas.
	publishMu sync.Mutex
}

func New(session *discordgo.Session, store *postgres.Store, log *slog.Logger) *Bot {
	return &Bot{Session: session, Store: store, Drafts: team.NewDrafts(), Log: log}
}

func Commands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name: "team", Description: "Set up or manage a static team",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "setup", Description: "Create a team interactively"},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "manage", Description: "Manage a team roster or delete a team"},
			},
		},
		{
			Name: "schedule", Description: "Set up or manage raid timetables",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "setup", Description: "Configure a recurring timetable"},
				{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "manage", Description: "Publish, pause, refresh, or replace a timetable"},
			},
		},
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
	case discordgo.InteractionModalSubmit:
		err = b.handleModal(ctx, event.Interaction)
	case discordgo.InteractionMessageComponent:
		err = b.handleComponent(ctx, event.Interaction)
	}
	if err != nil {
		b.Log.Error("interaction failed", "type", event.Type.String(), "user_id", userID(event.Interaction), "error", err)
		b.respondError(event.Interaction, err)
	}
}

func (b *Bot) handleCommand(ctx context.Context, i *discordgo.Interaction) error {
	data := i.ApplicationCommandData()
	if len(data.Options) != 1 {
		return errors.New("choose a subcommand")
	}
	subcommand := data.Options[0].Name
	switch data.Name + ":" + subcommand {
	case "team:setup":
		return b.openTeamSetup(i)
	case "team:manage":
		return b.openTeamManage(ctx, i)
	case "schedule:setup":
		return b.openScheduleSetup(ctx, i)
	case "schedule:manage":
		return b.openScheduleManage(ctx, i)
	default:
		return errors.New("unknown command")
	}
}

func (b *Bot) handleModal(ctx context.Context, i *discordgo.Interaction) error {
	data := i.ModalSubmitData()
	switch {
	case data.CustomID == "team_setup":
		return b.submitTeamSetup(i, modalValues(data))
	case strings.HasPrefix(data.CustomID, "team_timezone_custom:"):
		return b.submitCustomTimezone(i, strings.TrimPrefix(data.CustomID, "team_timezone_custom:"), modalValues(data))
	default:
		return errors.New("unknown or expired modal")
	}
}

func (b *Bot) handleComponent(ctx context.Context, i *discordgo.Interaction) error {
	data := i.MessageComponentData()
	parts := strings.Split(data.CustomID, ":")
	if len(parts) == 0 {
		return errors.New("invalid interaction")
	}
	switch parts[0] {
	case "team_timezone":
		return b.selectTeamTimezone(i, part(parts, 1), data.Values)
	case "team_timezone_default":
		return b.saveTeamTimezone(i, part(parts, 1), "Asia/Tokyo", true)
	case "team_roster":
		return b.selectTeamRoster(i, part(parts, 1), data.Values)
	case "team_roster_empty":
		return b.selectTeamRoster(i, part(parts, 1), nil)
	case "team_commit":
		return b.commitTeam(ctx, i, part(parts, 1))
	case "team_cancel":
		b.Drafts.DeleteTeam(part(parts, 1))
		return b.update(i, "Setup cancelled.", nil)
	case "team_manage_pick":
		return b.pickManagedTeam(ctx, i, data.Values)
	case "team_delete":
		return b.confirmDeleteTeam(ctx, i, partInt64(parts, 1))
	case "team_delete_confirm":
		return b.deleteTeam(ctx, i, partInt64(parts, 1))
	case "schedule_team":
		return b.pickScheduleTeam(ctx, i, part(parts, 1), data.Values)
	case "schedule_cadence":
		return b.pickCadence(i, part(parts, 1), partInt(parts, 2))
	case "schedule_weekdays":
		return b.pickWeekdays(i, part(parts, 1), data.Values)
	case "schedule_start_hour", "schedule_start_minute", "schedule_end_hour", "schedule_end_minute":
		return b.pickScheduleTime(i, part(parts, 1), parts[0], data.Values)
	case "schedule_time_continue":
		return b.continueScheduleTime(i, part(parts, 1))
	case "schedule_lead":
		return b.pickPublishLead(i, part(parts, 1), data.Values)
	case "schedule_lead_continue":
		return b.openScheduleDate(i, part(parts, 1))
	case "schedule_date":
		return b.pickScheduleDate(i, part(parts, 1), data.Values)
	case "schedule_date_page":
		return b.moveScheduleDatePage(i, part(parts, 1), partInt(parts, 2))
	case "schedule_date_use":
		return b.reviewSchedule(i, part(parts, 1))
	case "schedule_date_back":
		return b.openScheduleDate(i, part(parts, 1))
	case "schedule_commit":
		return b.commitSchedule(ctx, i, part(parts, 1))
	case "schedule_cancel":
		b.Drafts.DeleteSchedule(part(parts, 1))
		return b.update(i, "Setup cancelled.", nil)
	case "schedule_manage_pick":
		return b.pickScheduleManage(ctx, i, data.Values)
	case "schedule_publish":
		return b.publishNow(ctx, i, partInt64(parts, 1))
	case "schedule_enable":
		return b.enableSchedule(ctx, i, partInt64(parts, 1), true)
	case "schedule_disable":
		return b.enableSchedule(ctx, i, partInt64(parts, 1), false)
	case "schedule_replace":
		return b.beginScheduleForTeam(ctx, i, partInt64(parts, 1))
	case "schedule_republish":
		return b.republishLatest(ctx, i, partInt64(parts, 1))
	case "availability":
		return b.openAvailability(ctx, i, partInt64(parts, 1))
	case "availability_set":
		return b.saveAvailability(ctx, i, partInt64(parts, 1), data.Values)
	case "availability_none":
		return b.saveAvailability(ctx, i, partInt64(parts, 1), nil)
	case "poll_manage":
		return b.openPollManage(ctx, i, partInt64(parts, 1))
	case "occurrence_select":
		return b.selectOccurrence(i, partInt64s(data.Values))
	case "occurrence_action":
		return b.changeOccurrence(ctx, i, partInt64(parts, 1), part(parts, 2))
	default:
		return errors.New("unknown or expired interaction")
	}
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

func (b *Bot) update(i *discordgo.Interaction, content string, components []discordgo.MessageComponent) error {
	return b.Session.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{Content: content, Components: components},
	})
}

func (b *Bot) modal(i *discordgo.Interaction, customID, title string, inputs ...discordgo.TextInput) error {
	rows := make([]discordgo.MessageComponent, 0, len(inputs))
	for _, input := range inputs {
		rows = append(rows, discordgo.ActionsRow{Components: []discordgo.MessageComponent{input}})
	}
	return b.Session.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{CustomID: customID, Title: title, Components: rows},
	})
}

func (b *Bot) respondError(i *discordgo.Interaction, err error) {
	if responseErr := b.ephemeral(i, userMessage(err), nil); responseErr != nil {
		b.Log.Error("failed to send interaction error", "error", responseErr)
	}
}

func modalValues(data discordgo.ModalSubmitInteractionData) map[string]string {
	values := make(map[string]string)
	for _, component := range data.Components {
		var children []discordgo.MessageComponent
		switch row := component.(type) {
		case discordgo.ActionsRow:
			children = row.Components
		case *discordgo.ActionsRow:
			children = row.Components
		}
		for _, child := range children {
			switch input := child.(type) {
			case discordgo.TextInput:
				values[input.CustomID] = input.Value
			case *discordgo.TextInput:
				values[input.CustomID] = input.Value
			}
		}
	}
	return values
}

func part(parts []string, index int) string {
	if index >= len(parts) {
		return ""
	}
	return parts[index]
}

func partInt(parts []string, index int) int {
	value, _ := strconv.Atoi(part(parts, index))
	return value
}

func partInt64(parts []string, index int) int64 {
	value, _ := strconv.ParseInt(part(parts, index), 10, 64)
	return value
}

func partInt64s(values []string) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if id, err := strconv.ParseInt(value, 10, 64); err == nil {
			result = append(result, id)
		}
	}
	return result
}

func intPtr(value int) *int { return &value }

func teamOptions(teams []team.Team) []discordgo.SelectMenuOption {
	options := make([]discordgo.SelectMenuOption, 0, len(teams))
	for _, value := range teams {
		options = append(options, discordgo.SelectMenuOption{Label: value.Name, Value: strconv.FormatInt(value.ID, 10)})
	}
	return options
}

func mentionList(ids []string) string {
	if len(ids) == 0 {
		return "None"
	}
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = "<@" + id + ">"
	}
	return strings.Join(values, ", ")
}

func (b *Bot) ownsTeam(ctx context.Context, guildID, leaderID string, teamID int64) (team.Team, error) {
	teams, err := b.Store.TeamsLedBy(ctx, guildID, leaderID)
	if err != nil {
		return team.Team{}, err
	}
	for _, value := range teams {
		if value.ID == teamID {
			return value, nil
		}
	}
	return team.Team{}, postgres.ErrForbidden
}

func invalidID(id int64) error {
	if id <= 0 {
		return fmt.Errorf("invalid identifier")
	}
	return nil
}

func now() time.Time { return time.Now().UTC() }
