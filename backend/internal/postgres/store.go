package postgres

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/joshchen-dev/raidy/internal/team"
)

var (
	ErrForbidden = errors.New("forbidden")
	ErrNotFound  = errors.New("not found")
	ErrExpired   = errors.New("poll expired")
)

type Store struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close()                         { s.pool.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *Store) CreateTeam(ctx context.Context, guildID, guildName, leaderID, name, timezone string, members []string) (team.Team, error) {
	members, err := roster(leaderID, members)
	if err != nil {
		return team.Team{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return team.Team{}, err
	}
	defer tx.Rollback(ctx)

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO teams (guild_id, guild_name, name_key, display_name, timezone, leader_id)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		guildID, guildName, team.NormalizeName(name), team.DisplayName(name), timezone, leaderID,
	).Scan(&id)
	if err != nil {
		return team.Team{}, err
	}
	for _, memberID := range members {
		if _, err := tx.Exec(ctx, `INSERT INTO team_members (team_id, user_id) VALUES ($1, $2)`, id, memberID); err != nil {
			return team.Team{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return team.Team{}, err
	}
	return team.Team{ID: id, GuildID: guildID, GuildName: guildName, Name: team.DisplayName(name), Timezone: timezone, LeaderID: leaderID, MemberIDs: members}, nil
}

func (s *Store) ReplaceRoster(ctx context.Context, teamID int64, leaderID string, members []string) error {
	members, err := roster(leaderID, members)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := requireLeader(ctx, tx, teamID, leaderID); err != nil {
		return err
	}
	if err := replaceRoster(ctx, tx, teamID, members); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func replaceRoster(ctx context.Context, tx pgx.Tx, teamID int64, members []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM team_members WHERE team_id=$1`, teamID); err != nil {
		return err
	}
	for _, memberID := range members {
		if _, err := tx.Exec(ctx, `INSERT INTO team_members (team_id, user_id) VALUES ($1, $2)`, teamID, memberID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM poll_availability pa USING poll_occurrences o, schedule_polls p
		WHERE pa.occurrence_id=o.id AND o.poll_id=p.id AND p.team_id=$1 AND p.closed_at IS NULL
		  AND NOT EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id=$1 AND tm.user_id=pa.member_id)`, teamID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM poll_submissions ps USING schedule_polls p
		WHERE ps.poll_id=p.id AND p.team_id=$1 AND p.closed_at IS NULL
		  AND NOT EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id=$1 AND tm.user_id=ps.member_id)`, teamID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM poll_members pm USING schedule_polls p
		WHERE pm.poll_id=p.id AND p.team_id=$1 AND p.closed_at IS NULL
		  AND NOT EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id=$1 AND tm.user_id=pm.member_id)`, teamID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO poll_members (poll_id,member_id)
		SELECT p.id,tm.user_id FROM schedule_polls p JOIN team_members tm ON tm.team_id=p.team_id
		WHERE p.team_id=$1 AND p.closed_at IS NULL
		ON CONFLICT DO NOTHING`, teamID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		UPDATE poll_occurrences o SET status=CASE
			WHEN (SELECT count(*) FROM poll_members pm WHERE pm.poll_id=o.poll_id) =
			     (SELECT count(*) FROM poll_availability pa WHERE pa.occurrence_id=o.id AND pa.available)
			THEN 'confirmed' ELSE 'attention_required' END
		FROM schedule_polls p
		WHERE o.poll_id=p.id AND p.team_id=$1 AND p.closed_at IS NULL
		  AND o.status IN ('confirmed','attention_required')`, teamID)
	return err
}

func roster(leaderID string, members []string) ([]string, error) {
	seen := map[string]bool{leaderID: true}
	result := []string{leaderID}
	for _, id := range members {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	if len(result) > 8 {
		return nil, errors.New("a static team can have at most eight members")
	}
	sort.Strings(result)
	return result, nil
}

func (s *Store) TeamsLedBy(ctx context.Context, guildID, leaderID string) ([]team.Team, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.guild_id, t.guild_name, t.display_name, t.timezone, t.leader_id,
		       COALESCE(array_agg(tm.user_id ORDER BY tm.user_id) FILTER (WHERE tm.user_id IS NOT NULL), '{}')
		FROM teams t LEFT JOIN team_members tm ON tm.team_id=t.id
		WHERE t.guild_id=$1 AND t.leader_id=$2
		GROUP BY t.id ORDER BY t.display_name`, guildID, leaderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var teams []team.Team
	for rows.Next() {
		var value team.Team
		if err := rows.Scan(&value.ID, &value.GuildID, &value.GuildName, &value.Name, &value.Timezone, &value.LeaderID, &value.MemberIDs); err != nil {
			return nil, err
		}
		teams = append(teams, value)
	}
	return teams, rows.Err()
}

func (s *Store) TeamsForUser(ctx context.Context, userID string) ([]team.Team, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.guild_id, t.guild_name, t.display_name, t.timezone, t.leader_id,
		       COALESCE(array_agg(all_members.user_id ORDER BY all_members.user_id), '{}')
		FROM teams t
		JOIN team_members mine ON mine.team_id=t.id AND mine.user_id=$1
		JOIN team_members all_members ON all_members.team_id=t.id
		GROUP BY t.id ORDER BY t.display_name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var teams []team.Team
	for rows.Next() {
		var value team.Team
		if err := rows.Scan(&value.ID, &value.GuildID, &value.GuildName, &value.Name, &value.Timezone, &value.LeaderID, &value.MemberIDs); err != nil {
			return nil, err
		}
		teams = append(teams, value)
	}
	return teams, rows.Err()
}

func (s *Store) Team(ctx context.Context, teamID int64) (team.Team, error) {
	var value team.Team
	err := s.pool.QueryRow(ctx, `
		SELECT t.id, t.guild_id, t.guild_name, t.display_name, t.timezone, t.leader_id,
		       COALESCE(array_agg(tm.user_id ORDER BY tm.user_id) FILTER (WHERE tm.user_id IS NOT NULL), '{}')
		FROM teams t LEFT JOIN team_members tm ON tm.team_id=t.id
		WHERE t.id=$1 GROUP BY t.id`, teamID,
	).Scan(&value.ID, &value.GuildID, &value.GuildName, &value.Name, &value.Timezone, &value.LeaderID, &value.MemberIDs)
	if errors.Is(err, pgx.ErrNoRows) {
		return team.Team{}, ErrNotFound
	}
	return value, err
}

func (s *Store) DeleteTeam(ctx context.Context, teamID int64, leaderID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM teams WHERE id=$1 AND leader_id=$2`, teamID, leaderID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrForbidden
	}
	return nil
}

func (s *Store) UpdateGuildName(ctx context.Context, teamID int64, guildName string) error {
	_, err := s.pool.Exec(ctx, `UPDATE teams SET guild_name=$1 WHERE id=$2`, guildName, teamID)
	return err
}

func (s *Store) UpdateTeam(ctx context.Context, teamID int64, leaderID, name, timezone string, members []string) error {
	members, err := roster(leaderID, members)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := requireLeader(ctx, tx, teamID, leaderID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE teams SET name_key=$1,display_name=$2,timezone=$3 WHERE id=$4`, team.NormalizeName(name), team.DisplayName(name), timezone, teamID); err != nil {
		return err
	}
	if err := replaceRoster(ctx, tx, teamID, members); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) SaveSchedule(ctx context.Context, draft team.ScheduleDraft) error {
	if draft.CadenceDays != 7 && draft.CadenceDays != 14 {
		return errors.New("cadence must be weekly or biweekly")
	}
	if len(draft.Weekdays) == 0 {
		return errors.New("select at least one weekday")
	}
	if draft.PublishLeadDays == 0 {
		draft.PublishLeadDays = 3
	}
	if !validPublishLead(draft.PublishLeadDays) {
		return errors.New("invalid publication lead time")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := requireLeader(ctx, tx, draft.TeamID, draft.UserID); err != nil {
		return err
	}
	if draft.Timezone != "" {
		if _, err := time.LoadLocation(draft.Timezone); err != nil {
			return errors.New("invalid timezone")
		}
		if _, err := tx.Exec(ctx, `UPDATE teams SET timezone=$1 WHERE id=$2`, draft.Timezone, draft.TeamID); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO schedule_settings
		(team_id, cadence_days, start_minutes, end_minutes, publish_lead_days, next_period_start, next_publish_at, channel_id, channel_name, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,true)
		ON CONFLICT (team_id) DO UPDATE SET
		cadence_days=EXCLUDED.cadence_days, start_minutes=EXCLUDED.start_minutes,
		end_minutes=EXCLUDED.end_minutes, publish_lead_days=EXCLUDED.publish_lead_days, next_period_start=EXCLUDED.next_period_start,
		next_publish_at=EXCLUDED.next_publish_at, channel_id=EXCLUDED.channel_id, channel_name=EXCLUDED.channel_name, enabled=true`,
		draft.TeamID, draft.CadenceDays, draft.StartMinutes, draft.EndMinutes, draft.PublishLeadDays,
		draft.FirstPeriodStart, draft.FirstPublishAt.UTC(), draft.ChannelID, draft.ChannelName)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM schedule_weekdays WHERE team_id=$1`, draft.TeamID); err != nil {
		return err
	}
	for _, weekday := range draft.Weekdays {
		if _, err := tx.Exec(ctx, `INSERT INTO schedule_weekdays (team_id, weekday) VALUES ($1,$2)`, draft.TeamID, int(weekday)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func validPublishLead(days int) bool {
	for _, allowed := range []int{1, 2, 3, 5, 7, 10, 14} {
		if days == allowed {
			return true
		}
	}
	return false
}

func (s *Store) Schedule(ctx context.Context, teamID int64) (team.Schedule, error) {
	var value team.Schedule
	err := s.pool.QueryRow(ctx, `
		SELECT t.id,t.display_name,t.timezone,t.leader_id,ss.cadence_days,ss.start_minutes,
		       ss.end_minutes,ss.publish_lead_days,ss.next_period_start,ss.next_publish_at,ss.channel_id,ss.channel_name,ss.enabled
		FROM teams t JOIN schedule_settings ss ON ss.team_id=t.id WHERE t.id=$1`, teamID,
	).Scan(&value.TeamID, &value.TeamName, &value.Timezone, &value.LeaderID, &value.CadenceDays,
		&value.StartMinutes, &value.EndMinutes, &value.PublishLeadDays, &value.NextPeriodStart,
		&value.NextPublishAt, &value.ChannelID, &value.ChannelName, &value.Enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	if err != nil {
		return value, err
	}
	rows, err := s.pool.Query(ctx, `SELECT weekday FROM schedule_weekdays WHERE team_id=$1 ORDER BY weekday`, teamID)
	if err != nil {
		return value, err
	}
	defer rows.Close()
	for rows.Next() {
		var weekday int
		if err := rows.Scan(&weekday); err != nil {
			return value, err
		}
		value.Weekdays = append(value.Weekdays, time.Weekday(weekday))
	}
	return value, rows.Err()
}

func (s *Store) SetScheduleEnabled(ctx context.Context, teamID int64, leaderID string, enabled bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := requireLeader(ctx, tx, teamID, leaderID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE schedule_settings SET enabled=$1 WHERE team_id=$2`, enabled, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (s *Store) DueTeamIDs(ctx context.Context, now time.Time) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `SELECT team_id FROM schedule_settings WHERE enabled AND next_publish_at <= $1 ORDER BY next_publish_at`, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) PublishNext(ctx context.Context, teamID int64, force bool, now time.Time) (team.Poll, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return team.Poll{}, false, err
	}
	defer tx.Rollback(ctx)

	var schedule team.Schedule
	err = tx.QueryRow(ctx, `
		SELECT t.id,t.display_name,t.timezone,t.leader_id,ss.cadence_days,ss.start_minutes,
		       ss.end_minutes,ss.publish_lead_days,ss.next_period_start,ss.next_publish_at,ss.channel_id,ss.channel_name,ss.enabled
		FROM teams t JOIN schedule_settings ss ON ss.team_id=t.id
		WHERE t.id=$1 FOR UPDATE OF ss`, teamID,
	).Scan(&schedule.TeamID, &schedule.TeamName, &schedule.Timezone, &schedule.LeaderID,
		&schedule.CadenceDays, &schedule.StartMinutes, &schedule.EndMinutes, &schedule.PublishLeadDays,
		&schedule.NextPeriodStart, &schedule.NextPublishAt, &schedule.ChannelID, &schedule.ChannelName, &schedule.Enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return team.Poll{}, false, ErrNotFound
	}
	if err != nil {
		return team.Poll{}, false, err
	}
	if !force && (!schedule.Enabled || schedule.NextPublishAt.After(now)) {
		return team.Poll{}, false, tx.Commit(ctx)
	}
	rows, err := tx.Query(ctx, `SELECT weekday FROM schedule_weekdays WHERE team_id=$1 ORDER BY weekday`, teamID)
	if err != nil {
		return team.Poll{}, false, err
	}
	for rows.Next() {
		var weekday int
		if err := rows.Scan(&weekday); err != nil {
			rows.Close()
			return team.Poll{}, false, err
		}
		schedule.Weekdays = append(schedule.Weekdays, time.Weekday(weekday))
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return team.Poll{}, false, err
	}
	occurrences, err := team.GenerateOccurrences(schedule)
	if err != nil {
		return team.Poll{}, false, err
	}
	if len(occurrences) == 0 {
		return team.Poll{}, false, errors.New("schedule produced no occurrences")
	}

	periodEnd := schedule.NextPeriodStart.AddDate(0, 0, schedule.CadenceDays)
	var pollID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO schedule_polls (team_id,period_start,period_end,channel_id)
		VALUES ($1,$2,$3,$4) ON CONFLICT (team_id,period_start) DO NOTHING RETURNING id`,
		teamID, schedule.NextPeriodStart, periodEnd, schedule.ChannelID,
	).Scan(&pollID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `SELECT id FROM schedule_polls WHERE team_id=$1 AND period_start=$2`, teamID, schedule.NextPeriodStart).Scan(&pollID); err != nil {
			return team.Poll{}, false, err
		}
		if err := advanceSchedule(ctx, tx, schedule); err != nil {
			return team.Poll{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return team.Poll{}, false, err
		}
		return team.Poll{ID: pollID, TeamID: teamID}, true, nil
	}
	if err != nil {
		return team.Poll{}, false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO poll_members (poll_id,member_id) SELECT $1,user_id FROM team_members WHERE team_id=$2`, pollID, teamID); err != nil {
		return team.Poll{}, false, err
	}
	for i := range occurrences {
		occurrences[i].PollID = pollID
		if err := tx.QueryRow(ctx, `
			INSERT INTO poll_occurrences (poll_id,starts_at,ends_at,status)
			VALUES ($1,$2,$3,'proposed') RETURNING id`,
			pollID, occurrences[i].StartsAt, occurrences[i].EndsAt,
		).Scan(&occurrences[i].ID); err != nil {
			return team.Poll{}, false, err
		}
	}
	if err := advanceSchedule(ctx, tx, schedule); err != nil {
		return team.Poll{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return team.Poll{}, false, err
	}
	return team.Poll{ID: pollID, TeamID: teamID, TeamName: schedule.TeamName, Timezone: schedule.Timezone,
		LeaderID: schedule.LeaderID, PeriodStart: schedule.NextPeriodStart, PeriodEnd: periodEnd,
		ChannelID: schedule.ChannelID, Occurrences: occurrences}, true, nil
}

