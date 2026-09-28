export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

export async function api<T>(path: string, options: RequestInit = {}): Promise<T | null> {
  const headers = new Headers(options.headers);
  if (options.body) headers.set("Content-Type", "application/json");
  let response: Response;
  try {
    response = await fetch(path, { ...options, headers, signal: options.signal ?? AbortSignal.timeout(15_000) });
  } catch (reason) {
    if (reason instanceof DOMException && (reason.name === "TimeoutError" || reason.name === "AbortError")) {
      throw new ApiError(408, "The server took too long to respond. Try again.");
    }
    throw reason;
  }
  if (response.status === 204) return null;
  const value = await response.json().catch(() => ({}));
  if (!response.ok) throw new ApiError(response.status, value.error || "Request failed");
  return value as T;
}

export type User = { id: string; username: string; globalName: string; avatar: string };
export type Guild = { id: string; name: string; icon: string };
export type Member = { id: string; name: string; avatar?: string };
export type Team = {
  id: number;
  guildId: string;
  guildName: string;
  name: string;
  timezone: string;
  leaderId: string;
  isLeader: boolean;
  members: Member[];
};
export type Channel = { id: string; name: string };
export type Occurrence = {
  id?: number;
  startsAt: string;
  endsAt: string;
  status: string;
  available?: number;
  unavailable?: number;
  pending?: number;
  votes?: { memberId: string; status: "available" | "unavailable" | "pending" }[];
};
export type Schedule = {
  timezone: string;
  cadenceDays: number;
  weekdays: number[];
  startTime: string;
  endTime: string;
  publishLeadDays: number;
  firstPeriodStart: string;
  nextPublishAt: string;
  channelId: string;
  channelName: string;
  enabled: boolean;
  occurrences?: Occurrence[];
};
export type Poll = {
  periodStart: string;
  periodEnd: string;
  timezone: string;
  occurrences: Occurrence[];
};
