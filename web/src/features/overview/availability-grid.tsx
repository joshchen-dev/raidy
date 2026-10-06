import { Check, Ellipsis, Minus, X } from "lucide-react";
import { type Member, type Occurrence, type Poll } from "@/api";
import { Avatar, StatusChip } from "@/components/common";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { browserTimezone, formatDay, formatTime, teamTime } from "@/lib/format";
import { cn } from "@/lib/utils";

export type OccurrenceAction = "confirm" | "cancel" | "reopen";
type Vote = "available" | "unavailable" | "pending";

const voteLabels: Record<Vote, string> = { available: "Available", unavailable: "Unavailable", pending: "No answer" };

function VoteCell({ vote, name }: { vote: Vote; name: string }) {
  return (
    <span
      role="img"
      aria-label={`${name}: ${voteLabels[vote]}`}
      title={voteLabels[vote]}
      className={cn(
        "inline-grid size-7 place-items-center rounded",
        vote === "available" && "bg-available-soft text-available",
        vote === "unavailable" && "bg-unavailable-soft text-unavailable",
        vote === "pending" && "text-muted-foreground/70"
      )}
    >
      {vote === "available" ? (
        <Check className="size-4" strokeWidth={2.5} />
      ) : vote === "unavailable" ? (
        <X className="size-4" strokeWidth={2.5} />
      ) : (
        <Minus className="size-4" />
      )}
    </span>
  );
}

function ActionsMenu({
  occurrence,
  label,
  busy,
  onAction
}: {
  occurrence: Occurrence;
  label: string;
  busy: boolean;
  onAction: (action: OccurrenceAction) => void;
}) {
  const options: { action: OccurrenceAction; label: string; show: boolean }[] = [
    { action: "confirm", label: "Confirm date", show: occurrence.status !== "confirmed" },
    { action: "cancel", label: "Cancel date", show: occurrence.status !== "cancelled" },
    { action: "reopen", label: "Reopen", show: occurrence.status !== "proposed" }
  ];
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon-xs" aria-label={`Actions for ${label}`} disabled={busy}>
          <Ellipsis />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="center" className="w-40 p-1">
        {options
          .filter((option) => option.show)
          .map((option) => (
            <Button
              key={option.action}
              variant="ghost"
              size="sm"
              className={cn("w-full justify-start", option.action === "cancel" && "text-destructive")}
              onClick={() => onAction(option.action)}
            >
              {option.label}
            </Button>
          ))}
      </PopoverContent>
    </Popover>
  );
}

function TeamTime({
  value,
  teamTimezone,
  viewerTimezone
}: {
  value: string;
  teamTimezone: string;
  viewerTimezone: string;
}) {
  const label = teamTime(value, teamTimezone, viewerTimezone);
  if (!label) return null;
  return (
    <div
      className="mt-0.5 text-[11px] whitespace-nowrap text-muted-foreground/80"
      title={`Team time (${teamTimezone})`}
    >
      {label}
    </div>
  );
}

