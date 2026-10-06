export function parseLocalDate(value: string): Date | undefined {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return undefined;
  return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
}

export function formatLocalDate(value: Date): string {
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, "0")}-${String(value.getDate()).padStart(2, "0")}`;
}

export function timezones(): string[] {
  const values = (Intl as typeof Intl & { supportedValuesOf?: (key: string) => string[] }).supportedValuesOf?.(
    "timeZone"
  );
  return (
    values ?? [
      "Asia/Tokyo",
      "UTC",
      "America/Los_Angeles",
      "America/New_York",
      "Europe/London",
      "Europe/Paris",
      "Asia/Seoul",
      "Asia/Singapore",
      "Australia/Sydney"
    ]
  );
}

export function browserTimezone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Tokyo";
}

export function formatDateTime(value: string, timezone: string) {
  return new Intl.DateTimeFormat(undefined, {
    timeZone: timezone,
    weekday: "short",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23"
  }).format(new Date(value));
}

export function formatTime(value: string, timezone: string) {
  return new Intl.DateTimeFormat(undefined, {
    timeZone: timezone,
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23"
  }).format(new Date(value));
}

export function message(reason: unknown) {
  return reason instanceof Error ? reason.message : "Something went wrong";
}

const weekdayNames = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

/** "Wed 14" in the team's timezone. */
export function formatDay(value: string, timezone: string) {
  const parts = new Intl.DateTimeFormat(undefined, {
    timeZone: timezone,
    weekday: "short",
    day: "numeric"
  }).formatToParts(new Date(value));
  const part = (type: string) => parts.find((value) => value.type === type)?.value ?? "";
  return `${part("weekday")} ${part("day")}`;
}

/** "Oct 12 – 18" for an inclusive YYYY-MM-DD range. */
export function formatPeriod(start: string, end: string) {
  const from = parseLocalDate(start);
  const to = parseLocalDate(end);
  if (!from || !to) return `${start} – ${end}`;
  return new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric" }).formatRange(from, to);
}

/** "Weekly · Wed, Thu, Fri · 21:00–23:00" */
export function describeSchedule(schedule: {
  cadenceDays: number;
  weekdays: number[];
  startTime: string;
  endTime: string;
}) {
  const days = [...schedule.weekdays].sort().map((day) => weekdayNames[day]);
  return `${schedule.cadenceDays === 7 ? "Weekly" : "Every two weeks"} · ${days.join(", ")} · ${schedule.startTime}–${schedule.endTime}`;
}
