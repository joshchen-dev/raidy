import { useMemo, useState } from "react";
import { type Poll } from "@/api";
import { createState } from "@/demo/data";
import { addDays } from "@/demo/time";
import { AvailabilityGrid } from "@/features/overview/availability-grid";
import { DiscordPost } from "./discord-post";

/**
 * The grid and the Discord post side by side, both driven by the same sample
 * week the demo uses. The visitor plays the "You" row; every tap updates the
 * counts in both places, the way the real app edits the Discord message.
 */
export function HeroDemo() {
  const sample = useMemo(() => createState(), []);
  const team = sample.teams[0];
  const week = sample.polls[0];
  const you = sample.user.id;
  const [mine, setMine] = useState(
    () => new Set(week.occurrences.filter((night) => night.votes[you]).map((night) => night.id))
  );
  const [changed, setChanged] = useState<number | null>(null);

  const nights = week.occurrences.map((night) => {
    const votes = { ...night.votes, [you]: mine.has(night.id) };
    const available = team.memberIds.filter((id) => votes[id] === true).length;
    // Same rule as the server: a confirmed night needs everyone.
    const status =
      night.status === "confirmed" || night.status === "attention_required"
        ? available === team.memberIds.length
          ? "confirmed"
          : "attention_required"
        : night.status;
    return { ...night, votes, available, status };
  });

  const poll: Poll = {
    id: week.id,
    periodStart: week.periodStart,
    periodEnd: addDays(week.periodStart, week.cadenceDays - 1),
    timezone: team.timezone,
    occurrences: nights.map((night) => ({
      id: night.id,
      startsAt: night.startsAt,
      endsAt: night.endsAt,
      status: night.status,
      available: night.available,
      votes: team.memberIds.map((memberId) => ({
        memberId,
        status: memberId in night.votes ? (night.votes[memberId] ? "available" : "unavailable") : "pending"
      }))
    }))
  };

  function toggle(id: number) {
    setMine((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
    setChanged(id);
    window.setTimeout(() => setChanged((value) => (value === id ? null : value)), 900);
  }

  return (
    <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,0.85fr)]">
      <div>
        <AvailabilityGrid
          poll={poll}
          members={team.memberIds.map((id) => sample.people.find((person) => person.id === id)!)}
          canManage={false}
          busyID={null}
          onAction={() => {}}
          currentUserID={you}
          draft={mine}
          onToggle={toggle}
        />
        <p className="mt-2.5 text-sm text-muted-foreground">You're Alisaie. Tap a night.</p>
      </div>
      <DiscordPost
        team={team.name}
        period={`${poll.periodStart} – ${poll.periodEnd}`}
        timezone={team.timezone}
        nights={nights}
        total={team.memberIds.length}
        changed={changed}
      />
    </div>
  );
}
