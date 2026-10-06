import { useEffect, useState } from "react";
import { Plus } from "lucide-react";
import { api, ApiError, type Guild, type Team, type User } from "@/api";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ThemeSelect, Loading, ErrorBanner, EmptyState } from "@/components/common";
import { Landing } from "@/features/landing";
import { Overview } from "@/features/overview/overview";
import { SchedulePanel } from "@/features/schedule/schedule-panel";
import { TeamPanel } from "@/features/team/team-panel";
import { message } from "@/lib/format";
import { type Theme, type ThemePreference, applyTheme } from "@/lib/theme";

type Tab = "overview" | "schedule" | "team";

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [teams, setTeams] = useState<Team[]>([]);
  const [guilds, setGuilds] = useState<Guild[]>([]);
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

  useEffect(() => {
    const media = matchMedia("(prefers-color-scheme: dark)");
    const update = () =>
      applyTheme(themePreference === "system" ? (media.matches ? "dark" : "light") : themePreference);
    update();
    if (themePreference === "system") media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, [themePreference]);

  function chooseTheme(next: ThemePreference) {
    document.documentElement.dataset.themePreference = next;
    try {
      localStorage.setItem("raidy-theme", next);
    } catch {
      /* Theme still applies for this page. */
    }
    setThemePreference(next);
  }

  if (loading) return <Loading />;
  if (!user) return <Landing error={error} theme={themePreference} onThemeChange={chooseTheme} />;

  return (
    <div className="min-h-screen">
      <header className="border-b bg-card">
        <div className="mx-auto flex h-16 max-w-[1440px] items-center gap-4 px-4 sm:px-6">
          <div className="flex items-center gap-2 text-lg font-semibold tracking-tight">
            <span className="grid size-8 place-items-center rounded-lg bg-[#6957d8] text-sm font-bold text-white">
              R
            </span>
            Raidy
          </div>
          <Select
            value={teamID?.toString() ?? ""}
            onValueChange={(value) => selectTeam(Number(value))}
            disabled={teams.length === 0}
          >
            <SelectTrigger className="ml-auto h-11 w-44 sm:w-64" aria-label="Current team">
              <SelectValue placeholder="No teams yet" />
            </SelectTrigger>
            <SelectContent position="popper">
              {teams.map((team) => (
                <SelectItem key={team.id} value={team.id.toString()}>
                  {team.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <ThemeSelect value={themePreference} onChange={chooseTheme} />
          <Button
            variant="outline"
            className="hidden sm:inline-flex"
            onClick={() => {
              setCreating(true);
              setTab("team");
            }}
          >
            <Plus />
            New team
          </Button>
          <div className="hidden text-right sm:block">
            <div className="text-sm font-medium">{user.globalName || user.username}</div>
            <Button variant="link" size="xs" className="h-auto p-0 text-muted-foreground" onClick={logout}>
              Sign out
            </Button>
          </div>
        </div>
      </header>

      <div className="mx-auto grid max-w-[1440px] gap-6 px-4 py-6 sm:px-6 lg:grid-cols-[210px_minmax(0,1fr)]">
        <nav className="grid grid-cols-4 gap-1 lg:flex lg:flex-col" aria-label="Dashboard sections">
          {(["overview", "schedule", "team"] as Tab[]).map((value) => (
            <Button
              key={value}
              variant={tab === value && !creating ? "secondary" : "ghost"}
              className="h-11 justify-center px-2 text-xs whitespace-nowrap capitalize sm:text-sm lg:justify-start lg:px-4"
              onClick={() => {
                setCreating(false);
                setTab(value);
              }}
            >
              {value}
            </Button>
          ))}
          <Button
            variant={creating ? "secondary" : "ghost"}
            className="h-11 px-2 text-xs whitespace-nowrap sm:hidden"
            onClick={() => {
              setCreating(true);
              setTab("team");
            }}
          >
            New team
          </Button>
        </nav>

        <main className="min-w-0">
          {error && <ErrorBanner text={error} onClose={() => setError("")} />}
          {creating ? (
            <TeamPanel
              guilds={guilds}
              currentUser={user}
              onSaved={(id) => changed(id)}
              onCancel={() => setCreating(false)}
              onError={setError}
            />
          ) : !selected ? (
            <EmptyState
              onCreate={() => {
                setCreating(true);
                setTab("team");
              }}
            />
          ) : tab === "overview" ? (
            <Overview team={selected} revision={revision} onError={setError} />
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
      </div>
    </div>
  );
}
