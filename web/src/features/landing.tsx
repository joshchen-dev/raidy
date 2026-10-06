import { type Member, type Poll } from "@/api";
import { Wordmark } from "@/components/common";
import { Button } from "@/components/ui/button";
import { ThemeToggle } from "@/components/user-menu";
import { AvailabilityGrid } from "@/features/overview/availability-grid";
import { type ThemePreference } from "@/lib/theme";

const sampleMembers: Member[] = ["Alisaie", "Estinien", "G'raha", "Thancred", "Urianger", "Y'shtola"].map((name) => ({
  id: name,
  name
}));

const sampleVotes = [
  ["available", "available", "available"],
  ["available", "unavailable", "available"],
  ["available", "available", "pending"],
  ["available", "available", "available"],
  ["available", "available", "unavailable"],
  ["available", "pending", "available"]
] as const;

const samplePoll: Poll = {
  id: 0,
  periodStart: "2026-10-12",
  periodEnd: "2026-10-18",
  timezone: "Asia/Tokyo",
  occurrences: ["2026-10-14", "2026-10-15", "2026-10-16"].map((date, column) => {
    const votes = sampleMembers.map((member, row) => ({ memberId: member.id, status: sampleVotes[row][column] }));
    return {
      id: column + 1,
      startsAt: `${date}T12:00:00Z`,
      endsAt: `${date}T14:00:00Z`,
      status: column === 0 ? "confirmed" : "proposed",
      available: votes.filter((vote) => vote.status === "available").length,
      votes
    };
  })
};

export function Landing({
  error,
  theme,
  onThemeChange
}: {
  error: string;
  theme: ThemePreference;
  onThemeChange: (value: ThemePreference) => void;
}) {
  return (
    <div className="min-h-screen">
      <header className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4 sm:px-6">
        <Wordmark className="text-base" />
        <ThemeToggle value={theme} onChange={onThemeChange} />
      </header>
      <main className="mx-auto grid max-w-6xl items-center gap-12 px-4 py-12 sm:px-6 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] lg:py-24">
        <section>
          <h1 className="text-3xl font-semibold tracking-tight text-balance sm:text-4xl">
            See which nights your whole static can raid.
          </h1>
          <p className="mt-4 max-w-md text-muted-foreground">
            Members mark the nights they can make here, and Raidy keeps a live summary posted in your Discord server.
            The leader confirms dates in the same grid.
          </p>
          {error && <p className="mt-5 text-sm text-destructive">{error}</p>}
          <Button asChild size="lg" className="mt-8">
            <a href={loginURL()}>Sign in with Discord</a>
          </Button>
        </section>
        <figure aria-label="Example availability grid">
          <AvailabilityGrid
            poll={samplePoll}
            members={sampleMembers}
            canManage={false}
            busyID={null}
            onAction={() => {}}
          />
          <figcaption className="mt-3 text-xs text-muted-foreground">
            Example week. Wednesday works for everyone, so the leader confirmed it.
          </figcaption>
        </figure>
      </main>
    </div>
  );
}

/** Signing in from a team link in Discord returns to that team. */
function loginURL() {
  const team = new URLSearchParams(location.search).get("team");
  return team && /^\d+$/.test(team) ? `/api/auth/login?team=${team}` : "/api/auth/login";
}
