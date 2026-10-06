import { useEffect, useState } from "react";
import { toast } from "sonner";
import { api, type Occurrence, type Poll, type Schedule, type Team } from "@/api";
import { LoadingPanel, PageHeader } from "@/components/common";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { AvailabilityGrid, type OccurrenceAction } from "@/features/overview/availability-grid";
import { describeSchedule, formatDateTime, formatDay, formatPeriod, message } from "@/lib/format";

const actionMessages: Record<OccurrenceAction, string> = {
  confirm: "confirmed",
  cancel: "cancelled",
  reopen: "reopened"
};

export function Overview({
  team,
  currentUserID,
  revision,
  onEditSchedule,
  onError
}: {
  team: Team;
  currentUserID: string;
  revision: number;
  onEditSchedule: () => void;
  onError: (value: string) => void;
}) {
  const [schedule, setSchedule] = useState<Schedule | null>(null);
  const [polls, setPolls] = useState<Poll[]>([]);
  const [pollID, setPollID] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [reload, setReload] = useState(0);
  const [busyID, setBusyID] = useState<number | null>(null);
  const [draft, setDraft] = useState<Set<number> | null>(null);
  const [savingVotes, setSavingVotes] = useState(false);

  useEffect(() => {
    let active = true;
    if (reload === 0) setLoading(true);
    Promise.all([api<Schedule>(`/api/teams/${team.id}/schedule`), api<Poll[]>(`/api/teams/${team.id}/polls`)])
      .then(([nextSchedule, nextPolls]) => {
        if (!active) return;
        const open = nextPolls ?? [];
        setSchedule(nextSchedule);
        setPolls(open);
        setPollID((current) => (open.some((value) => value.id === current) ? current : (open[0]?.id ?? null)));
      })
      .catch((reason) => active && onError(message(reason)))
      .finally(() => active && setLoading(false));
    return () => {
      active = false;
    };
  }, [team.id, revision, reload]);

  async function changeOccurrence(occurrence: Occurrence, action: OccurrenceAction) {
    if (!occurrence.id || !poll) return;
    setBusyID(occurrence.id);
    try {
      await api(`/api/occurrences/${occurrence.id}/status`, { method: "POST", body: JSON.stringify({ action }) });
      toast.success(`${formatDay(occurrence.startsAt, poll.timezone)} ${actionMessages[action]}`);
      setReload((value) => value + 1);
    } catch (reason) {
      toast.error(message(reason));
    } finally {
      setBusyID(null);
    }
  }

  function startEditing(current: Poll) {
    const mine = current.occurrences.filter(
      (occurrence) =>
        occurrence.id &&
        occurrence.votes?.some((vote) => vote.memberId === currentUserID && vote.status === "available")
    );
    setDraft(new Set(mine.map((occurrence) => occurrence.id!)));
  }

  function toggle(occurrenceID: number) {
    setDraft((current) => {
      const next = new Set(current);
      if (next.has(occurrenceID)) next.delete(occurrenceID);
      else next.add(occurrenceID);
      return next;
    });
  }

  async function saveVotes(current: Poll) {
    if (!draft) return;
    setSavingVotes(true);
    try {
      await api(`/api/polls/${current.id}/availability`, {
        method: "PUT",
        body: JSON.stringify({ available: [...draft] })
      });
      toast.success("Availability saved");
      setDraft(null);
      setReload((value) => value + 1);
    } catch (reason) {
      toast.error(message(reason));
    } finally {
      setSavingVotes(false);
    }
  }

  if (loading) return <LoadingPanel />;
  const poll = polls.find((value) => value.id === pollID) ?? null;
  const canVote =
    poll !== null &&
    poll.occurrences.some((occurrence) => occurrence.votes?.some((vote) => vote.memberId === currentUserID)) &&
    poll.occurrences.some((occurrence) => new Date(occurrence.startsAt) > new Date());

  if (!schedule) {
    return (
      <div className="max-w-lg">
        <h1 className="text-xl font-semibold tracking-tight">No timetable yet</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          {team.isLeader
            ? "Choose the days and time your static raids. Raidy then opens voting and announces the dates in Discord before every period."
            : "Your leader hasn't set up the recurring schedule yet."}
        </p>
        {team.isLeader && (
          <Button className="mt-6" onClick={onEditSchedule}>
            Set up schedule
          </Button>
        )}
      </div>
    );
  }

  const summary = `${describeSchedule(schedule)} · ${schedule.timezone}`;
  const nextPost = schedule.enabled
    ? `Next timetable posts ${formatDateTime(schedule.nextPublishAt, schedule.timezone)}`
    : "Automatic posting is paused";

  if (!poll) {
    return (
      <>
        <PageHeader title="No open voting period" detail={summary} />
        <p className="text-sm text-muted-foreground">{nextPost}.</p>
      </>
    );
  }

  return (
    <>
      <PageHeader
        title={formatPeriod(poll.periodStart, poll.periodEnd)}
        detail={
          <>
            {summary}
            <span className="mx-1.5 text-muted-foreground/50">·</span>
            {nextPost}
          </>
        }
        actions={
          polls.length > 1 && (
            <Select
              value={poll.id.toString()}
              onValueChange={(value) => {
                setPollID(Number(value));
                setDraft(null);
              }}
            >
              <SelectTrigger className="h-8 w-44" aria-label="Voting period">
                <SelectValue />
              </SelectTrigger>
              <SelectContent position="popper" align="end">
                {polls.map((value) => (
                  <SelectItem key={value.id} value={value.id.toString()}>
                    {formatPeriod(value.periodStart, value.periodEnd)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )
        }
      />
      {canVote && (
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
          <p className="text-sm text-muted-foreground">
            {draft ? "Tap each date you can make, then save." : "Your answers are in the first row."}
          </p>
          {draft ? (
            <div className="flex gap-2">
              <Button variant="ghost" size="sm" disabled={savingVotes} onClick={() => setDraft(null)}>
                Cancel
              </Button>
              <Button size="sm" disabled={savingVotes} onClick={() => saveVotes(poll)}>
                {savingVotes ? "Saving…" : "Save availability"}
              </Button>
            </div>
          ) : (
            <Button variant="outline" size="sm" onClick={() => startEditing(poll)}>
              Edit my availability
            </Button>
          )}
        </div>
      )}
      <AvailabilityGrid
        poll={poll}
        members={team.members}
        canManage={team.isLeader && !draft}
        busyID={busyID}
        onAction={changeOccurrence}
        currentUserID={currentUserID}
        draft={draft}
        onToggle={toggle}
      />
      {team.isLeader && !draft && (
        <p className="mt-3 text-xs text-muted-foreground">Use ⋯ under a date to confirm, cancel, or reopen it.</p>
      )}
    </>
  );
}
