import { ApiError, type Occurrence, type Poll, type Schedule, type Team } from "@/api";
import {
  createState,
  type DemoOccurrence,
  type DemoPoll,
  type DemoSchedule,
  type DemoState,
  type DemoTeam
} from "./data";
import { addDays, localDate, raidTimes, weekday } from "./time";

/**
 * An in-browser stand-in for the Raidy API, so the real web app runs as a
 * demo with no server. It follows the server's rules closely enough that the
 * app behaves the same; state resets on reload.
 */
let state: DemoState = createState();

const rosterLimit = 8;

export async function demoApi(path: string, options: RequestInit = {}): Promise<unknown> {
  await new Promise((resolve) => setTimeout(resolve, 150 + Math.random() * 150));
  const method = (options.method ?? "GET").toUpperCase();
  const url = new URL(path, "http://demo");
  const body = typeof options.body === "string" ? JSON.parse(options.body) : {};
  const parts = url.pathname.split("/").filter(Boolean).slice(1);
  const pattern = parts.map((part, index) =>
    /^\d+$/.test(part) ? ":id" : parts[0] === "guilds" && index === 1 ? ":guild" : part
  );
  const route = `${method} /${pattern.join("/")}`;
  const id = Number(parts.find((part) => /^\d+$/.test(part)));

  switch (route) {
    case "GET /config":
      return { inviteUrl: "" };
    case "GET /auth/me":
      return state.user;
    case "POST /auth/logout":
      return null;
    case "GET /guilds":
      return state.guilds;
    case "GET /guilds/:guild/members": {
      const query = (url.searchParams.get("q") ?? "").toLowerCase();
      return state.people.filter((person) => person.name.toLowerCase().includes(query));
    }
    case "GET /guilds/:guild/channels":
      return state.channels;
    case "GET /teams":
      return state.teams.map(teamJSON);
    case "POST /teams":
      return createTeam(body);
    case "PATCH /teams/:id":
      return updateTeam(leaderTeam(id), body);
    case "DELETE /teams/:id":
      return deleteTeam(leaderTeam(id));
    case "GET /teams/:id/schedule": {
      const saved = state.schedules[team(id).id];
      return saved ? scheduleJSON(team(id), saved) : null;
    }
    case "PUT /teams/:id/schedule":
      return saveSchedule(leaderTeam(id), body);
    case "POST /teams/:id/schedule/preview":
      return previewSchedule(leaderTeam(id), body);
    case "POST /teams/:id/schedule/enabled":
      scheduleFor(leaderTeam(id)).enabled = Boolean(body.enabled);
      return null;
    case "POST /teams/:id/publish":
      return publish(leaderTeam(id));
    case "POST /teams/:id/republish":
      leaderTeam(id);
      return null;
    case "GET /teams/:id/polls":
      return openPolls(team(id)).map(pollJSON);
    case "POST /occurrences/:id/status":
      return setStatus(id, body.action);
    case "PUT /polls/:id/availability":
      return setAvailability(id, body.available);
    default:
      throw new ApiError(404, "Not found");
  }
}

function team(id: number) {
  const value = state.teams.find((candidate) => candidate.id === id);
  if (!value) throw new ApiError(404, "Team not found");
  return value;
}

function leaderTeam(id: number) {
  const value = team(id);
  if (value.leaderId !== state.user.id) throw new ApiError(403, "Only the team leader can do that");
  return value;
}

function member(id: string) {
  return state.people.find((person) => person.id === id) ?? { id, name: "Former member" };
}

function teamJSON(value: DemoTeam): Team {
  return {
    id: value.id,
    guildId: value.guildId,
    guildName: state.guilds.find((guild) => guild.id === value.guildId)?.name ?? "",
    name: value.name,
    timezone: value.timezone,
    leaderId: value.leaderId,
    isLeader: value.leaderId === state.user.id,
    members: value.memberIds.map(member)
  };
}

function roster(memberIds: unknown): string[] {
  const ids = [state.user.id, ...(Array.isArray(memberIds) ? memberIds.map(String) : [])];
  const unique = [...new Set(ids)];
  if (unique.length > rosterLimit) throw new ApiError(400, `a team can have at most ${rosterLimit} members`);
  return unique;
}

function teamName(name: unknown, guildId: string, except?: number) {
  const value = String(name ?? "")
    .trim()
    .replace(/\s+/g, " ");
  if (!value) throw new ApiError(400, "team name is required");
  const taken = state.teams.some(
    (other) => other.id !== except && other.guildId === guildId && other.name.toLowerCase() === value.toLowerCase()
  );
  if (taken) throw new ApiError(409, "a team with this name already exists in this server");
  return value;
}

function createTeam(body: { guildId?: string; name?: string; timezone?: string; memberIds?: string[] }) {
  const guildId = body.guildId ?? "g1";
  const value: DemoTeam = {
    id: state.nextID++,
    guildId,
    name: teamName(body.name, guildId),
    timezone: body.timezone || "UTC",
    leaderId: state.user.id,
    memberIds: roster(body.memberIds)
  };
  state.teams.push(value);
  return teamJSON(value);
}