func advanceSchedule(ctx context.Context, tx pgx.Tx, schedule team.Schedule) error {
	loc, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		return err
	}
	nextPublish := schedule.NextPublishAt.In(loc).AddDate(0, 0, schedule.CadenceDays).UTC()
	nextPeriod := schedule.NextPeriodStart.AddDate(0, 0, schedule.CadenceDays)
	_, err = tx.Exec(ctx, `UPDATE schedule_settings SET next_period_start=$1,next_publish_at=$2 WHERE team_id=$3`, nextPeriod, nextPublish, schedule.TeamID)
	return err
}

func (s *Store) UnpublishedPolls(ctx context.Context) ([]team.Poll, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id,p.team_id,t.display_name,t.timezone,t.leader_id,p.period_start,p.period_end,p.channel_id,p.message_id
		FROM schedule_polls p JOIN teams t ON t.id=p.team_id
		WHERE p.message_id='' AND p.closed_at IS NULL ORDER BY p.created_at`)
	if err != nil {
		return nil, err
	}
	var polls []team.Poll
	for rows.Next() {
		var poll team.Poll
		if err := rows.Scan(&poll.ID, &poll.TeamID, &poll.TeamName, &poll.Timezone, &poll.LeaderID, &poll.PeriodStart, &poll.PeriodEnd, &poll.ChannelID, &poll.MessageID); err != nil {
			rows.Close()
			return nil, err
		}
		polls = append(polls, poll)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return polls, nil
}

func (s *Store) SetPollMessage(ctx context.Context, pollID int64, messageID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE schedule_polls SET message_id=$1 WHERE id=$2`, messageID, pollID)
	return err
}

