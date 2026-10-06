import { useEffect, useState } from "react";
import { Trash2, X } from "lucide-react";
import { api, type Guild, type Member, type Schedule, type Team, type User } from "@/api";
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
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter } from "@/components/ui/card";
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SectionTitle, Field, Avatar } from "@/components/common";
import { TimezoneCombobox } from "@/components/pickers";
import { browserTimezone, message } from "@/lib/format";

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
    if (selected.length >= 7 || selected.some((value) => value.id === member.id)) return;
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
        await onSaved(team.id);
      } else {
        const created = await api<Team>("/api/teams", {
          method: "POST",
          body: JSON.stringify({ guildId: guildID, name, timezone, memberIds: selected.map((member) => member.id) })
        });
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
      await onDeleted?.();
    } catch (reason) {
      onError(message(reason));
    }
  }

  if (team && !team.isLeader) {
    return (
      <>
        <SectionTitle title="Team" detail="Only the team leader can change this roster." />
        <Roster team={team} />
      </>
    );
  }

  return (
    <>
      <SectionTitle
        title={team ? "Team settings" : "Create a team"}
        detail={
          team
            ? "Update the name or current Discord roster."
            : "The creator becomes the leader and occupies the first roster slot."
        }
      />
      <Card className="max-w-3xl">
        <form onSubmit={submit}>
          <CardContent className="space-y-6 pb-6">
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
                  <SelectTrigger className="h-11 w-full" aria-label="Discord server">
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
            <Field label="Team name">
              <Input
                className="h-11"
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
              <div>
                <Label>Timezone</Label>
                <div className="mt-1.5 flex h-11 items-center rounded-lg border bg-muted/40 px-3 text-sm text-muted-foreground">
                  {team.timezone}
                </div>
                <p className="mt-1 text-sm text-muted-foreground">
                  Change this from Schedule so future publication times are reviewed together.
                </p>
              </div>
            )}
            <div>
              <Label className="mb-2">Teammates ({selected.length + 1}/8)</Label>
              <div className="mb-3 flex flex-wrap gap-2">
                <MemberChip
                  member={{ id: currentUser.id, name: currentUser.globalName || currentUser.username }}
                  fixed
                />
                {selected.map((member) => (
                  <MemberChip
                    key={member.id}
                    member={member}
                    onRemove={() => setSelected(selected.filter((value) => value.id !== member.id))}
                  />
                ))}
              </div>
              <MemberSearch
                query={query}
                onQueryChange={setQuery}
                results={results.filter((result) => !selected.some((member) => member.id === result.id))}
                disabled={!guildID || selected.length >= 7}
                full={selected.length >= 7}
                onSelect={addMember}
              />
            </div>
          </CardContent>
          <CardFooter className="justify-between gap-3">
            <div>
              {team && (
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button type="button" variant="destructive">
                      <Trash2 />
                      Delete team
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>Delete {team.name}?</AlertDialogTitle>
                      <AlertDialogDescription>
                        This permanently removes the team, schedule, polls, and timetable history.
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>Cancel</AlertDialogCancel>
                      <AlertDialogAction variant="destructive" onClick={removeTeam}>
                        Delete team
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              )}
            </div>
            <div className="flex gap-3">
              {onCancel && (
                <Button type="button" variant="outline" onClick={onCancel}>
                  Cancel
                </Button>
              )}
              <Button disabled={saving || !guildID}>{saving ? "Saving…" : team ? "Save team" : "Create team"}</Button>
            </div>
          </CardFooter>
        </form>
      </Card>
    </>
  );
}

export function Roster({ team }: { team: Team }) {
  return (
    <Card className="max-w-2xl">
      <CardContent>
        <div className="grid gap-3 sm:grid-cols-2">
          {team.members.map((member) => (
            <div key={member.id} className="flex items-center gap-3 rounded-lg bg-muted p-3">
              <Avatar member={member} />
              <div>
                <div className="font-medium">{member.name}</div>
                <div className="text-xs text-muted-foreground">{member.id === team.leaderId ? "Leader" : "Member"}</div>
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

export function MemberChip({ member, fixed, onRemove }: { member: Member; fixed?: boolean; onRemove?: () => void }) {
  return (
    <Badge variant="secondary" className="h-9 gap-2 px-3 text-sm">
      <span>{member.name}</span>
      {fixed ? (
        <span className="text-xs text-muted-foreground">Leader</span>
      ) : (
        <Button type="button" variant="ghost" size="icon-xs" aria-label={`Remove ${member.name}`} onClick={onRemove}>
          <X />
        </Button>
      )}
    </Badge>
  );
}

export function MemberSearch({
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
    <Command shouldFilter={false} className="relative overflow-visible bg-transparent p-0">
      <CommandInput
        value={query}
        onValueChange={onQueryChange}
        disabled={disabled}
        placeholder={full ? "Roster is full" : "Search Discord members"}
        aria-label="Search Discord members"
      />
      {query.trim() && !disabled && (
        <CommandList className="absolute top-11 z-20 w-full rounded-lg bg-popover p-1 shadow-md ring-1 ring-foreground/10">
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
