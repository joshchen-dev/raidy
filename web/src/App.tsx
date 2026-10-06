import { useEffect, useMemo, useState } from "react";
import { CalendarDays, ChevronsUpDown, Plus, Trash2, X } from "lucide-react";
import { api, ApiError, type Channel, type Guild, type Member, type Occurrence, type Poll, type Schedule, type Team, type User } from "./api";
import { Alert, AlertAction, AlertDescription } from "@/components/ui/alert";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger
} from "@/components/ui/alert-dialog";
import { Avatar as UserAvatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

type Tab = "overview" | "schedule" | "team";
type Theme = "light" | "dark";
type ThemePreference = Theme | "system";

const weekdays = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const leadOptions = [1, 2, 3, 5, 7, 10, 14];
const timeOptions = Array.from({ length: 96 }, (_, index) => {
  const hours = String(Math.floor(index / 4)).padStart(2, "0");
  const minutes = String((index % 4) * 15).padStart(2, "0");
  return `${hours}:${minutes}`;
});

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
    const [nextTeams, nextGuilds] = await Promise.all([
      api<Team[]>("/api/teams"),
      api<Guild[]>("/api/guilds")
    ]);
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
    const update = () => applyTheme(themePreference === "system" ? (media.matches ? "dark" : "light") : themePreference);
    update();
    if (themePreference === "system") media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, [themePreference]);

  function chooseTheme(next: ThemePreference) {
    document.documentElement.dataset.themePreference = next;
    try { localStorage.setItem("raidy-theme", next); } catch { /* Theme still applies for this page. */ }
    setThemePreference(next);
  }

  if (loading) return <Loading />;
  if (!user) return <Landing error={error} theme={themePreference} onThemeChange={chooseTheme} />;

  return (
    <div className="min-h-screen">
      <header className="border-b bg-card">
        <div className="mx-auto flex h-16 max-w-[1440px] items-center gap-4 px-4 sm:px-6">
          <div className="flex items-center gap-2 text-lg font-semibold tracking-tight">
            <span className="grid size-8 place-items-center rounded-lg bg-[#6957d8] text-sm font-bold text-white">R</span>
            Raidy
          </div>
          <Select value={teamID?.toString() ?? ""} onValueChange={(value) => selectTeam(Number(value))} disabled={teams.length === 0}>
            <SelectTrigger className="ml-auto h-11 w-44 sm:w-64" aria-label="Current team"><SelectValue placeholder="No teams yet" /></SelectTrigger>
            <SelectContent position="popper">{teams.map((team) => <SelectItem key={team.id} value={team.id.toString()}>{team.name}</SelectItem>)}</SelectContent>
          </Select>
          <ThemeSelect value={themePreference} onChange={chooseTheme} />
          <Button variant="outline" className="hidden sm:inline-flex" onClick={() => { setCreating(true); setTab("team"); }}><Plus />New team</Button>
          <div className="hidden text-right sm:block">
            <div className="text-sm font-medium">{user.globalName || user.username}</div>
            <Button variant="link" size="xs" className="h-auto p-0 text-muted-foreground" onClick={logout}>Sign out</Button>
          </div>
        </div>
      </header>

      <div className="mx-auto grid max-w-[1440px] gap-6 px-4 py-6 sm:px-6 lg:grid-cols-[210px_minmax(0,1fr)]">
        <nav className="grid grid-cols-4 gap-1 lg:flex lg:flex-col" aria-label="Dashboard sections">
          {(["overview", "schedule", "team"] as Tab[]).map((value) => (
            <Button key={value} variant={tab === value && !creating ? "secondary" : "ghost"} className="h-11 justify-center px-2 text-xs whitespace-nowrap capitalize sm:text-sm lg:justify-start lg:px-4" onClick={() => { setCreating(false); setTab(value); }}>
              {value}
            </Button>
          ))}
          <Button variant={creating ? "secondary" : "ghost"} className="h-11 px-2 text-xs whitespace-nowrap sm:hidden" onClick={() => { setCreating(true); setTab("team"); }}>New team</Button>
        </nav>

        <main className="min-w-0">
          {error && <ErrorBanner text={error} onClose={() => setError("")} />}
          {creating ? (
            <TeamPanel guilds={guilds} currentUser={user} onSaved={(id) => changed(id)} onCancel={() => setCreating(false)} onError={setError} />
          ) : !selected ? (
            <EmptyState onCreate={() => { setCreating(true); setTab("team"); }} />
          ) : tab === "overview" ? (
            <Overview team={selected} revision={revision} onError={setError} />
          ) : tab === "schedule" ? (
            <SchedulePanel team={selected} revision={revision} onChanged={() => changed(selected.id)} onError={setError} />
          ) : (
            <TeamPanel team={selected} guilds={guilds} currentUser={user} onSaved={() => changed(selected.id)} onDeleted={() => changed()} onError={setError} />
          )}
        </main>
      </div>
    </div>
  );
}