func (s *Store) ClearPollMessage(ctx context.Context, pollID int64, leaderID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE schedule_polls p SET message_id=''
		FROM teams t WHERE p.id=$1 AND p.team_id=t.id AND t.leader_id=$2`, pollID, leaderID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrForbidden
	}
	return nil
}

func (s *Store) PollView(ctx context.Context, pollID int64) (team.PollView, error) {
	var view team.PollView
	err := s.pool.QueryRow(ctx, `
		SELECT p.id,p.team_id,t.display_name,t.timezone,t.leader_id,p.period_start,p.period_end,p.channel_id,p.message_id
		FROM schedule_polls p JOIN teams t ON t.id=p.team_id WHERE p.id=$1`, pollID,
	).Scan(&view.Poll.ID, &view.Poll.TeamID, &view.Poll.TeamName, &view.Poll.Timezone, &view.Poll.LeaderID,
		&view.Poll.PeriodStart, &view.Poll.PeriodEnd, &view.Poll.ChannelID, &view.Poll.MessageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return view, ErrNotFound
	}
	if err != nil {
		return view, err
	}
	view.Poll.Occurrences, err = s.occurrences(ctx, pollID)
	if err != nil {
		return view, err
	}
	view.Members, err = s.pollMembers(ctx, pollID)
	if err != nil {
		return view, err
	}
	view.Submitted = make(map[string]bool)
	view.Availability = make(map[int64]map[string]bool)
	rows, err := s.pool.Query(ctx, `SELECT member_id FROM poll_submissions WHERE poll_id=$1`, pollID)
	if err != nil {
		return view, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return view, err
		}
		view.Submitted[id] = true
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `
		SELECT pa.occurrence_id,pa.member_id,pa.available
		FROM poll_availability pa JOIN poll_occurrences o ON o.id=pa.occurrence_id
		WHERE o.poll_id=$1`, pollID)
	if err != nil {
		return view, err
	}
	defer rows.Close()
	for rows.Next() {
		var occurrenceID int64
		var memberID string
		var available bool
		if err := rows.Scan(&occurrenceID, &memberID, &available); err != nil {
			return view, err
		}
		if view.Availability[occurrenceID] == nil {
			view.Availability[occurrenceID] = make(map[string]bool)
		}
		view.Availability[occurrenceID][memberID] = available
	}
	return view, rows.Err()
}

func (s *Store) SetAvailability(ctx context.Context, pollID int64, memberID string, availableIDs []int64, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var member bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM poll_members WHERE poll_id=$1 AND member_id=$2)`, pollID, memberID).Scan(&member); err != nil {
		return err
	}
	if !member {
		return ErrForbidden
	}
	rows, err := tx.Query(ctx, `SELECT id FROM poll_occurrences WHERE poll_id=$1 AND starts_at>$2`, pollID, now.UTC())
	if err != nil {
		return err
	}
	valid := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		valid[id] = true
	}
	rows.Close()
	if len(valid) == 0 {
		return ErrExpired
	}
	wanted := make(map[int64]bool, len(availableIDs))
	for _, id := range availableIDs {
		if !valid[id] {
			return errors.New("invalid or expired occurrence")
		}
		wanted[id] = true
	}
	_, err = tx.Exec(ctx, `INSERT INTO poll_submissions (poll_id,member_id) VALUES ($1,$2) ON CONFLICT (poll_id,member_id) DO UPDATE SET submitted_at=now()`, pollID, memberID)
	if err != nil {
		return err
	}
	for id := range valid {
		_, err := tx.Exec(ctx, `INSERT INTO poll_availability (occurrence_id,member_id,available) VALUES ($1,$2,$3) ON CONFLICT (occurrence_id,member_id) DO UPDATE SET available=EXCLUDED.available`, id, memberID, wanted[id])
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE poll_occurrences o SET status=CASE
			WHEN (SELECT count(*) FROM poll_members pm WHERE pm.poll_id=o.poll_id) =
			     (SELECT count(*) FROM poll_availability pa WHERE pa.occurrence_id=o.id AND pa.available)
			THEN 'confirmed' ELSE 'attention_required' END
		WHERE o.poll_id=$1 AND o.starts_at>$2 AND o.status IN ('confirmed','attention_required')`, pollID, now.UTC())
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) SetOccurrenceStatus(ctx context.Context, occurrenceID int64, leaderID, action string, now time.Time) error {
	if action != "confirm" && action != "cancel" && action != "reopen" {
		return errors.New("invalid occurrence action")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var pollID int64
	var startsAt time.Time
	var actualLeader string
	err = tx.QueryRow(ctx, `SELECT o.poll_id,o.starts_at,t.leader_id FROM poll_occurrences o JOIN schedule_polls p ON p.id=o.poll_id JOIN teams t ON t.id=p.team_id WHERE o.id=$1 FOR UPDATE OF o`, occurrenceID).Scan(&pollID, &startsAt, &actualLeader)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if actualLeader != leaderID {
		return ErrForbidden
	}
	if !startsAt.After(now) {
		return ErrExpired
	}
	status := map[string]string{"cancel": "cancelled", "reopen": "proposed"}[action]
	if action == "confirm" {
		var allAvailable bool
		err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM poll_members WHERE poll_id=$1) = (SELECT count(*) FROM poll_availability WHERE occurrence_id=$2 AND available)`, pollID, occurrenceID).Scan(&allAvailable)
		if err != nil {
			return err
		}
		if allAvailable {
			status = "confirmed"
		} else {
			status = "attention_required"
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE poll_occurrences SET status=$1 WHERE id=$2`, status, occurrenceID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CloseExpiredPolls(ctx context.Context, now time.Time) ([]team.Poll, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE schedule_polls p SET closed_at=$1
		WHERE closed_at IS NULL AND NOT EXISTS (
			SELECT 1 FROM poll_occurrences o WHERE o.poll_id=p.id AND o.starts_at>$1
		)
		RETURNING id`, now.UTC())
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	polls := make([]team.Poll, 0, len(ids))
	for _, id := range ids {
		view, err := s.PollView(ctx, id)
		if err != nil {
			return nil, err
		}
		polls = append(polls, view.Poll)
	}
	return polls, nil
}