function updateTeam(value: DemoTeam, body: { name?: string; memberIds?: string[] }) {
  value.name = teamName(body.name, value.guildId, value.id);
  value.memberIds = roster(body.memberIds);
  // Open periods follow the roster: newcomers start unanswered, leavers' votes go.
  for (const poll of openPolls(value)) {
    for (const occurrence of poll.occurrences) {
      for (const memberId of Object.keys(occurrence.votes)) {
        if (!value.memberIds.includes(memberId)) delete occurrence.votes[memberId];
      }
    }
    refreshStatuses(poll, value);
  }
  return teamJSON(value);
}

function deleteTeam(value: DemoTeam) {
  state.teams = state.teams.filter((other) => other.id !== value.id);
  state.polls = state.polls.filter((poll) => poll.teamId !== value.id);
  delete state.schedules[value.id];
  return null;
}

function scheduleFor(value: DemoTeam) {
  const saved = state.schedules[value.id];
  if (!saved) throw new ApiError(404, "This team has no schedule yet");
  return saved;
}

function occurrencesFor(value: DemoTeam, schedule: DemoSchedule, periodStart: string) {
  const result: { startsAt: string; endsAt: string }[] = [];
  for (let day = 0; day < schedule.cadenceDays; day++) {
    const date = addDays(periodStart, day);
    if (schedule.weekdays.includes(weekday(date))) {
      result.push(raidTimes(date, schedule.startTime, schedule.endTime, value.timezone));
    }
  }
  return result;
}

function publishAt(value: DemoTeam, schedule: DemoSchedule, periodStart: string) {
  const first = occurrencesFor(value, schedule, periodStart)[0];
  const at = new Date(first?.startsAt ?? `${periodStart}T00:00:00Z`);
  at.setUTCDate(at.getUTCDate() - schedule.publishLeadDays);
  return at.toISOString();
}

function scheduleJSON(value: DemoTeam, schedule: DemoSchedule): Schedule {
  return {
    timezone: value.timezone,
    cadenceDays: schedule.cadenceDays,
    weekdays: schedule.weekdays,
    startTime: schedule.startTime,
    endTime: schedule.endTime,
    publishLeadDays: schedule.publishLeadDays,
    firstPeriodStart: schedule.firstPeriodStart,
    nextPublishAt: publishAt(value, schedule, schedule.firstPeriodStart),
    channelId: schedule.channelId,
    channelName: state.channels.find((channel) => channel.id === schedule.channelId)?.name ?? "",
    enabled: schedule.enabled
  };
}

type ScheduleInput = Omit<DemoSchedule, "enabled"> & { timezone: string };

function scheduleInput(value: DemoTeam, body: Partial<ScheduleInput>) {
  const timezone = body.timezone || value.timezone;
  const input: DemoSchedule = {
    cadenceDays: Number(body.cadenceDays),
    weekdays: [...new Set((body.weekdays ?? []).map(Number))].sort(),
    startTime: String(body.startTime ?? ""),
    endTime: String(body.endTime ?? ""),
    publishLeadDays: Number(body.publishLeadDays) || 3,
    firstPeriodStart: String(body.firstPeriodStart ?? ""),
    channelId: String(body.channelId ?? ""),
    enabled: true
  };
  if (input.cadenceDays !== 7 && input.cadenceDays !== 14)
    throw new ApiError(400, "cadence must be weekly or biweekly");
  if (input.weekdays.length === 0) throw new ApiError(400, "select at least one weekday");
  if (!/^\d{2}:\d{2}$/.test(input.startTime) || !/^\d{2}:\d{2}$/.test(input.endTime)) {
    throw new ApiError(400, "choose a start and end time");
  }
  if (input.startTime === input.endTime) throw new ApiError(400, "start and end time must differ");
  if (!/^\d{4}-\d{2}-\d{2}$/.test(input.firstPeriodStart)) throw new ApiError(400, "choose a valid first period date");
  if (input.firstPeriodStart < localDate(new Date(), timezone)) {
    throw new ApiError(400, "first period date cannot be in the past");
  }
  if (!state.channels.some((channel) => channel.id === input.channelId)) throw new ApiError(400, "choose a channel");
  return { timezone, input };
}

function previewSchedule(value: DemoTeam, body: Partial<ScheduleInput>) {
  const { timezone, input } = scheduleInput(value, body);
  const draft = { ...value, timezone };
  return {
    ...scheduleJSON(draft, input),
    occurrences: occurrencesFor(draft, input, input.firstPeriodStart).map((times) => ({ ...times, status: "proposed" }))
  };
}