function Landing({ error, theme, onThemeChange }: { error: string; theme: ThemePreference; onThemeChange: (value: ThemePreference) => void }) {
  return (
    <main className="relative grid min-h-screen place-items-center px-5">
      <div className="absolute right-4 top-4"><ThemeSelect value={theme} onChange={onThemeChange} /></div>
      <section className="w-full max-w-md text-center">
        <div className="mx-auto mb-6 grid size-14 place-items-center rounded-xl bg-[#6957d8] text-2xl font-bold text-white">R</div>
        <p className="mb-3 text-sm font-semibold uppercase tracking-[.18em] text-primary">Discord raid scheduling</p>
        <h1 className="text-4xl font-semibold tracking-tight">Plan the raid. Keep the party together.</h1>
        <p className="mx-auto mt-4 max-w-sm text-muted-foreground">Configure your static team and recurring timetable here. Availability voting stays where your group already is—Discord.</p>
        {error && <p className="mt-5 text-sm text-destructive">{error}</p>}
        <Button asChild size="lg" className="mt-8 w-full"><a href="/api/auth/login">Continue with Discord</a></Button>
      </section>
    </main>
  );
}

function ThemeSelect({ value, onChange }: { value: ThemePreference; onChange: (value: ThemePreference) => void }) {
  return (
    <Select value={value} onValueChange={(next) => onChange(next as ThemePreference)}>
      <SelectTrigger className="h-11 w-28" aria-label="Color theme"><SelectValue /></SelectTrigger>
      <SelectContent position="popper"><SelectItem value="system">System</SelectItem><SelectItem value="light">Light</SelectItem><SelectItem value="dark">Dark</SelectItem></SelectContent>
    </Select>
  );
}

function applyTheme(theme: Theme) {
  document.documentElement.classList.toggle("dark", theme === "dark");
  document.documentElement.dataset.theme = theme;
  document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')?.setAttribute("content", theme === "dark" ? "#0f1117" : "#f5f6f8");
}

function Loading() {
  return <div className="grid min-h-screen place-items-center text-sm text-muted-foreground">Loading Raidy…</div>;
}

function ErrorBanner({ text, onClose }: { text: string; onClose: () => void }) {
  return (
    <Alert variant="destructive" className="mb-5"><AlertDescription>{text}</AlertDescription><AlertAction><Button variant="ghost" size="icon-sm" aria-label="Dismiss error" onClick={onClose}><X /></Button></AlertAction></Alert>
  );
}

function EmptyState({ onCreate }: { onCreate: () => void }) {
  return (
    <Card className="py-12 text-center"><CardContent>
      <h1 className="text-2xl font-semibold">Create your first team</h1>
      <p className="mx-auto mt-2 max-w-md text-muted-foreground">Choose a Discord server, add up to seven teammates, then configure your recurring timetable.</p>
      <Button className="mt-6" onClick={onCreate}><Plus />Create team</Button>
    </CardContent></Card>
  );
}

function SectionTitle({ title, detail }: { title: string; detail: string }) {
  return <div className="mb-6"><h1 className="text-2xl font-semibold tracking-tight">{title}</h1><p className="mt-1 text-sm text-muted-foreground">{detail}</p></div>;
}

