/** Date helpers for the demo backend. Dates are "YYYY-MM-DD" in a team's timezone. */

export function localDate(at: Date, timezone: string) {
  return new Intl.DateTimeFormat("en-CA", { timeZone: timezone }).format(at);
}

export function addDays(date: string, days: number) {
  const value = new Date(`${date}T00:00:00Z`);
  value.setUTCDate(value.getUTCDate() + days);
  return value.toISOString().slice(0, 10);
}

export function weekday(date: string) {
  return new Date(`${date}T00:00:00Z`).getUTCDay();
}

function offsetMinutes(at: number, timezone: string) {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone: timezone,
    hourCycle: "h23",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).formatToParts(at);
  const part = (type: string) => Number(parts.find((value) => value.type === type)?.value);
  const wall = Date.UTC(part("year"), part("month") - 1, part("day"), part("hour"), part("minute"));
  return Math.round((wall - Math.floor(at / 60_000) * 60_000) / 60_000);
}

/** The instant a wall-clock time occurs on a local date. */
export function zoned(date: string, minutes: number, timezone: string) {
  const [year, month, day] = date.split("-").map(Number);
  const wall = Date.UTC(year, month - 1, day, 0, minutes);
  let instant = wall - offsetMinutes(wall, timezone) * 60_000;
  instant = wall - offsetMinutes(instant, timezone) * 60_000;
  return new Date(instant);
}

export function minutes(clock: string) {
  const [hours, mins] = clock.split(":").map(Number);
  return hours * 60 + mins;
}

/** A raid's start and end on a local date; an end at or before the start is the next day. */
export function raidTimes(date: string, startTime: string, endTime: string, timezone: string) {
  const start = zoned(date, minutes(startTime), timezone);
  let end = zoned(date, minutes(endTime), timezone);
  if (end <= start) end = zoned(addDays(date, 1), minutes(endTime), timezone);
  return { startsAt: start.toISOString(), endsAt: end.toISOString() };
}
