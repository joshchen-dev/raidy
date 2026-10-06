import { type Page, type Route } from "@playwright/test";

export const members = ["Alisaie", "Estinien", "G'raha", "Thancred", "Urianger", "Y'shtola", "Alphinaud", "Krile"].map(
  (name, index) => ({ id: `u${index}`, name })
);

const leader = { id: "u0", username: "alisaie", globalName: "Alisaie", avatar: "" };

const team = {
  id: 1,
  guildId: "g1",
  guildName: "Scions",
  name: "The Echo",
  timezone: "Asia/Tokyo",
  leaderId: "u0",
  isLeader: true,
  members
};

const schedule = {
  timezone: "Asia/Tokyo",
  cadenceDays: 7,
  weekdays: [3, 4, 5],
  startTime: "21:00",
  endTime: "23:00",
  publishLeadDays: 3,
  firstPeriodStart: "2099-10-19",
  nextPublishAt: "2099-10-16T02:00:00Z",
  channelId: "c1",
  channelName: "raid-schedule",
  enabled: true
};

function occurrence(id: number, date: string, available: number, status = "proposed") {
  const votes = members.map((member, index) => ({
    memberId: member.id,
    status: index < available ? "available" : index === available ? "unavailable" : "pending"
  }));
  return {
    id,
    startsAt: `${date}T12:00:00Z`,
    endsAt: `${date}T14:00:00Z`,
    status,
    available,
    unavailable: available < members.length ? 1 : 0,
    pending: Math.max(0, members.length - available - 1),
    votes
  };
}

export const polls = [
  {
    id: 10,
    periodStart: "2099-10-05",
    periodEnd: "2099-10-11",
    timezone: "Asia/Tokyo",
    occurrences: [
      occurrence(101, "2099-10-07", 8, "confirmed"),
      occurrence(102, "2099-10-08", 7),
      occurrence(103, "2099-10-09", 5)
    ]
  },
  {
    id: 11,
    periodStart: "2099-10-12",
    periodEnd: "2099-10-18",
    timezone: "Asia/Tokyo",
    occurrences: [occurrence(111, "2099-10-14", 2)]
  }
];

export type Recorder = { requests: string[]; bodies: Record<string, unknown> };

/** Serves the dashboard API for a signed-in leader and records every call. */
export async function mockSignedInLeader(page: Page): Promise<Recorder> {
  const recorder: Recorder = { requests: [], bodies: {} };
  const json = (route: Route, body: unknown, status = 200) =>
    route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    recorder.requests.push(`${request.method()} ${path}`);
    if (request.postData()) recorder.bodies[`${request.method()} ${path}`] = request.postDataJSON();
    switch (`${request.method()} ${path}`) {
      case "GET /api/config":
        return json(route, { inviteUrl: "https://discord.com/oauth2/authorize?client_id=1" });
      case "GET /api/auth/me":
        return json(route, leader);
      case "GET /api/teams":
        return json(route, [team]);
      case "GET /api/guilds":
        return json(route, [{ id: "g1", name: "Scions", icon: "" }]);
      case "GET /api/teams/1/schedule":
        return json(route, schedule);
      case "GET /api/teams/1/polls":
        return json(route, polls);
      case "POST /api/teams/1/schedule/preview":
        return json(route, {
          ...schedule,
          occurrences: [occurrence(0, "2099-10-21", 0), occurrence(0, "2099-10-22", 0)]
        });
      case "GET /api/guilds/g1/channels":
        return json(route, [
          { id: "c1", name: "raid-schedule" },
          { id: "c2", name: "general" }
        ]);
      case "POST /api/occurrences/102/status":
        return route.fulfill({ status: 204 });
      default:
        return json(route, { error: `unmocked ${request.method()} ${path}` }, 404);
    }
  });
  return recorder;
}