function Overview({ team, revision, onError }: { team: Team; revision: number; onError: (value: string) => void }) {
  const [schedule, setSchedule] = useState<Schedule | null>(null);
  const [polls, setPolls] = useState<Poll[]>([]);
  const [pollID, setPollID] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    Promise.all([api<Schedule>(`/api/teams/${team.id}/schedule`), api<Poll[]>(`/api/teams/${team.id}/polls`)]).then(([nextSchedule, nextPolls]) => {
      const open = nextPolls ?? [];
      setSchedule(nextSchedule); setPolls(open);
      setPollID((current) => open.some((value) => value.id === current) ? current : open[0]?.id ?? null);
    }).catch((reason) => onError(message(reason))).finally(() => setLoading(false));
  }, [team.id, revision]);

  const poll = polls.find((value) => value.id === pollID) ?? null;

  if (loading) return <LoadingPanel />;
  return (
    <>
      <SectionTitle title={team.name} detail={`${team.members.length} members · ${team.timezone}`} />
      <div className="mb-6 grid gap-4 sm:grid-cols-3">
        <Metric label="Automation" value={!schedule ? "Not configured" : schedule.enabled ? "Enabled" : "Paused"} accent={schedule?.enabled} />
        <Metric label="Next publication" value={schedule ? formatDateTime(schedule.nextPublishAt, schedule.timezone) : "—"} />
        <Metric label="Cadence" value={schedule ? (schedule.cadenceDays === 7 ? "Weekly" : "Biweekly") : "—"} />
      </div>
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-end justify-between gap-3">
          <div><CardTitle>Current timetable</CardTitle><CardDescription className="mt-1">Voting happens in Discord; current responses are shown here.</CardDescription></div>
          {poll && (polls.length > 1 ? (
            <Select value={poll.id.toString()} onValueChange={(value) => setPollID(Number(value))}>
              <SelectTrigger className="h-9 w-56" aria-label="Voting period"><SelectValue /></SelectTrigger>
              <SelectContent position="popper">{polls.map((value) => <SelectItem key={value.id} value={value.id.toString()}>{value.periodStart} — {value.periodEnd}</SelectItem>)}</SelectContent>
            </Select>
          ) : <span className="text-sm text-muted-foreground">{poll.periodStart} — {poll.periodEnd}</span>)}
        </CardHeader>
        <CardContent>
        {!poll ? <p className="py-8 text-center text-sm text-muted-foreground">No active timetable has been published.</p> : (
          <div className="divide-y">
            {poll.occurrences.map((occurrence) => (
              <div key={occurrence.id} className="grid gap-3 py-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
                <div><div className="font-medium">{formatDateTime(occurrence.startsAt, poll.timezone)}</div><Status status={occurrence.status} /></div>
                <div className="flex flex-wrap gap-2"><AttendanceCount label="Available" status="available" value={occurrence.available} votes={occurrence.votes} members={team.members} /><AttendanceCount label="Unavailable" status="unavailable" value={occurrence.unavailable} votes={occurrence.votes} members={team.members} /><AttendanceCount label="Pending" status="pending" value={occurrence.pending} votes={occurrence.votes} members={team.members} /></div>
              </div>
            ))}
          </div>
        )}
        </CardContent>
      </Card>
    </>
  );
}

function Metric({ label, value, accent }: { label: string; value: string; accent?: boolean }) {
  return <Card><CardContent><div className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{label}</div><div className={`mt-2 font-semibold ${accent ? "text-primary" : ""}`}>{value}</div></CardContent></Card>;
}

function AttendanceCount({ label, status, value = 0, votes = [], members }: { label: string; status: "available" | "unavailable" | "pending"; value?: number; votes?: Occurrence["votes"]; members: Member[] }) {
  const names = votes.filter((vote) => vote.status === status).map((vote) => members.find((member) => member.id === vote.memberId)?.name ?? "Member");
  return <Popover><PopoverTrigger asChild><Button type="button" variant="outline" size="sm"><strong>{value}</strong><span className="text-muted-foreground">{label}</span></Button></PopoverTrigger><PopoverContent className="w-60" align="end"><div className="font-medium">{label}</div>{names.length ? <ul className="mt-2 space-y-1 text-sm">{names.map((name, index) => <li key={`${name}-${index}`}>{name}</li>)}</ul> : <p className="mt-2 text-sm text-muted-foreground">No members</p>}</PopoverContent></Popover>;
}

function Status({ status }: { status: string }) {
  const label = status.replaceAll("_", " ");
  return <Badge variant={status === "attention_required" ? "destructive" : "secondary"} className="mt-1 capitalize">{label}</Badge>;
}

