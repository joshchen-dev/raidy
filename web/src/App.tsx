import { useEffect, useState } from "react";
import { Toaster } from "sonner";
import { api, ApiError, type Guild, type Team, type User } from "@/api";
import { ErrorBanner, Loading, Wordmark } from "@/components/common";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { UserMenu } from "@/components/user-menu";
import { Landing } from "@/features/landing";
import { Overview } from "@/features/overview/overview";
import { SchedulePanel } from "@/features/schedule/schedule-panel";
import { TeamPanel } from "@/features/team/team-panel";
import { message } from "@/lib/format";
import { applyTheme, type Theme, type ThemePreference } from "@/lib/theme";
import { cn } from "@/lib/utils";

type Tab = "overview" | "schedule" | "team";

const tabs: { value: Tab; label: string }[] = [
  { value: "overview", label: "Availability" },
  { value: "schedule", label: "Schedule" },
  { value: "team", label: "Roster" }
];

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [teams, setTeams] = useState<Team[]>([]);
  const [guilds, setGuilds] = useState<Guild[]>([]);
  const [inviteUrl, setInviteUrl] = useState("");
  const [teamID, setTeamID] = useState<number | null>(null);
  const [tab, setTab] = useState<Tab>("overview");
  const [creating, setCreating] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);
  const [themePreference, setThemePreference] = useState<ThemePreference>(() => {
    const saved = document.documentElement.dataset.themePreference;
    return saved === "light" || saved === "dark" ? saved : "system";
  });
  const [resolvedTheme, setResolvedTheme] = useState<Theme>("light");

  async function loadData(preferred?: number) {
    const [nextTeams, nextGuilds] = await Promise.all([api<Team[]>("/api/teams"), api<Guild[]>("/api/guilds")]);
    const values = nextTeams ?? [];
    setTeams(values);
    setGuilds(nextGuilds ?? []);
    const fromURL = Number(new URLSearchParams(location.search).get("team"));
    const wanted = preferred || teamID || fromURL;
    const selected = values.find((team) => team.id === wanted)?.id ?? values[0]?.id ?? null;
    setTeamID(selected);
    if (selected) history.replaceState(null, "", `/app?team=${selected}`);
  }

  useEffect(() => {
    api<{ inviteUrl: string }>("/api/config")
      .then((config) => setInviteUrl(config?.inviteUrl ?? ""))
      .catch(() => setInviteUrl(""));
    (async () => {
      try {
        const current = await api<User>("/api/auth/me");
        setUser(current);
        await loadData();
      } catch (reason) {
        if (!(reason instanceof ApiError) || reason.status !== 401) setError(message(reason));
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  useEffect(() => {
    const media = matchMedia("(prefers-color-scheme: dark)");
    const update = () => {
      const theme = themePreference === "system" ? (media.matches ? "dark" : "light") : themePreference;
      applyTheme(theme);
      setResolvedTheme(theme);
    };
    update();
    if (themePreference === "system") media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, [themePreference]);

  const selected = teams.find((team) => team.id === teamID) ?? null;

  function selectTeam(id: number) {
    setTeamID(id);
    setCreating(false);
    setTab("overview");
    history.replaceState(null, "", `/app?team=${id}`);
  }

  async function changed(id?: number) {
    await loadData(id);
    setRevision((value) => value + 1);
    setCreating(false);
  }

  async function logout() {
    await api("/api/auth/logout", { method: "POST" });
    location.assign("/");
  }

  function chooseTheme(next: ThemePreference) {
    document.documentElement.dataset.themePreference = next;
    try {
      localStorage.setItem("raidy-theme", next);
    } catch {
      /* Theme still applies for this page. */
    }
    setThemePreference(next);
  }

  const toaster = <Toaster theme={resolvedTheme} position="bottom-right" toastOptions={{ className: "font-sans" }} />;

  if (loading) return <Loading />;
  if (!user)
    return (
      <>
        <Landing error={error} theme={themePreference} onThemeChange={chooseTheme} />
        {toaster}
      </>
    );

  return (
    <div className="min-h-screen">
      <header className="border-b bg-background">
        <div className="mx-auto flex h-14 max-w-6xl items-center gap-2 px-4 sm:px-6">
          <Wordmark className="text-base" />
          {teams.length > 0 && !creating && (
            <>
              <span className="text-muted-foreground/60" aria-hidden="true">
                /
              </span>
              <Select value={teamID?.toString() ?? ""} onValueChange={(value) => selectTeam(Number(value))}>
                <SelectTrigger
                  className="h-8 max-w-56 border-0 px-2 font-medium shadow-none hover:bg-accent"
                  aria-label="Current team"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent position="popper" align="start">
                  {teams.map((team) => (
                    <SelectItem key={team.id} value={team.id.toString()}>
                      {team.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </>
          )}
          <div className="ml-auto">
            <UserMenu
              user={user}
              theme={themePreference}
              onThemeChange={chooseTheme}
              onNewTeam={() => setCreating(true)}
              onSignOut={logout}
            />
          </div>
        </div>
        {selected && !creating && (
          <nav className="mx-auto flex max-w-6xl gap-5 px-4 sm:px-6" aria-label="Team sections">
            {tabs.map(({ value, label }) => (
              <button
                key={value}
                type="button"
                aria-current={tab === value ? "page" : undefined}
                className={cn(
                  "-mb-px border-b-2 py-2.5 text-sm transition-colors",
                  tab === value
                    ? "border-primary font-medium text-foreground"
                    : "border-transparent text-muted-foreground hover:text-foreground"
                )}
                onClick={() => setTab(value)}
              >
                {label}
              </button>
            ))}
          </nav>
        )}
      </header>

      <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
        {error && <ErrorBanner text={error} onClose={() => setError("")} />}
        {creating || (!selected && guilds.length > 0) ? (
          <TeamPanel
            guilds={guilds}
            currentUser={user}
            onSaved={(id) => changed(id)}
            onCancel={teams.length > 0 ? () => setCreating(false) : undefined}
            onError={setError}
          />
        ) : !selected ? (
          <NoServers inviteUrl={inviteUrl} />
        ) : tab === "overview" ? (
          <Overview team={selected} revision={revision} onEditSchedule={() => setTab("schedule")} onError={setError} />
        ) : tab === "schedule" ? (
          <SchedulePanel
            team={selected}
            revision={revision}
            onChanged={() => changed(selected.id)}
            onError={setError}
          />
        ) : (
          <TeamPanel
            team={selected}
            guilds={guilds}
            currentUser={user}
            onSaved={() => changed(selected.id)}
            onDeleted={() => changed()}
            onError={setError}
          />
        )}
      </main>
      {toaster}
    </div>
  );
}

function NoServers({ inviteUrl }: { inviteUrl: string }) {
  return (
    <div className="max-w-lg">
      <h1 className="text-xl font-semibold tracking-tight">Raidy isn't in any of your servers</h1>
      <p className="mt-2 text-sm text-muted-foreground">
        Teams belong to a Discord server, and Raidy needs to be installed there before you can pick teammates or post
        timetables. Add it to a server you manage, then reload this page.
      </p>
      <div className="mt-6 flex gap-2">
        {inviteUrl && (
          <Button asChild>
            <a href={inviteUrl} target="_blank" rel="noreferrer">
              Add Raidy to a server
            </a>
          </Button>
        )}
        <Button variant="outline" onClick={() => location.reload()}>
          Reload
        </Button>
      </div>
    </div>
  );
}
