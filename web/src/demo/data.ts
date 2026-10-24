import { type Channel, type Guild, type Member, type User } from "@/api";
import { addDays, localDate, raidTimes, weekday } from "./time";

export type DemoOccurrence = {
  id: number;
  startsAt: string;
  endsAt: string;
  status: string;
  /** Member ID to whether they can attend; missing means no answer yet. */
  votes: Record<string, boolean>;
};
export type DemoPoll = {
  id: number;
  teamId: number;
  periodStart: string;
  cadenceDays: number;
  occurrences: DemoOccurrence[];
};
export type DemoTeam = {
  id: number;
  guildId: string;
  name: string;
  timezone: string;
  leaderId: string;
  memberIds: string[];
};
export type DemoSchedule = {
  cadenceDays: number;
  weekdays: number[];
  startTime: string;
  endTime: string;
  publishLeadDays: number;
  firstPeriodStart: string;
  channelId: string;
  enabled: boolean;
};
export type DemoState = {
  user: User;
  guilds: Guild[];
  people: Member[];
  channels: Channel[];
  teams: DemoTeam[];
  schedules: Record<number, DemoSchedule>;
  polls: DemoPoll[];
  nextID: number;
};

const people: Member[] = [
  "Alisaie",
  "Estinien",
  "G'raha",
  "Thancred",
  "Urianger",
  "Y'shtola",
  "Alphinaud",
  "Krile",
  "Tataru",
  "Lyse",
  "Aymeric",
  "Hien",
  "Lucia",
  "Wuk Lamat"
].map((name, index) => ({ id: `u${index}`, name }));

/** Votes per raid night, as the indexes of members who can and can't make it. */
const answers: { yes: number[]; no: number[] }[][] = [
  [
    { yes: [0, 1, 2, 3, 4, 5, 6, 7], no: [] },
    { yes: [0, 1, 2, 3, 4, 5, 6], no: [7] },
    { yes: [0, 1, 2, 3, 5], no: [4] }
  ],
  [
    { yes: [1, 2, 5], no: [] },
    { yes: [1, 3], no: [6] },
    { yes: [2], no: [1] }
  ]
];

/**
 * Sample static in Tokyo with this week's voting open and next week's opened
 * early. Dates follow today, so the demo never looks stale.
 */
export function createState(now = new Date()): DemoState {
  const timezone = "Asia/Tokyo";
  const schedule: DemoSchedule = {
    cadenceDays: 7,
    weekdays: [3, 4, 5],
    startTime: "21:00",
    endTime: "23:00",
    publishLeadDays: 3,
    firstPeriodStart: "",
    channelId: "c1",
    enabled: true
  };
  const today = localDate(now, timezone);
  let monday = addDays(today, -((weekday(today) + 6) % 7));
  // Once Friday's raid has started, this week is over; show next week instead.
  if (new Date(raidTimes(addDays(monday, 4), schedule.startTime, schedule.endTime, timezone).startsAt) <= now) {
    monday = addDays(monday, 7);
  }
  let nextID = 100;
  const memberIds = people.slice(0, 8).map((person) => person.id);
  const polls = answers.map((week, index): DemoPoll => {
    const periodStart = addDays(monday, index * 7);
    return {
      id: nextID++,
      teamId: 1,
      periodStart,
      cadenceDays: 7,
      occurrences: week.map((answer, day) => {
        const votes: Record<string, boolean> = {};
        for (const member of answer.yes) votes[memberIds[member]] = true;
        for (const member of answer.no) votes[memberIds[member]] = false;
        return {
          id: nextID++,
          ...raidTimes(addDays(periodStart, 2 + day), schedule.startTime, schedule.endTime, timezone),
          status: index === 0 && day === 0 ? "confirmed" : "proposed",
          votes
        };
      })
    };
  });
  schedule.firstPeriodStart = addDays(monday, 14);
  return {
    user: { id: "u0", username: "alisaie", globalName: "Alisaie", avatar: "" },
    guilds: [{ id: "g1", name: "Scions of the Seventh Dawn", icon: "" }],
    people,
    channels: [
      { id: "c1", name: "raid-schedule" },
      { id: "c2", name: "general" },
      { id: "c3", name: "announcements" }
    ],
    teams: [{ id: 1, guildId: "g1", name: "The Echo", timezone, leaderId: "u0", memberIds }],
    schedules: { 1: schedule },
    polls,
    nextID
  };
}