function TeamPanel({ team, guilds, currentUser, onSaved, onDeleted, onCancel, onError }: { team?: Team; guilds: Guild[]; currentUser: User; onSaved: (id: number) => void | Promise<void>; onDeleted?: () => void | Promise<void>; onCancel?: () => void; onError: (value: string) => void }) {
  const [name, setName] = useState(team?.name ?? "");
  const [guildID, setGuildID] = useState(team?.guildId ?? guilds[0]?.id ?? "");
  const [timezone, setTimezone] = useState(team?.timezone ?? browserTimezone());
  const [selected, setSelected] = useState<Member[]>(team?.members.filter((member) => member.id !== team.leaderId) ?? []);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Member[]>([]);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!guildID || query.trim().length < 1) { setResults([]); return; }
    let active = true;
    const timer = window.setTimeout(() => {
      api<Member[]>(`/api/guilds/${guildID}/members?q=${encodeURIComponent(query.trim())}`)
        .then((values) => { if (active) setResults((values ?? []).filter((value) => value.id !== currentUser.id)); })
        .catch((reason) => onError(message(reason)));
    }, 250);
    return () => { active = false; clearTimeout(timer); };
  }, [guildID, query, currentUser.id]);

  function addMember(member: Member) {
    if (selected.length >= 7 || selected.some((value) => value.id === member.id)) return;
    setSelected([...selected, member]);
    setQuery(""); setResults([]);
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault(); setSaving(true);
    try {
      if (team) {
        await api<Team>(`/api/teams/${team.id}`, { method: "PATCH", body: JSON.stringify({ name, memberIds: selected.map((member) => member.id) }) });
        await onSaved(team.id);
      } else {
        const created = await api<Team>("/api/teams", { method: "POST", body: JSON.stringify({ guildId: guildID, name, timezone, memberIds: selected.map((member) => member.id) }) });
        if (created) await onSaved(created.id);
      }
    } catch (reason) { onError(message(reason)); } finally { setSaving(false); }
  }

  async function removeTeam() {
    if (!team) return;
    try { await api(`/api/teams/${team.id}`, { method: "DELETE" }); await onDeleted?.(); }
    catch (reason) { onError(message(reason)); }
  }

  if (team && !team.isLeader) {
    return <><SectionTitle title="Team" detail="Only the team leader can change this roster." /><Roster team={team} /></>;
  }

  return (
    <>
      <SectionTitle title={team ? "Team settings" : "Create a team"} detail={team ? "Update the name or current Discord roster." : "The creator becomes the leader and occupies the first roster slot."} />
      <Card className="max-w-3xl"><form onSubmit={submit}><CardContent className="space-y-6 pb-6">
        {!team && <Field label="Discord server"><Select required value={guildID} onValueChange={(value) => { setGuildID(value); setSelected([]); }}><SelectTrigger className="h-11 w-full" aria-label="Discord server"><SelectValue placeholder="Choose a server" /></SelectTrigger><SelectContent position="popper">{guilds.map((guild) => <SelectItem key={guild.id} value={guild.id}>{guild.name}</SelectItem>)}</SelectContent></Select></Field>}
        <Field label="Team name"><Input className="h-11" aria-label="Team name" maxLength={80} required value={name} onChange={(event) => setName(event.target.value)} placeholder="The Echo" /></Field>
        {!team ? <Field label="Timezone"><TimezoneCombobox value={timezone} onChange={setTimezone} /></Field> : <div><Label>Timezone</Label><div className="mt-1.5 flex h-11 items-center rounded-lg border bg-muted/40 px-3 text-sm text-muted-foreground">{team.timezone}</div><p className="mt-1 text-sm text-muted-foreground">Change this from Schedule so future publication times are reviewed together.</p></div>}
        <div>
          <Label className="mb-2">Teammates ({selected.length + 1}/8)</Label>
          <div className="mb-3 flex flex-wrap gap-2"><MemberChip member={{ id: currentUser.id, name: currentUser.globalName || currentUser.username }} fixed />{selected.map((member) => <MemberChip key={member.id} member={member} onRemove={() => setSelected(selected.filter((value) => value.id !== member.id))} />)}</div>
          <MemberSearch query={query} onQueryChange={setQuery} results={results.filter((result) => !selected.some((member) => member.id === result.id))} disabled={!guildID || selected.length >= 7} full={selected.length >= 7} onSelect={addMember} />
        </div>
        </CardContent><CardFooter className="justify-between gap-3">
          <div>{team && <AlertDialog><AlertDialogTrigger asChild><Button type="button" variant="destructive"><Trash2 />Delete team</Button></AlertDialogTrigger><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Delete {team.name}?</AlertDialogTitle><AlertDialogDescription>This permanently removes the team, schedule, polls, and timetable history.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>Cancel</AlertDialogCancel><AlertDialogAction variant="destructive" onClick={removeTeam}>Delete team</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>}</div>
          <div className="flex gap-3">{onCancel && <Button type="button" variant="outline" onClick={onCancel}>Cancel</Button>}<Button disabled={saving || !guildID}>{saving ? "Saving…" : team ? "Save team" : "Create team"}</Button></div>
        </CardFooter></form></Card>
    </>
  );
}

