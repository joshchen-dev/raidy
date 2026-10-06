package team

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Team struct {
	ID        int64
	GuildID   string
	GuildName string
	Name      string
	Timezone  string
	LeaderID  string
	MemberIDs []string
}

type Schedule struct {
	TeamID          int64
	TeamName        string
	Timezone        string
	LeaderID        string
	CadenceDays     int
	StartMinutes    int
	EndMinutes      int
	PublishLeadDays int
	Weekdays        []time.Weekday
	NextPeriodStart time.Time
	NextPublishAt   time.Time
	ChannelID       string
	ChannelName     string
	Enabled         bool
}

type Occurrence struct {
	ID       int64
	PollID   int64
	StartsAt time.Time
	EndsAt   time.Time
	Status   string
}

type Poll struct {
	ID          int64
	TeamID      int64
	TeamName    string
	Timezone    string
	LeaderID    string
	PeriodStart time.Time
	PeriodEnd   time.Time
	ChannelID   string
	MessageID   string
	Occurrences []Occurrence
}

func (p Poll) Expired(at time.Time) bool {
	for _, occurrence := range p.Occurrences {
		if occurrence.StartsAt.After(at) {
			return false
		}
	}
	return true
}

type PollView struct {
	Poll         Poll
	Members      []string
	Submitted    map[string]bool
	Availability map[int64]map[string]bool
}

type ScheduleDraft struct {
	GuildID          string
	UserID           string
	TeamID           int64
	TeamName         string
	Timezone         string
	CadenceDays      int
	Weekdays         []time.Weekday
	StartMinutes     int
	EndMinutes       int
	PublishLeadDays  int
	FirstPeriodStart time.Time
	FirstPublishAt   time.Time
	ChannelID        string
	ChannelName      string
}

func NormalizeName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func DisplayName(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func FormatClock(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

func FirstPublication(firstRaid time.Time, timezone string, leadDays int) (time.Time, error) {
	if leadDays <= 0 {
		return time.Time{}, errors.New("publication lead time must be positive")
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, err
	}
	return firstRaid.In(loc).AddDate(0, 0, -leadDays).UTC(), nil
}

func GenerateOccurrences(s Schedule) ([]Occurrence, error) {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return nil, err
	}
	wanted := make(map[time.Weekday]bool, len(s.Weekdays))
	for _, weekday := range s.Weekdays {
		wanted[weekday] = true
	}
	var occurrences []Occurrence
	for day := 0; day < s.CadenceDays; day++ {
		date := s.NextPeriodStart.AddDate(0, 0, day)
		if !wanted[date.Weekday()] {
			continue
		}
		start := localDateTime(date, s.StartMinutes, loc)
		if start.Hour()*60+start.Minute() != s.StartMinutes {
			return nil, fmt.Errorf("%s %s does not exist in %s due to a clock change", date.Format("2006-01-02"), FormatClock(s.StartMinutes), s.Timezone)
		}
		end := localDateTime(date, s.EndMinutes, loc)
		if !end.After(start) {
			end = end.AddDate(0, 0, 1)
		}
		if end.Hour()*60+end.Minute() != s.EndMinutes {
			return nil, fmt.Errorf("end time %s does not exist in %s due to a clock change", FormatClock(s.EndMinutes), s.Timezone)
		}
		occurrences = append(occurrences, Occurrence{StartsAt: start.UTC(), EndsAt: end.UTC(), Status: "proposed"})
	}
	return occurrences, nil
}

func localDateTime(date time.Time, minutes int, loc *time.Location) time.Time {
	y, m, d := date.Date()
	return time.Date(y, m, d, minutes/60, minutes%60, 0, 0, loc)
}

func ParseWeekdays(values []string) ([]time.Weekday, error) {
	seen := make(map[time.Weekday]bool)
	days := make([]time.Weekday, 0, len(values))
	for _, value := range values {
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 || n > 6 {
			return nil, fmt.Errorf("invalid weekday %q", value)
		}
		day := time.Weekday(n)
		if !seen[day] {
			seen[day] = true
			days = append(days, day)
		}
	}
	if len(days) == 0 {
		return nil, errors.New("select at least one weekday")
	}
	sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })
	return days, nil
}