func (s *Store) LatestPoll(ctx context.Context, teamID int64) (team.PollView, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `SELECT id FROM schedule_polls WHERE team_id=$1 AND closed_at IS NULL ORDER BY period_start DESC LIMIT 1`, teamID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return team.PollView{}, ErrNotFound
	}
	if err != nil {
		return team.PollView{}, err
	}
	return s.PollView(ctx, id)
}

func (s *Store) PollIDForOccurrence(ctx context.Context, occurrenceID int64) (int64, error) {
	var pollID int64
	err := s.pool.QueryRow(ctx, `SELECT poll_id FROM poll_occurrences WHERE id=$1`, occurrenceID).Scan(&pollID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return pollID, err
}

func (s *Store) occurrences(ctx context.Context, pollID int64) ([]team.Occurrence, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,poll_id,starts_at,ends_at,status FROM poll_occurrences WHERE poll_id=$1 ORDER BY starts_at`, pollID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []team.Occurrence
	for rows.Next() {
		var o team.Occurrence
		if err := rows.Scan(&o.ID, &o.PollID, &o.StartsAt, &o.EndsAt, &o.Status); err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

func (s *Store) pollMembers(ctx context.Context, pollID int64) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT member_id FROM poll_members WHERE poll_id=$1 ORDER BY member_id`, pollID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func requireLeader(ctx context.Context, tx pgx.Tx, teamID int64, leaderID string) error {
	var actual string
	err := tx.QueryRow(ctx, `SELECT leader_id FROM teams WHERE id=$1 FOR UPDATE`, teamID).Scan(&actual)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if actual != leaderID {
		return ErrForbidden
	}
	return nil
}
