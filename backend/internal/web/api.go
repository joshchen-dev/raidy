package web

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/joshchen-dev/raidy/internal/postgres"
	"github.com/joshchen-dev/raidy/internal/team"
)

type memberResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
}

type teamResponse struct {
	ID        int64            `json:"id"`
	GuildID   string           `json:"guildId"`
	GuildName string           `json:"guildName"`
	Name      string           `json:"name"`
	Timezone  string           `json:"timezone"`
	LeaderID  string           `json:"leaderId"`
	IsLeader  bool             `json:"isLeader"`
	Members   []memberResponse `json:"members"`
}

type teamInput struct {
	GuildID   string   `json:"guildId"`
	Name      string   `json:"name"`
	Timezone  string   `json:"timezone"`
	MemberIDs []string `json:"memberIds"`
}

func (h *Handler) teams(w http.ResponseWriter, r *http.Request) {
	current := sessionFrom(r.Context())
	values, err := h.store.TeamsForUser(r.Context(), current.User.ID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), discordTimeout)
	defer cancel()
	result := make([]teamResponse, 0, len(values))
	for _, value := range values {
		if current.hasGuild(value.GuildID) {
			if name := current.guildName(value.GuildID); name != "" && name != value.GuildName {
				if err := h.store.UpdateGuildName(r.Context(), value.ID, name); err != nil {
					h.log.Warn("guild name refresh failed", "team_id", value.ID, "error", err)
				} else {
					value.GuildName = name
				}
			}
			result = append(result, h.teamResponse(ctx, value, current.User.ID))
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createTeam(w http.ResponseWriter, r *http.Request) {
	var input teamInput
	if err := decode(r, &input); err != nil {
		h.fail(w, r, clientError{err.Error()})
		return
	}
	current := sessionFrom(r.Context())
	if !current.hasGuild(input.GuildID) {
		h.fail(w, r, postgres.ErrForbidden)
		return
	}
	if err := validateTeamInput(input.Name, input.Timezone); err != nil {
		h.fail(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), discordTimeout)
	defer cancel()
	if err := h.validateMembers(ctx, input.GuildID, current.User.ID, input.MemberIDs); err != nil {
		h.fail(w, r, err)
		return
	}
	created, err := h.store.CreateTeam(r.Context(), input.GuildID, current.guildName(input.GuildID), current.User.ID, input.Name, input.Timezone, input.MemberIDs)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, h.teamResponse(ctx, created, current.User.ID))
}

func (h *Handler) updateTeam(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name      string   `json:"name"`
		MemberIDs []string `json:"memberIds"`
	}
	if err := decode(r, &input); err != nil {
		h.fail(w, r, clientError{err.Error()})
		return
	}
	current := sessionFrom(r.Context())
	value, err := h.leaderTeam(r.Context(), current, r.PathValue("teamID"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := validateName(input.Name); err != nil {
		h.fail(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), discordTimeout)
	defer cancel()
	if err := h.validateMembers(ctx, value.GuildID, current.User.ID, input.MemberIDs); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.store.UpdateTeam(r.Context(), value.ID, current.User.ID, input.Name, value.Timezone, input.MemberIDs); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.bot.RefreshOpenPolls(r.Context(), value.ID); err != nil {
		h.log.Error("poll refresh after roster update failed", "team_id", value.ID, "error", err)
	}
	updated, err := h.store.Team(r.Context(), value.ID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.teamResponse(ctx, updated, current.User.ID))
}

func (h *Handler) deleteTeam(w http.ResponseWriter, r *http.Request) {
	current := sessionFrom(r.Context())
	value, err := h.leaderTeam(r.Context(), current, r.PathValue("teamID"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.store.DeleteTeam(r.Context(), value.ID, current.User.ID); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validateTeamInput(name, timezone string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return clientError{message: "choose a valid IANA timezone"}
	}
	return nil
}

func validateName(name string) error {
	name = team.DisplayName(name)
	if name == "" || len(name) > 80 {
		return clientError{message: "team name must be between 1 and 80 characters"}
	}
	return nil
}

func (h *Handler) validateMembers(ctx context.Context, guildID, leaderID string, ids []string) error {
	seen := map[string]bool{leaderID: true}
	if len(ids) > 7 {
		return clientError{message: "choose at most seven teammates"}
	}
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		member, err := h.member(ctx, guildID, id)
		if err != nil || member.User == nil || member.User.Bot {
			return clientError{message: "every teammate must be a human member of the Discord server"}
		}
	}
	return nil
}

// discordTimeout bounds every Discord REST lookup made while serving one request.
const discordTimeout = 5 * time.Second

// member resolves a guild member from the gateway cache and only falls back to
// REST for members the bot has not seen yet.
func (h *Handler) member(ctx context.Context, guildID, userID string) (*discordgo.Member, error) {
	if member, err := h.discord.State.Member(guildID, userID); err == nil {
		return member, nil
	}
	member, err := h.discord.GuildMember(guildID, userID, discordgo.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	member.GuildID = guildID
	_ = h.discord.State.MemberAdd(member) // Best effort: caching needs the guild in state.
	return member, nil
}

func (h *Handler) teamResponse(ctx context.Context, value team.Team, userID string) teamResponse {
	members := make([]memberResponse, 0, len(value.MemberIDs))
	for _, id := range value.MemberIDs {
		member, err := h.member(ctx, value.GuildID, id)
		if err != nil || member.User == nil {
			members = append(members, memberResponse{ID: id, Name: id})
			continue
		}
		members = append(members, memberJSON(member))
	}
	return teamResponse{ID: value.ID, GuildID: value.GuildID, GuildName: value.GuildName, Name: value.Name, Timezone: value.Timezone, LeaderID: value.LeaderID, IsLeader: value.LeaderID == userID, Members: members}
}

func memberJSON(member *discordgo.Member) memberResponse {
	name := member.Nick
	if name == "" {
		name = member.User.GlobalName
	}
	if name == "" {
		name = member.User.Username
	}
	return memberResponse{ID: member.User.ID, Name: name, Avatar: member.AvatarURL("64")}
}

func (h *Handler) members(w http.ResponseWriter, r *http.Request) {
	current := sessionFrom(r.Context())
	guildID := r.PathValue("guildID")
	if !current.hasGuild(guildID) {
		h.fail(w, r, postgres.ErrForbidden)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusOK, []memberResponse{})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	values, err := h.discord.GuildMembersSearch(guildID, query, 20, discordgo.WithContext(ctx))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	result := make([]memberResponse, 0, len(values))
	for _, value := range values {
		if value.User != nil && !value.User.Bot {
			result = append(result, memberJSON(value))
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) channels(w http.ResponseWriter, r *http.Request) {
	current := sessionFrom(r.Context())
	guildID := r.PathValue("guildID")
	if !current.hasGuild(guildID) {
		h.fail(w, r, postgres.ErrForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	values, err := h.discord.GuildChannels(guildID, discordgo.WithContext(ctx))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	permissions, err := h.botChannelPermissions(ctx, guildID, values)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	type channel struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	result := make([]channel, 0, len(values))
	for _, value := range values {
		if value.Type != discordgo.ChannelTypeGuildText && value.Type != discordgo.ChannelTypeGuildNews {
			continue
		}
		needed := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)
		if permissions[value.ID]&needed == needed {
			result = append(result, channel{ID: value.ID, Name: value.Name})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	writeJSON(w, http.StatusOK, result)
}

type scheduleInput struct {
	Timezone         string `json:"timezone"`
	CadenceDays      int    `json:"cadenceDays"`
	Weekdays         []int  `json:"weekdays"`
	StartTime        string `json:"startTime"`
	EndTime          string `json:"endTime"`
	PublishLeadDays  int    `json:"publishLeadDays"`
	FirstPeriodStart string `json:"firstPeriodStart"`
	ChannelID        string `json:"channelId"`
}

type occurrenceResponse struct {
	ID          int64          `json:"id,omitempty"`
	StartsAt    time.Time      `json:"startsAt"`
	EndsAt      time.Time      `json:"endsAt"`
	Status      string         `json:"status"`
	Available   int            `json:"available,omitempty"`
	Unavailable int            `json:"unavailable,omitempty"`
	Pending     int            `json:"pending,omitempty"`
	Votes       []voteResponse `json:"votes,omitempty"`
}

type voteResponse struct {
	MemberID string `json:"memberId"`
	Status   string `json:"status"`
}

type scheduleResponse struct {
	Timezone         string               `json:"timezone"`
	CadenceDays      int                  `json:"cadenceDays"`
	Weekdays         []int                `json:"weekdays"`
	StartTime        string               `json:"startTime"`
	EndTime          string               `json:"endTime"`
	PublishLeadDays  int                  `json:"publishLeadDays"`
	FirstPeriodStart string               `json:"firstPeriodStart"`
	NextPublishAt    time.Time            `json:"nextPublishAt"`
	ChannelID        string               `json:"channelId"`
	ChannelName      string               `json:"channelName"`
	Enabled          bool                 `json:"enabled"`
	Occurrences      []occurrenceResponse `json:"occurrences,omitempty"`
}

func (h *Handler) getSchedule(w http.ResponseWriter, r *http.Request) {
	current := sessionFrom(r.Context())
	value, err := h.memberTeam(r.Context(), current, r.PathValue("teamID"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	schedule, err := h.store.Schedule(r.Context(), value.ID)
	if errors.Is(err, postgres.ErrNotFound) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, scheduleJSON(schedule))
}

func scheduleJSON(value team.Schedule) scheduleResponse {
	weekdays := make([]int, len(value.Weekdays))
	for index, day := range value.Weekdays {
		weekdays[index] = int(day)
	}
	return scheduleResponse{
		Timezone: value.Timezone, CadenceDays: value.CadenceDays, Weekdays: weekdays,
		StartTime: team.FormatClock(value.StartMinutes), EndTime: team.FormatClock(value.EndMinutes),
		PublishLeadDays: value.PublishLeadDays, FirstPeriodStart: value.NextPeriodStart.Format("2006-01-02"),
		NextPublishAt: value.NextPublishAt, ChannelID: value.ChannelID, ChannelName: value.ChannelName, Enabled: value.Enabled,
	}
}

func (h *Handler) previewSchedule(w http.ResponseWriter, r *http.Request) {
	current := sessionFrom(r.Context())
	value, err := h.leaderTeam(r.Context(), current, r.PathValue("teamID"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var input scheduleInput
	if err := decode(r, &input); err != nil {
		h.fail(w, r, clientError{err.Error()})
		return
	}
	draft, occurrences, err := h.scheduleDraft(value, current.User.ID, input)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	response := scheduleResponse{Timezone: draft.Timezone, CadenceDays: draft.CadenceDays, StartTime: team.FormatClock(draft.StartMinutes), EndTime: team.FormatClock(draft.EndMinutes), PublishLeadDays: draft.PublishLeadDays, FirstPeriodStart: draft.FirstPeriodStart.Format("2006-01-02"), NextPublishAt: draft.FirstPublishAt, ChannelID: draft.ChannelID, Enabled: true}
	for _, day := range draft.Weekdays {
		response.Weekdays = append(response.Weekdays, int(day))
	}
	for _, occurrence := range occurrences {
		response.Occurrences = append(response.Occurrences, occurrenceResponse{StartsAt: occurrence.StartsAt, EndsAt: occurrence.EndsAt, Status: occurrence.Status})
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) saveSchedule(w http.ResponseWriter, r *http.Request) {
	current := sessionFrom(r.Context())
	value, err := h.leaderTeam(r.Context(), current, r.PathValue("teamID"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var input scheduleInput
	if err := decode(r, &input); err != nil {
		h.fail(w, r, clientError{err.Error()})
		return
	}
	draft, _, err := h.scheduleDraft(value, current.User.ID, input)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	channel, err := h.postChannel(r.Context(), value.GuildID, draft.ChannelID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	draft.ChannelName = channel.Name
	if err := h.store.SaveSchedule(r.Context(), draft, time.Now()); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.bot.RefreshOpenPolls(r.Context(), value.ID); err != nil {
		h.log.Error("poll refresh after schedule update failed", "team_id", value.ID, "error", err)
	}
	saved, err := h.store.Schedule(r.Context(), value.ID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, scheduleJSON(saved))
}

func (h *Handler) scheduleDraft(value team.Team, userID string, input scheduleInput) (team.ScheduleDraft, []team.Occurrence, error) {
	if input.Timezone == "" {
		input.Timezone = value.Timezone
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return team.ScheduleDraft{}, nil, clientError{message: "choose a valid IANA timezone"}
	}
	if input.CadenceDays != 7 && input.CadenceDays != 14 {
		return team.ScheduleDraft{}, nil, clientError{message: "cadence must be weekly or biweekly"}
	}
	weekdayStrings := make([]string, len(input.Weekdays))
	for index, day := range input.Weekdays {
		weekdayStrings[index] = strconv.Itoa(day)
	}
	weekdays, err := team.ParseWeekdays(weekdayStrings)
	if err != nil {
		return team.ScheduleDraft{}, nil, clientError{message: err.Error()}
	}
	start, err := parseClock(input.StartTime)
	if err != nil {
		return team.ScheduleDraft{}, nil, err
	}
	end, err := parseClock(input.EndTime)
	if err != nil {
		return team.ScheduleDraft{}, nil, err
	}
	if start == end {
		return team.ScheduleDraft{}, nil, clientError{message: "start and end time must differ"}
	}
	if !containsInt([]int{1, 2, 3, 5, 7, 10, 14}, input.PublishLeadDays) {
		return team.ScheduleDraft{}, nil, clientError{message: "choose a valid publication lead time"}
	}
	location, _ := time.LoadLocation(input.Timezone)
	period, err := time.ParseInLocation("2006-01-02", input.FirstPeriodStart, location)
	if err != nil || period.Format("2006-01-02") != input.FirstPeriodStart {
		return team.ScheduleDraft{}, nil, clientError{message: "choose a valid first period date"}
	}
	now := time.Now().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	if period.Before(today) {
		return team.ScheduleDraft{}, nil, clientError{message: "first period date cannot be in the past"}
	}
	draft := team.ScheduleDraft{TeamID: value.ID, TeamName: value.Name, GuildID: value.GuildID, UserID: userID, Timezone: input.Timezone, CadenceDays: input.CadenceDays, Weekdays: weekdays, StartMinutes: start, EndMinutes: end, PublishLeadDays: input.PublishLeadDays, FirstPeriodStart: period, ChannelID: input.ChannelID}
	occurrences, err := team.GenerateOccurrences(team.Schedule{Timezone: draft.Timezone, CadenceDays: draft.CadenceDays, StartMinutes: draft.StartMinutes, EndMinutes: draft.EndMinutes, Weekdays: draft.Weekdays, NextPeriodStart: draft.FirstPeriodStart})
	if err != nil {
		return team.ScheduleDraft{}, nil, clientError{message: err.Error()}
	}
	if len(occurrences) == 0 {
		return team.ScheduleDraft{}, nil, clientError{message: "schedule produced no occurrences"}
	}
	draft.FirstPublishAt, err = team.FirstPublication(occurrences[0].StartsAt, draft.Timezone, draft.PublishLeadDays)
	return draft, occurrences, err
}

func parseClock(value string) (int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil || parsed.Format("15:04") != value {
		return 0, clientError{message: "choose a valid time"}
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func containsInt(values []int, wanted int) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func (h *Handler) setScheduleEnabled(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Enabled bool `json:"enabled"`
	}
	if err := decode(r, &input); err != nil {
		h.fail(w, r, clientError{err.Error()})
		return
	}
	current := sessionFrom(r.Context())
	value, err := h.leaderTeam(r.Context(), current, r.PathValue("teamID"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.bot.SetTeamScheduleEnabled(r.Context(), value.ID, current.User.ID, input.Enabled); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	current := sessionFrom(r.Context())
	value, err := h.leaderTeam(r.Context(), current, r.PathValue("teamID"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.bot.PublishTeam(r.Context(), value.ID, current.User.ID); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) republish(w http.ResponseWriter, r *http.Request) {
	current := sessionFrom(r.Context())
	value, err := h.leaderTeam(r.Context(), current, r.PathValue("teamID"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.bot.RepublishTeam(r.Context(), value.ID, current.User.ID); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type pollResponse struct {
	ID          int64                `json:"id"`
	PeriodStart string               `json:"periodStart"`
	PeriodEnd   string               `json:"periodEnd"`
	Timezone    string               `json:"timezone"`
	Occurrences []occurrenceResponse `json:"occurrences"`
}

func (h *Handler) openPolls(w http.ResponseWriter, r *http.Request) {
	current := sessionFrom(r.Context())
	value, err := h.memberTeam(r.Context(), current, r.PathValue("teamID"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	views, err := h.store.OpenPolls(r.Context(), value.ID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	result := make([]pollResponse, 0, len(views))
	for _, view := range views {
		result = append(result, pollJSON(view))
	}
	writeJSON(w, http.StatusOK, result)
}

func pollJSON(view team.PollView) pollResponse {
	result := pollResponse{ID: view.Poll.ID, PeriodStart: view.Poll.PeriodStart.Format("2006-01-02"), PeriodEnd: view.Poll.PeriodEnd.AddDate(0, 0, -1).Format("2006-01-02"), Timezone: view.Poll.Timezone, Occurrences: []occurrenceResponse{}}
	for _, occurrence := range view.Poll.Occurrences {
		available, unavailable := 0, 0
		votes := make([]voteResponse, 0, len(view.Members))
		for _, memberID := range view.Members {
			choice, submitted := view.Availability[occurrence.ID][memberID]
			if !submitted {
				votes = append(votes, voteResponse{MemberID: memberID, Status: "pending"})
				continue
			}
			if choice {
				available++
				votes = append(votes, voteResponse{MemberID: memberID, Status: "available"})
			} else {
				unavailable++
				votes = append(votes, voteResponse{MemberID: memberID, Status: "unavailable"})
			}
		}
		result.Occurrences = append(result.Occurrences, occurrenceResponse{ID: occurrence.ID, StartsAt: occurrence.StartsAt, EndsAt: occurrence.EndsAt, Status: occurrence.Status, Available: available, Unavailable: unavailable, Pending: len(view.Members) - available - unavailable, Votes: votes})
	}
	return result
}

func (h *Handler) availability(w http.ResponseWriter, r *http.Request) {
	pollID, err := strconv.ParseInt(r.PathValue("pollID"), 10, 64)
	if err != nil || pollID <= 0 {
		h.fail(w, r, clientError{message: "invalid timetable"})
		return
	}
	var input struct {
		Available *[]int64 `json:"available"`
	}
	if err := decode(r, &input); err != nil {
		h.fail(w, r, clientError{err.Error()})
		return
	}
	if input.Available == nil {
		h.fail(w, r, clientError{message: "available must list the dates you can attend"})
		return
	}
	for _, id := range *input.Available {
		if id <= 0 {
			h.fail(w, r, clientError{message: "invalid raid date"})
			return
		}
	}
	// SetAvailability only accepts members of this poll's roster snapshot.
	if err := h.bot.SubmitAvailability(r.Context(), pollID, sessionFrom(r.Context()).User.ID, *input.Available); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) occurrenceStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("occurrenceID"), 10, 64)
	if err != nil || id <= 0 {
		h.fail(w, r, clientError{message: "invalid raid date"})
		return
	}
	var input struct {
		Action string `json:"action"`
	}
	if err := decode(r, &input); err != nil {
		h.fail(w, r, clientError{err.Error()})
		return
	}
	if input.Action != "confirm" && input.Action != "cancel" && input.Action != "reopen" {
		h.fail(w, r, clientError{message: "action must be confirm, cancel, or reopen"})
		return
	}
	// SetOccurrenceStatus authorizes against the leader of the date's team.
	if err := h.bot.ChangeOccurrence(r.Context(), id, sessionFrom(r.Context()).User.ID, input.Action); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) memberTeam(ctx context.Context, current session, rawID string) (team.Team, error) {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return team.Team{}, clientError{message: "invalid team"}
	}
	value, err := h.store.Team(ctx, id)
	if err != nil {
		return value, err
	}
	if !current.hasGuild(value.GuildID) || !containsString(value.MemberIDs, current.User.ID) {
		return team.Team{}, postgres.ErrForbidden
	}
	return value, nil
}

func (h *Handler) leaderTeam(ctx context.Context, current session, rawID string) (team.Team, error) {
	value, err := h.memberTeam(ctx, current, rawID)
	if err != nil {
		return value, err
	}
	if value.LeaderID != current.User.ID {
		return team.Team{}, postgres.ErrForbidden
	}
	return value, nil
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func (h *Handler) postChannel(parent context.Context, guildID, channelID string) (*discordgo.Channel, error) {
	if channelID == "" {
		return nil, clientError{message: "choose a Discord channel"}
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	channel, err := h.discord.Channel(channelID, discordgo.WithContext(ctx))
	if err != nil || channel.GuildID != guildID {
		return nil, clientError{message: "choose a channel from this Discord server"}
	}
	permissions, err := h.botChannelPermissions(ctx, guildID, []*discordgo.Channel{channel})
	if err != nil {
		return nil, err
	}
	needed := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)
	if permissions[channelID]&needed != needed {
		return nil, clientError{message: "the bot cannot post embeds in that channel"}
	}
	return channel, nil
}

func (h *Handler) botChannelPermissions(ctx context.Context, guildID string, channels []*discordgo.Channel) (map[string]int64, error) {
	if _, err := h.discord.State.Guild(guildID); err != nil {
		guild, fetchErr := h.discord.Guild(guildID, discordgo.WithContext(ctx))
		if fetchErr != nil {
			return nil, fetchErr
		}
		if err := h.discord.State.GuildAdd(guild); err != nil {
			return nil, err
		}
	}
	botID := h.discord.State.User.ID
	if _, err := h.discord.State.Member(guildID, botID); err != nil {
		member, fetchErr := h.discord.GuildMember(guildID, botID, discordgo.WithContext(ctx))
		if fetchErr != nil {
			return nil, fetchErr
		}
		if err := h.discord.State.MemberAdd(member); err != nil {
			return nil, err
		}
	}
	result := make(map[string]int64, len(channels))
	for _, channel := range channels {
		if err := h.discord.State.ChannelAdd(channel); err != nil {
			return nil, err
		}
		permissions, err := h.discord.State.UserChannelPermissions(botID, channel.ID)
		if err != nil {
			return nil, err
		}
		result[channel.ID] = permissions
	}
	return result, nil
}