function saveSchedule(value: DemoTeam, body: Partial<ScheduleInput>) {
  const { timezone, input } = scheduleInput(value, body);
  const previousTimezone = value.timezone;
  // A new raid time moves upcoming dates in open periods and keeps their
  // votes; weekday changes start with the next period.
  for (const poll of openPolls(value)) {
    for (const occurrence of poll.occurrences) {
      if (new Date(occurrence.startsAt) <= new Date()) continue;
      const date = localDate(new Date(occurrence.startsAt), previousTimezone);
      Object.assign(occurrence, raidTimes(date, input.startTime, input.endTime, timezone));
    }
  }
  value.timezone = timezone;
  state.schedules[value.id] = { ...input, enabled: state.schedules[value.id]?.enabled ?? true };
  return scheduleJSON(value, state.schedules[value.id]);
}

function publish(value: DemoTeam) {
  const schedule = scheduleFor(value);
  const today = localDate(new Date(), value.timezone);
  if (state.polls.some((poll) => poll.teamId === value.id && poll.periodStart >= today)) {
    throw new ApiError(409, "the next period is already published; use Republish to restore its message");
  }
  state.polls.push({
    id: state.nextID++,
    teamId: value.id,
    periodStart: schedule.firstPeriodStart,
    cadenceDays: schedule.cadenceDays,
    occurrences: occurrencesFor(value, schedule, schedule.firstPeriodStart).map((times) => ({
      id: state.nextID++,
      ...times,
      status: "proposed",
      votes: {}
    }))
  });
  schedule.firstPeriodStart = addDays(schedule.firstPeriodStart, schedule.cadenceDays);
  return null;
}

function openPolls(value: DemoTeam) {
  const now = new Date();
  return state.polls
    .filter((poll) => poll.teamId === value.id && poll.occurrences.some((o) => new Date(o.startsAt) > now))
    .sort((a, b) => a.periodStart.localeCompare(b.periodStart));
}

function pollJSON(poll: DemoPoll): Poll {
  const value = team(poll.teamId);
  return {
    id: poll.id,
    periodStart: poll.periodStart,
    periodEnd: addDays(poll.periodStart, poll.cadenceDays - 1),
    timezone: value.timezone,
    occurrences: poll.occurrences.map((occurrence): Occurrence => {
      const votes = value.memberIds.map((memberId) => ({
        memberId,
        status: (memberId in occurrence.votes
          ? occurrence.votes[memberId]
            ? "available"
            : "unavailable"
          : "pending") as "available" | "unavailable" | "pending"
      }));
      const count = (status: string) => votes.filter((vote) => vote.status === status).length;
      return {
        id: occurrence.id,
        startsAt: occurrence.startsAt,
        endsAt: occurrence.endsAt,
        status: occurrence.status,
        available: count("available"),
        unavailable: count("unavailable"),
        pending: count("pending"),
        votes
      };
    })
  };
}

function findOccurrence(id: number) {
  for (const poll of state.polls) {
    const occurrence = poll.occurrences.find((value) => value.id === id);
    if (occurrence) return { poll, occurrence };
  }
  throw new ApiError(404, "Raid date not found");
}

function everyoneAvailable(occurrence: DemoOccurrence, value: DemoTeam) {
  return value.memberIds.every((memberId) => occurrence.votes[memberId] === true);
}

/** Confirmed dates turn to Attention required when someone can no longer make it, and back. */
function refreshStatuses(poll: DemoPoll, value: DemoTeam) {
  for (const occurrence of poll.occurrences) {
    if (new Date(occurrence.startsAt) <= new Date()) continue;
    if (occurrence.status === "confirmed" || occurrence.status === "attention_required") {
      occurrence.status = everyoneAvailable(occurrence, value) ? "confirmed" : "attention_required";
    }
  }
}

function setStatus(id: number, action: string) {
  const { poll, occurrence } = findOccurrence(id);
  const value = leaderTeam(poll.teamId);
  if (new Date(occurrence.startsAt) <= new Date()) {
    throw new ApiError(409, "this raid date has already started or its timetable has closed");
  }
  const next = { confirm: "", cancel: "cancelled", reopen: "proposed" }[action];
  if (next === undefined) throw new ApiError(400, "invalid occurrence action");
  occurrence.status = next || (everyoneAvailable(occurrence, value) ? "confirmed" : "attention_required");
  return null;
}

function setAvailability(pollID: number, available: unknown) {
  const poll = state.polls.find((value) => value.id === pollID);
  if (!poll) throw new ApiError(404, "Timetable not found");
  const value = team(poll.teamId);
  if (!value.memberIds.includes(state.user.id)) throw new ApiError(403, "You are not on this team");
  if (!Array.isArray(available)) throw new ApiError(400, "available must be a list of raid dates");
  const wanted = new Set(available.map(Number));
  const upcoming = poll.occurrences.filter((occurrence) => new Date(occurrence.startsAt) > new Date());
  if (upcoming.length === 0) throw new ApiError(409, "this raid date has already started or its timetable has closed");
  if ([...wanted].some((id) => !upcoming.some((occurrence) => occurrence.id === id))) {
    throw new ApiError(400, "invalid occurrence");
  }
  for (const occurrence of upcoming) occurrence.votes[state.user.id] = wanted.has(occurrence.id);
  refreshStatuses(poll, value);
  return null;
}
