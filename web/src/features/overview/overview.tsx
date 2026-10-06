import { useEffect, useState } from "react";
import { api, type Member, type Occurrence, type Poll, type Schedule, type Team } from "@/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SectionTitle, LoadingPanel, Status } from "@/components/common";
import { formatDateTime, message } from "@/lib/format";

export function Overview({
  team,
  revision,
  onError
}: {
  team: Team;
  revision: number;
  onError: (value: string) => void;
}) {
  const [schedule, setSchedule] = useState<Schedule | null>(null);
  const [polls, setPolls] = useState<Poll[]>([]);
  const [pollID, setPollID] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    Promise.all([api<Schedule>(`/api/teams/${team.id}/schedule`), api<Poll[]>(`/api/teams/${team.id}/polls`)])
      .then(([nextSchedule, nextPolls]) => {
        const open = nextPolls ?? [];
        setSchedule(nextSchedule);
        setPolls(open);
        setPollID((current) => (open.some((value) => value.id === current) ? current : (open[0]?.id ?? null)));
      })
      .catch((reason) => onError(message(reason)))
      .finally(() => setLoading(false));
  }, [team.id, revision]);

  const poll = polls.find((value) => value.id === pollID) ?? null;

  if (loading) return <LoadingPanel />;
  return (
    <>
      <SectionTitle title={team.name} detail={`${team.members.length} members · ${team.timezone}`} />
      <div className="mb-6 grid gap-4 sm:grid-cols-3">
        <Metric
          label="Automation"
          value={!schedule ? "Not configured" : schedule.enabled ? "Enabled" : "Paused"}
          accent={schedule?.enabled}
        />
        <Metric
          label="Next publication"
          value={schedule ? formatDateTime(schedule.nextPublishAt, schedule.timezone) : "—"}
        />
        <Metric label="Cadence" value={schedule ? (schedule.cadenceDays === 7 ? "Weekly" : "Biweekly") : "—"} />
      </div>
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-end justify-between gap-3">
          <div>
            <CardTitle>Current timetable</CardTitle>
            <CardDescription className="mt-1">
              Voting happens in Discord; current responses are shown here.
            </CardDescription>
          </div>
          {poll &&
            (polls.length > 1 ? (
              <Select value={poll.id.toString()} onValueChange={(value) => setPollID(Number(value))}>
                <SelectTrigger className="h-9 w-56" aria-label="Voting period">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent position="popper">
                  {polls.map((value) => (
                    <SelectItem key={value.id} value={value.id.toString()}>
                      {value.periodStart} — {value.periodEnd}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : (
              <span className="text-sm text-muted-foreground">
                {poll.periodStart} — {poll.periodEnd}
              </span>
            ))}
        </CardHeader>
        <CardContent>
          {!poll ? (
            <p className="py-8 text-center text-sm text-muted-foreground">No active timetable has been published.</p>
          ) : (
            <div className="divide-y">
              {poll.occurrences.map((occurrence) => (
                <div key={occurrence.id} className="grid gap-3 py-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
                  <div>
                    <div className="font-medium">{formatDateTime(occurrence.startsAt, poll.timezone)}</div>
                    <Status status={occurrence.status} />
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <AttendanceCount
                      label="Available"
                      status="available"
                      value={occurrence.available}
                      votes={occurrence.votes}
                      members={team.members}
                    />
                    <AttendanceCount
                      label="Unavailable"
                      status="unavailable"
                      value={occurrence.unavailable}
                      votes={occurrence.votes}
                      members={team.members}
                    />
                    <AttendanceCount
                      label="Pending"
                      status="pending"
                      value={occurrence.pending}
                      votes={occurrence.votes}
                      members={team.members}
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </>
  );
}

export function Metric({ label, value, accent }: { label: string; value: string; accent?: boolean }) {
  return (
    <Card>
      <CardContent>
        <div className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{label}</div>
        <div className={`mt-2 font-semibold ${accent ? "text-primary" : ""}`}>{value}</div>
      </CardContent>
    </Card>
  );
}

export function AttendanceCount({
  label,
  status,
  value = 0,
  votes = [],
  members
}: {
  label: string;
  status: "available" | "unavailable" | "pending";
  value?: number;
  votes?: Occurrence["votes"];
  members: Member[];
}) {
  const names = votes
    .filter((vote) => vote.status === status)
    .map((vote) => members.find((member) => member.id === vote.memberId)?.name ?? "Member");
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button type="button" variant="outline" size="sm">
          <strong>{value}</strong>
          <span className="text-muted-foreground">{label}</span>
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-60" align="end">
        <div className="font-medium">{label}</div>
        {names.length ? (
          <ul className="mt-2 space-y-1 text-sm">
            {names.map((name, index) => (
              <li key={`${name}-${index}`}>{name}</li>
            ))}
          </ul>
        ) : (
          <p className="mt-2 text-sm text-muted-foreground">No members</p>
        )}
      </PopoverContent>
    </Popover>
  );
}
