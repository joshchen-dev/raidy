import { useEffect, useState } from "react";
import { X } from "lucide-react";
import { toast } from "sonner";
import { api, type Guild, type Member, type Team, type User } from "@/api";
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
import { Button } from "@/components/ui/button";
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Avatar, Field, PageHeader, Section } from "@/components/common";
import { TimezoneCombobox } from "@/components/pickers";
import { browserTimezone, message } from "@/lib/format";

const rosterLimit = 8;

export function TeamPanel({
  team,
  guilds,
  currentUser,
  onSaved,
  onDeleted,
  onCancel,
  onError
}: {
  team?: Team;
  guilds: Guild[];
  currentUser: User;
  onSaved: (id: number) => void | Promise<void>;
  onDeleted?: () => void | Promise<void>;
  onCancel?: () => void;
  onError: (value: string) => void;
}) {
  const [name, setName] = useState(team?.name ?? "");
  const [guildID, setGuildID] = useState(team?.guildId ?? guilds[0]?.id ?? "");
  const [timezone, setTimezone] = useState(team?.timezone ?? browserTimezone());
  const [selected, setSelected] = useState<Member[]>(
    team?.members.filter((member) => member.id !== team.leaderId) ?? []
  );
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Member[]>([]);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!guildID || query.trim().length < 1) {
      setResults([]);
      return;
    }
    let active = true;
    const timer = window.setTimeout(() => {
      api<Member[]>(`/api/guilds/${guildID}/members?q=${encodeURIComponent(query.trim())}`)
        .then((values) => {
          if (active) setResults((values ?? []).filter((value) => value.id !== currentUser.id));
        })
        .catch((reason) => onError(message(reason)));
    }, 250);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [guildID, query, currentUser.id]);

  function addMember(member: Member) {
    if (selected.length >= rosterLimit - 1 || selected.some((value) => value.id === member.id)) return;
    setSelected([...selected, member]);
    setQuery("");
    setResults([]);
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setSaving(true);
    try {
      if (team) {
        await api<Team>(`/api/teams/${team.id}`, {
          method: "PATCH",
          body: JSON.stringify({ name, memberIds: selected.map((member) => member.id) })
        });
        toast.success("Roster saved");
        await onSaved(team.id);
      } else {
        const created = await api<Team>("/api/teams", {
          method: "POST",
          body: JSON.stringify({ guildId: guildID, name, timezone, memberIds: selected.map((member) => member.id) })
        });
        toast.success(`${name.trim()} created`);
        if (created) await onSaved(created.id);
      }
    } catch (reason) {
      onError(message(reason));
    } finally {
      setSaving(false);
    }
  }

  async function removeTeam() {
    if (!team) return;
    try {
      await api(`/api/teams/${team.id}`, { method: "DELETE" });
      toast.success(`${team.name} deleted`);
      await onDeleted?.();
    } catch (reason) {
      onError(message(reason));
    }
  }

  if (team && !team.isLeader) {
    return (
      <>
        <PageHeader title="Roster" detail="Only the team leader can change the roster." />
        <RosterList
          members={team.members.map((member) => ({ member, role: member.id === team.leaderId ? "Leader" : "Member" }))}
        />
      </>
    );
  }

  const leader: Member = team?.members.find((member) => member.id === team.leaderId) ?? {
    id: currentUser.id,
    name: currentUser.globalName || currentUser.username
  };
  const full = selected.length >= rosterLimit - 1;

  return (
    <>
      <PageHeader
        title={team ? "Roster" : "Create a team"}
        detail={team ? team.guildName : "You become the leader and take the first of eight roster slots."}
      />
      <form onSubmit={submit} className="max-w-4xl">
        <Section title="Team" description={team ? undefined : "A team belongs to one Discord server."}>
          {!team && (
            <Field label="Discord server">
              <Select
                required
                value={guildID}
                onValueChange={(value) => {
                  setGuildID(value);
                  setSelected([]);
                }}
              >
                <SelectTrigger className="h-10 w-full" aria-label="Discord server">
                  <SelectValue placeholder="Choose a server" />
                </SelectTrigger>
                <SelectContent position="popper">
                  {guilds.map((guild) => (
                    <SelectItem key={guild.id} value={guild.id}>
                      {guild.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
          )}
          <Field label="Name">
            <Input
              className="h-10"
              aria-label="Team name"
              maxLength={80}
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="The Echo"
            />
          </Field>
          {!team ? (
            <Field label="Timezone">
              <TimezoneCombobox value={timezone} onChange={setTimezone} />
            </Field>
          ) : (
            <Field label="Timezone" hint="Change it on the Schedule tab so raid times are reviewed together.">
              <div className="flex h-10 items-center font-mono text-sm text-muted-foreground">{team.timezone}</div>
            </Field>
          )}
        </Section>

        <Section
          title="Members"
          description={`${selected.length + 1} of ${rosterLimit}. Members are Discord accounts in this server.`}
        >
          <RosterList
            members={[
              { member: leader, role: "Leader" },
              ...selected.map((member) => ({
                member,
                role: "Member",
                onRemove: () => setSelected(selected.filter((value) => value.id !== member.id))
              }))
            ]}
          />
          <MemberSearch
            query={query}
            onQueryChange={setQuery}
            results={results.filter((result) => !selected.some((member) => member.id === result.id))}
            disabled={!guildID || full}
            full={full}
            onSelect={addMember}
          />
        </Section>

        <div className="flex justify-end gap-2 border-t pt-6">
          {onCancel && (
            <Button type="button" variant="ghost" onClick={onCancel}>
              Cancel
            </Button>
          )}
          <Button disabled={saving || !guildID}>{saving ? "Saving…" : team ? "Save roster" : "Create team"}</Button>
        </div>
      </form>

      {team && (
        <Section title="Danger zone" className="mt-10 max-w-4xl border-t">
          <div className="flex flex-wrap items-center justify-between gap-4 rounded-md border border-destructive/30 p-4">
            <div>
              <div className="text-sm font-medium">Delete this team</div>
              <p className="mt-0.5 text-sm text-muted-foreground">Removes the schedule and all timetable history.</p>
            </div>
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button type="button" variant="outline" className="border-destructive/40 text-destructive">
                  Delete team
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Delete {team.name}?</AlertDialogTitle>
                  <AlertDialogDescription>
                    This permanently removes the team, its schedule, and every timetable. It cannot be undone.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Keep team</AlertDialogCancel>
                  <AlertDialogAction variant="destructive" onClick={removeTeam}>
                    Delete team
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        </Section>
      )}
    </>
  );
}

function RosterList({ members }: { members: { member: Member; role: string; onRemove?: () => void }[] }) {
  return (
    <ul className="divide-y rounded-md border">
      {members.map(({ member, role, onRemove }) => (
        <li key={member.id} className="flex h-12 items-center gap-3 px-3">
          <Avatar member={member} className="size-7" />
          <span className="min-w-0 flex-1 truncate text-sm">{member.name}</span>
          <span className="text-xs text-muted-foreground">{role}</span>
          {onRemove ? (
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              aria-label={`Remove ${member.name}`}
              onClick={onRemove}
            >
              <X />
            </Button>
          ) : (
            <span className="w-6" aria-hidden="true" />
          )}
        </li>
      ))}
    </ul>
  );
}

function MemberSearch({
  query,
  onQueryChange,
  results,
  disabled,
  full,
  onSelect
}: {
  query: string;
  onQueryChange: (value: string) => void;
  results: Member[];
  disabled: boolean;
  full: boolean;
  onSelect: (member: Member) => void;
}) {
  return (
    <Command shouldFilter={false} className="relative h-auto overflow-visible bg-transparent p-0">
      <CommandInput
        value={query}
        onValueChange={onQueryChange}
        disabled={disabled}
        placeholder={full ? "Roster is full" : "Add a member by Discord name"}
        aria-label="Search Discord members"
      />
      {query.trim() && !disabled && (
        <CommandList className="absolute top-11 z-20 w-full rounded-md border bg-popover p-1 shadow-sm">
          <CommandEmpty>No matching members.</CommandEmpty>
          <CommandGroup>
            {results.map((member) => (
              <CommandItem key={member.id} value={member.id} onSelect={() => onSelect(member)}>
                <Avatar member={member} />
                <span>{member.name}</span>
              </CommandItem>
            ))}
          </CommandGroup>
        </CommandList>
      )}
    </Command>
  );
}