export function AvailabilityGrid({
  poll,
  members,
  canManage,
  busyID,
  now = new Date(),
  onAction,
  currentUserID,
  draft,
  onToggle,
  viewerTimezone = browserTimezone()
}: {
  poll: Poll;
  members: Member[];
  canManage: boolean;
  busyID: number | null;
  now?: Date;
  onAction: (occurrence: Occurrence, action: OccurrenceAction) => void;
  currentUserID?: string;
  /** Dates the current user marks available while editing; null when not editing. */
  draft?: Set<number> | null;
  onToggle?: (occurrenceID: number) => void;
  /** Times show in the viewer's timezone, with the team's time beneath when it differs. */
  viewerTimezone?: string;
}) {
  const occurrences = poll.occurrences;
  const memberIDs = Array.from(
    new Set(occurrences.flatMap((occurrence) => occurrence.votes ?? []).map((v) => v.memberId))
  );
  const name = (id: string) => members.find((member) => member.id === id)?.name ?? "Former member";
  const rows = memberIDs
    .map((id) => members.find((member) => member.id === id) ?? { id, name: name(id) })
    .sort((a, b) => (a.id === currentUserID ? -1 : b.id === currentUserID ? 1 : a.name.localeCompare(b.name)));
  const vote = (occurrence: Occurrence, id: string): Vote =>
    occurrence.votes?.find((value) => value.memberId === id)?.status ?? "pending";

  return (
    <div className="overflow-x-auto rounded-md border bg-card">
      <table className="w-full border-collapse text-sm">
        <caption className="sr-only">Availability by member and raid date</caption>
        <thead>
          <tr className="border-b">
            <th
              scope="col"
              className="sticky left-0 z-10 bg-card px-4 py-3 text-left font-normal text-muted-foreground"
            >
              Member
            </th>
            {occurrences.map((occurrence) => (
              <th
                key={occurrence.id}
                scope="col"
                className="min-w-20 px-2 py-3 text-center align-top font-normal sm:min-w-24"
              >
                <div className="font-medium">{formatDay(occurrence.startsAt, viewerTimezone)}</div>
                <div className="font-mono text-xs text-muted-foreground">
                  {formatTime(occurrence.startsAt, viewerTimezone)}
                </div>
                <TeamTime value={occurrence.startsAt} teamTimezone={poll.timezone} viewerTimezone={viewerTimezone} />
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((member) => (
            <tr key={member.id} className="border-b last:border-b-0">
              <th scope="row" className="sticky left-0 z-10 bg-card px-3 py-2 text-left font-normal sm:px-4">
                <span className="flex items-center gap-2.5">
                  <Avatar member={member} />
                  <span className="max-w-24 truncate sm:max-w-none">{member.name}</span>
                  {member.id === currentUserID && <span className="text-xs text-muted-foreground">You</span>}
                </span>
              </th>
              {occurrences.map((occurrence) => (
                <td key={occurrence.id} className="px-2 py-2 text-center">
                  {draft && member.id === currentUserID && occurrence.id && new Date(occurrence.startsAt) > now ? (
                    <button
                      type="button"
                      aria-pressed={draft.has(occurrence.id)}
                      aria-label={`Available on ${formatDay(occurrence.startsAt, viewerTimezone)}`}
                      className="rounded ring-2 ring-primary/40 focus-visible:ring-primary"
                      onClick={() => onToggle?.(occurrence.id!)}
                    >
                      <VoteCell vote={draft.has(occurrence.id) ? "available" : "unavailable"} name={member.name} />
                    </button>
                  ) : (
                    <VoteCell vote={vote(occurrence, member.id)} name={member.name} />
                  )}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
        <tfoot className="border-t bg-muted">
          <tr>
            <th
              scope="row"
              className="sticky left-0 z-10 bg-muted px-4 py-3 text-left font-normal text-muted-foreground"
            >
              Available
            </th>
            {occurrences.map((occurrence) => {
              const total = rows.length;
              const available = occurrence.available ?? 0;
              const everyone = total > 0 && available === total;
              return (
                <td key={occurrence.id} className="px-2 py-3 text-center">
                  <div
                    className={cn("font-mono text-sm tabular-nums", everyone ? "font-semibold text-available" : "")}
                    data-testid="available-count"
                  >
                    {available}/{total}
                    {everyone && <span className="sr-only"> — everyone is available</span>}
                  </div>
                  <div className="mt-1.5 flex items-center justify-center gap-0.5">
                    <StatusChip status={occurrence.status} />
                    {canManage && occurrence.id && new Date(occurrence.startsAt) > now && (
                      <ActionsMenu
                        occurrence={occurrence}
                        label={formatDay(occurrence.startsAt, viewerTimezone)}
                        busy={busyID === occurrence.id}
                        onAction={(action) => onAction(occurrence, action)}
                      />
                    )}
                  </div>
                </td>
              );
            })}
          </tr>
        </tfoot>
      </table>
    </div>
  );
}