function Roster({ team }: { team: Team }) {
  return <Card className="max-w-2xl"><CardContent><div className="grid gap-3 sm:grid-cols-2">{team.members.map((member) => <div key={member.id} className="flex items-center gap-3 rounded-lg bg-muted p-3"><Avatar member={member} /><div><div className="font-medium">{member.name}</div><div className="text-xs text-muted-foreground">{member.id === team.leaderId ? "Leader" : "Member"}</div></div></div>)}</div></CardContent></Card>;
}

function MemberChip({ member, fixed, onRemove }: { member: Member; fixed?: boolean; onRemove?: () => void }) {
  return <Badge variant="secondary" className="h-9 gap-2 px-3 text-sm"><span>{member.name}</span>{fixed ? <span className="text-xs text-muted-foreground">Leader</span> : <Button type="button" variant="ghost" size="icon-xs" aria-label={`Remove ${member.name}`} onClick={onRemove}><X /></Button>}</Badge>;
}

function Avatar({ member }: { member: Member }) {
  return <UserAvatar><AvatarImage src={member.avatar} alt="" /><AvatarFallback>{member.name.slice(0, 1).toUpperCase()}</AvatarFallback></UserAvatar>;
}

function SchedulePanel({ team, revision, onChanged, onError }: { team: Team; revision: number; onChanged: () => void | Promise<void>; onError: (value: string) => void }) {
  const [form, setForm] = useState<Schedule>(defaultSchedule(team.timezone));
  const [saved, setSaved] = useState<Schedule | null>(null);
  const [preview, setPreview] = useState<Schedule | null>(null);
  const [previewError, setPreviewError] = useState("");
  const [channels, setChannels] = useState<Channel[]>([]);
  const [choosingChannel, setChoosingChannel] = useState(false);
  const [channelsLoading, setChannelsLoading] = useState(false);
  const [channelsError, setChannelsError] = useState("");
  const [channelsRevision, setChannelsRevision] = useState(0);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [pending, setPending] = useState<"publish" | "republish" | "enabled" | null>(null);

  useEffect(() => {
    if (!team.isLeader) { setLoading(false); return; }
    let active = true;
    setLoading(true);
    setSaved(null);
    setPreview(null);
    setPreviewError("");
    setChannels([]);
    setChannelsError("");
    setForm(defaultSchedule(team.timezone));
    api<Schedule>(`/api/teams/${team.id}/schedule`).then((schedule) => {
      if (!active) return;
      const next = schedule ?? defaultSchedule(team.timezone);
      setSaved(schedule);
      setForm(next);
      setChoosingChannel(!schedule);
    }).catch((reason) => { if (active) onError(message(reason)); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [team.id, revision]);

  useEffect(() => {
    if (!team.isLeader || !choosingChannel) { setChannelsLoading(false); return; }
    let active = true;
    setChannels([]);
    setChannelsError("");
    setChannelsLoading(true);
    api<Channel[]>(`/api/guilds/${team.guildId}/channels`).then((availableChannels) => {
      if (!active) return;
      const values = availableChannels ?? [];
      setChannels(values);
      setForm((current) => current.channelId || !values[0] ? current : { ...current, channelId: values[0].id, channelName: values[0].name });
    }).catch((reason) => { if (active) setChannelsError(message(reason)); }).finally(() => { if (active) setChannelsLoading(false); });
    return () => { active = false; };
  }, [team.guildId, choosingChannel, channelsRevision]);

  useEffect(() => {
    if (loading || form.weekdays.length === 0 || !form.firstPeriodStart || !form.channelId) return;
    let active = true;
    const timer = window.setTimeout(() => {
      api<Schedule>(`/api/teams/${team.id}/schedule/preview`, { method: "POST", body: JSON.stringify(schedulePayload(form)) })
        .then((value) => { if (active) { setPreview(value); setPreviewError(""); } })
        .catch((reason) => { if (active) { setPreview(null); setPreviewError(message(reason)); } });
    }, 350);
    return () => { active = false; clearTimeout(timer); };
  }, [form, team.id, loading]);

  function patch(value: Partial<Schedule>) { setForm((current) => ({ ...current, ...value })); }
  function chooseChannel(id: string) {
    patch({ channelId: id, channelName: channels.find((channel) => channel.id === id)?.name ?? "" });
  }

  function cancelChannelChange() {
    if (saved) patch({ channelId: saved.channelId, channelName: saved.channelName });
    setChoosingChannel(false);
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault(); setSaving(true);
    try {
      const value = await api<Schedule>(`/api/teams/${team.id}/schedule`, { method: "PUT", body: JSON.stringify(schedulePayload(form)) });
      setSaved(value); if (value) setForm(value); await onChanged();
    } catch (reason) { onError(message(reason)); } finally { setSaving(false); }
  }

  async function setEnabled(enabled: boolean) {
    setPending("enabled");
    try { await api(`/api/teams/${team.id}/schedule/enabled`, { method: "POST", body: JSON.stringify({ enabled }) }); setSaved(saved ? { ...saved, enabled } : saved); await onChanged(); }
    catch (reason) { onError(message(reason)); }
    finally { setPending(null); }
  }

  async function action(name: "publish" | "republish") {
    setPending(name);
    try { await api(`/api/teams/${team.id}/${name}`, { method: "POST" }); await onChanged(); }
    catch (reason) { onError(message(reason)); }
    finally { setPending(null); }
  }

  if (!team.isLeader) return <><SectionTitle title="Schedule" detail="Only the team leader can edit the recurring schedule." /><Card><CardContent>Ask your team leader to update the timetable.</CardContent></Card></>;
  if (loading) return <LoadingPanel />;

  return (
    <>
      <div className="flex flex-wrap items-start justify-between gap-4"><SectionTitle title="Schedule" detail="Configure the next weekly or biweekly voting period." />{saved && <div className="flex flex-wrap gap-2"><Button variant="outline" disabled={pending !== null} onClick={() => action("publish")}>{pending === "publish" ? "Publishing…" : "Publish next now"}</Button><Button variant="outline" disabled={pending !== null} onClick={() => action("republish")}>{pending === "republish" ? "Republishing…" : "Republish latest"}</Button><Button variant="outline" disabled={pending !== null} onClick={() => setEnabled(!saved.enabled)}>{saved.enabled ? "Pause automation" : "Enable automation"}</Button></div>}</div>
      <form className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]" onSubmit={submit}>
        <Card><CardContent className="space-y-6">
          <div className="grid gap-5 sm:grid-cols-2">
            <Field label="Cadence"><Select value={form.cadenceDays.toString()} onValueChange={(value) => patch({ cadenceDays: Number(value) })}><SelectTrigger className="h-11 w-full" aria-label="Cadence"><SelectValue /></SelectTrigger><SelectContent position="popper"><SelectItem value="7">Weekly</SelectItem><SelectItem value="14">Biweekly</SelectItem></SelectContent></Select></Field>
            <Field label="Timezone"><TimezoneCombobox value={form.timezone} onChange={(value) => patch({ timezone: value })} /></Field>
          </div>
          <fieldset><Label asChild><legend>Raid weekdays</legend></Label><ToggleGroup type="multiple" variant="outline" value={form.weekdays.map(String)} onValueChange={(values) => patch({ weekdays: values.map(Number).sort() })} className="mt-1.5 grid w-full grid-cols-4 gap-2 sm:grid-cols-7">{weekdays.map((label, day) => <ToggleGroupItem key={label} value={day.toString()} aria-label={label} className="h-11 w-full data-[state=on]:border-primary data-[state=on]:bg-primary data-[state=on]:text-primary-foreground">{label}</ToggleGroupItem>)}</ToggleGroup></fieldset>
          <div className="grid gap-5 sm:grid-cols-2"><Field label="Start time"><TimeSelect label="Start time" value={form.startTime} onChange={(value) => patch({ startTime: value })} /></Field><Field label="End time"><TimeSelect label="End time" value={form.endTime} onChange={(value) => patch({ endTime: value })} /></Field></div>
          <div className="grid gap-5 sm:grid-cols-2"><Field label="First period starts"><DatePicker value={form.firstPeriodStart} onChange={(value) => patch({ firstPeriodStart: value })} /></Field><Field label="Voting opens"><Select value={form.publishLeadDays.toString()} onValueChange={(value) => patch({ publishLeadDays: Number(value) })}><SelectTrigger className="h-11 w-full" aria-label="Voting opens"><SelectValue /></SelectTrigger><SelectContent position="popper">{leadOptions.map((days) => <SelectItem key={days} value={days.toString()}>{days} day{days === 1 ? "" : "s"} before</SelectItem>)}</SelectContent></Select></Field></div>
          <Field label="Discord destination"><div className="rounded-lg border p-3">
            <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Server</div>
            <div className="mt-1 text-sm font-medium">{team.guildName || "Discord server"}</div>
            <Separator className="my-3" />
            {choosingChannel ? <div>
              {channelsLoading ? <div className="flex h-11 items-center rounded-lg border px-3 text-sm text-muted-foreground">Loading Discord channels…</div> : <Select required value={form.channelId} onValueChange={chooseChannel}><SelectTrigger className="h-11 w-full" aria-label="Discord channel"><SelectValue placeholder="Choose a channel" /></SelectTrigger><SelectContent position="popper">{channels.map((channel) => <SelectItem key={channel.id} value={channel.id}>#{channel.name}</SelectItem>)}</SelectContent></Select>}
              {channelsError ? <div className="mt-2 flex items-center justify-between gap-3 text-sm text-destructive"><span>{channelsError}</span><Button type="button" variant="outline" size="sm" onClick={() => setChannelsRevision((value) => value + 1)}>Retry</Button></div> : !channelsLoading && channels.length === 0 && <p className="mt-2 text-sm text-muted-foreground">Raidy needs View Channel, Send Messages, and Embed Links permissions.</p>}
              {saved && <Button type="button" variant="ghost" size="sm" className="mt-2" onClick={cancelChannelChange}>Cancel</Button>}
            </div> : <div className="flex items-center justify-between gap-3">
              <div><div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Channel</div><div className="mt-1 text-sm font-medium">{form.channelName ? `#${form.channelName}` : "Saved channel"}</div></div>
              <Button type="button" variant="outline" size="sm" onClick={() => setChoosingChannel(true)}>Change channel</Button>
            </div>}
          </div></Field>
          <Separator />
          <div className="flex justify-end"><Button disabled={saving || form.weekdays.length === 0 || !form.channelId}>{saving ? "Saving…" : saved ? "Replace schedule" : "Activate schedule"}</Button></div>
        </CardContent></Card>
        <SchedulePreview schedule={preview} error={previewError} />
      </form>
    </>
  );
}

function SchedulePreview({ schedule, error }: { schedule: Schedule | null; error: string }) {
  return <Card className="self-start xl:sticky xl:top-6"><CardHeader><CardTitle>Generated timetable</CardTitle><CardDescription>Times are generated and validated by the backend.</CardDescription></CardHeader><CardContent>{!schedule ? <p className={`py-10 text-center text-sm ${error ? "text-destructive" : "text-muted-foreground"}`}>{error || "Complete the form to preview dates."}</p> : <><div className="space-y-3">{schedule.occurrences?.map((occurrence) => <div key={occurrence.startsAt} className="rounded-lg bg-muted p-3"><div className="font-medium">{formatDateTime(occurrence.startsAt, schedule.timezone)}</div><div className="mt-1 text-xs text-muted-foreground">Until {formatTime(occurrence.endsAt, schedule.timezone)}</div></div>)}</div><Separator className="my-5" /><div className="text-sm"><span className="text-muted-foreground">First publication</span><div className="mt-1 font-medium">{formatDateTime(schedule.nextPublishAt, schedule.timezone)}</div></div></>}</CardContent></Card>;
}

function LoadingPanel() { return <Card><CardContent className="space-y-3 py-10"><Skeleton className="mx-auto h-4 w-32" /><Skeleton className="mx-auto h-4 w-52" /></CardContent></Card>; }

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <div><Label className="mb-1.5">{label}</Label>{children}</div>;
}

function MemberSearch({ query, onQueryChange, results, disabled, full, onSelect }: { query: string; onQueryChange: (value: string) => void; results: Member[]; disabled: boolean; full: boolean; onSelect: (member: Member) => void }) {
  return (
    <Command shouldFilter={false} className="relative overflow-visible bg-transparent p-0">
      <CommandInput value={query} onValueChange={onQueryChange} disabled={disabled} placeholder={full ? "Roster is full" : "Search Discord members"} aria-label="Search Discord members" />
      {query.trim() && !disabled && <CommandList className="absolute top-11 z-20 w-full rounded-lg bg-popover p-1 shadow-md ring-1 ring-foreground/10">
        <CommandEmpty>No matching members.</CommandEmpty>
        <CommandGroup>{results.map((member) => <CommandItem key={member.id} value={member.id} onSelect={() => onSelect(member)}><Avatar member={member} /><span>{member.name}</span></CommandItem>)}</CommandGroup>
      </CommandList>}
    </Command>
  );
}

function TimezoneCombobox({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const [open, setOpen] = useState(false);
  const zones = useMemo(() => {
    const supported = timezones();
    return value && !supported.includes(value) ? [value, ...supported] : supported;
  }, [value]);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild><Button type="button" variant="outline" role="combobox" aria-label="Timezone" aria-expanded={open} className="h-11 w-full justify-between font-normal"><span className="truncate">{value || "Choose a timezone"}</span><ChevronsUpDown className="text-muted-foreground" /></Button></PopoverTrigger>
      <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-0" align="start">
        <Command><CommandInput placeholder="Search timezones…" /><CommandList><CommandEmpty>No timezone found.</CommandEmpty><CommandGroup>{zones.map((zone) => <CommandItem key={zone} value={zone} data-checked={value === zone} onSelect={() => { onChange(zone); setOpen(false); }}>{zone}</CommandItem>)}</CommandGroup></CommandList></Command>
      </PopoverContent>
    </Popover>
  );
}

function TimeSelect({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  const options = timeOptions.includes(value) ? timeOptions : [...timeOptions, value].sort();
  return <Select value={value} onValueChange={onChange}><SelectTrigger className="h-11 w-full" aria-label={label}><SelectValue /></SelectTrigger><SelectContent position="popper" className="max-h-72">{options.map((time) => <SelectItem key={time} value={time}>{time}</SelectItem>)}</SelectContent></Select>;
}

function DatePicker({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const [open, setOpen] = useState(false);
  const selected = parseLocalDate(value);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild><Button type="button" variant="outline" aria-label="First period start date" className="h-11 w-full justify-start font-normal"><CalendarDays className="text-muted-foreground" />{selected ? new Intl.DateTimeFormat(undefined, { dateStyle: "medium" }).format(selected) : "Choose a date"}</Button></PopoverTrigger>
      <PopoverContent className="w-auto p-0" align="start"><Calendar mode="single" selected={selected} defaultMonth={selected} onSelect={(date) => { if (date) { onChange(formatLocalDate(date)); setOpen(false); } }} /></PopoverContent>
    </Popover>
  );
}

function parseLocalDate(value: string): Date | undefined {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return undefined;
  return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
}

function formatLocalDate(value: Date): string {
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, "0")}-${String(value.getDate()).padStart(2, "0")}`;
}

function timezones(): string[] {
  const values = (Intl as typeof Intl & { supportedValuesOf?: (key: string) => string[] }).supportedValuesOf?.("timeZone");
  return values ?? ["Asia/Tokyo", "UTC", "America/Los_Angeles", "America/New_York", "Europe/London", "Europe/Paris", "Asia/Seoul", "Asia/Singapore", "Australia/Sydney"];
}

function browserTimezone() { return Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Tokyo"; }

function defaultSchedule(timezone: string): Schedule {
  const date = new Date();
  const untilMonday = (8 - date.getDay()) % 7 || 7;
  date.setDate(date.getDate() + untilMonday);
  const localDate = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
  return { timezone, cadenceDays: 7, weekdays: [1], startTime: "21:00", endTime: "23:00", publishLeadDays: 3, firstPeriodStart: localDate, nextPublishAt: "", channelId: "", channelName: "", enabled: true };
}

function schedulePayload(schedule: Schedule) {
  return {
    timezone: schedule.timezone,
    cadenceDays: schedule.cadenceDays,
    weekdays: schedule.weekdays,
    startTime: schedule.startTime,
    endTime: schedule.endTime,
    publishLeadDays: schedule.publishLeadDays,
    firstPeriodStart: schedule.firstPeriodStart,
    channelId: schedule.channelId
  };
}

function formatDateTime(value: string, timezone: string) {
  return new Intl.DateTimeFormat(undefined, { timeZone: timezone, weekday: "short", month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(new Date(value));
}

function formatTime(value: string, timezone: string) {
  return new Intl.DateTimeFormat(undefined, { timeZone: timezone, hour: "2-digit", minute: "2-digit" }).format(new Date(value));
}

function message(reason: unknown) { return reason instanceof Error ? reason.message : "Something went wrong"; }
