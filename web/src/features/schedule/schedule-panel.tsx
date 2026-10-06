import { useEffect, useState } from "react";
import { api, type Channel, type Schedule, type Team } from "@/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Loading, SectionTitle, LoadingPanel, Field } from "@/components/common";
import { TimezoneCombobox, TimeSelect, DatePicker } from "@/components/pickers";
import { formatDateTime, formatTime, message } from "@/lib/format";

export const weekdays = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

export const leadOptions = [1, 2, 3, 5, 7, 10, 14];

export function SchedulePanel({
  team,
  revision,
  onChanged,
  onError
}: {
  team: Team;
  revision: number;
  onChanged: () => void | Promise<void>;
  onError: (value: string) => void;
}) {
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
    if (!team.isLeader) {
      setLoading(false);
      return;
    }
    let active = true;
    setLoading(true);
    setSaved(null);
    setPreview(null);
    setPreviewError("");
    setChannels([]);
    setChannelsError("");
    setForm(defaultSchedule(team.timezone));
    api<Schedule>(`/api/teams/${team.id}/schedule`)
      .then((schedule) => {
        if (!active) return;
        const next = schedule ?? defaultSchedule(team.timezone);
        setSaved(schedule);
        setForm(next);
        setChoosingChannel(!schedule);
      })
      .catch((reason) => {
        if (active) onError(message(reason));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [team.id, revision]);

  useEffect(() => {
    if (!team.isLeader || !choosingChannel) {
      setChannelsLoading(false);
      return;
    }
    let active = true;
    setChannels([]);
    setChannelsError("");
    setChannelsLoading(true);
    api<Channel[]>(`/api/guilds/${team.guildId}/channels`)
      .then((availableChannels) => {
        if (!active) return;
        const values = availableChannels ?? [];
        setChannels(values);
        setForm((current) =>
          current.channelId || !values[0]
            ? current
            : { ...current, channelId: values[0].id, channelName: values[0].name }
        );
      })
      .catch((reason) => {
        if (active) setChannelsError(message(reason));
      })
      .finally(() => {
        if (active) setChannelsLoading(false);
      });
    return () => {
      active = false;
    };
  }, [team.guildId, choosingChannel, channelsRevision]);

  useEffect(() => {
    if (loading || form.weekdays.length === 0 || !form.firstPeriodStart || !form.channelId) return;
    let active = true;
    const timer = window.setTimeout(() => {
      api<Schedule>(`/api/teams/${team.id}/schedule/preview`, {
        method: "POST",
        body: JSON.stringify(schedulePayload(form))
      })
        .then((value) => {
          if (active) {
            setPreview(value);
            setPreviewError("");
          }
        })
        .catch((reason) => {
          if (active) {
            setPreview(null);
            setPreviewError(message(reason));
          }
        });
    }, 350);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [form, team.id, loading]);

  function patch(value: Partial<Schedule>) {
    setForm((current) => ({ ...current, ...value }));
  }
  function chooseChannel(id: string) {
    patch({ channelId: id, channelName: channels.find((channel) => channel.id === id)?.name ?? "" });
  }

  function cancelChannelChange() {
    if (saved) patch({ channelId: saved.channelId, channelName: saved.channelName });
    setChoosingChannel(false);
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setSaving(true);
    try {
      const value = await api<Schedule>(`/api/teams/${team.id}/schedule`, {
        method: "PUT",
        body: JSON.stringify(schedulePayload(form))
      });
      setSaved(value);
      if (value) setForm(value);
      await onChanged();
    } catch (reason) {
      onError(message(reason));
    } finally {
      setSaving(false);
    }
  }

  async function setEnabled(enabled: boolean) {
    setPending("enabled");
    try {
      await api(`/api/teams/${team.id}/schedule/enabled`, { method: "POST", body: JSON.stringify({ enabled }) });
      setSaved(saved ? { ...saved, enabled } : saved);
      await onChanged();
    } catch (reason) {
      onError(message(reason));
    } finally {
      setPending(null);
    }
  }

  async function action(name: "publish" | "republish") {
    setPending(name);
    try {
      await api(`/api/teams/${team.id}/${name}`, { method: "POST" });
      await onChanged();
    } catch (reason) {
      onError(message(reason));
    } finally {
      setPending(null);
    }
  }

  if (!team.isLeader)
    return (
      <>
        <SectionTitle title="Schedule" detail="Only the team leader can edit the recurring schedule." />
        <Card>
          <CardContent>Ask your team leader to update the timetable.</CardContent>
        </Card>
      </>
    );
  if (loading) return <LoadingPanel />;

  return (
    <>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <SectionTitle title="Schedule" detail="Configure the next weekly or biweekly voting period." />
        {saved && (
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" disabled={pending !== null} onClick={() => action("publish")}>
              {pending === "publish" ? "Publishing…" : "Publish next now"}
            </Button>
            <Button variant="outline" disabled={pending !== null} onClick={() => action("republish")}>
              {pending === "republish" ? "Republishing…" : "Republish latest"}
            </Button>
            <Button variant="outline" disabled={pending !== null} onClick={() => setEnabled(!saved.enabled)}>
              {saved.enabled ? "Pause automation" : "Enable automation"}
            </Button>
          </div>
        )}
      </div>
      <form className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]" onSubmit={submit}>
        <Card>
          <CardContent className="space-y-6">
            <div className="grid gap-5 sm:grid-cols-2">
              <Field label="Cadence">
                <Select
                  value={form.cadenceDays.toString()}
                  onValueChange={(value) => patch({ cadenceDays: Number(value) })}
                >
                  <SelectTrigger className="h-11 w-full" aria-label="Cadence">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent position="popper">
                    <SelectItem value="7">Weekly</SelectItem>
                    <SelectItem value="14">Biweekly</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <Field label="Timezone">
                <TimezoneCombobox value={form.timezone} onChange={(value) => patch({ timezone: value })} />
              </Field>
            </div>
            <fieldset>
              <Label asChild>
                <legend>Raid weekdays</legend>
              </Label>
              <ToggleGroup
                type="multiple"
                variant="outline"
                value={form.weekdays.map(String)}
                onValueChange={(values) => patch({ weekdays: values.map(Number).sort() })}
                className="mt-1.5 grid w-full grid-cols-4 gap-2 sm:grid-cols-7"
              >
                {weekdays.map((label, day) => (
                  <ToggleGroupItem
                    key={label}
                    value={day.toString()}
                    aria-label={label}
                    className="h-11 w-full data-[state=on]:border-primary data-[state=on]:bg-primary data-[state=on]:text-primary-foreground"
                  >
                    {label}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            </fieldset>
            <div className="grid gap-5 sm:grid-cols-2">
              <Field label="Start time">
                <TimeSelect
                  label="Start time"
                  value={form.startTime}
                  onChange={(value) => patch({ startTime: value })}
                />
              </Field>
              <Field label="End time">
                <TimeSelect label="End time" value={form.endTime} onChange={(value) => patch({ endTime: value })} />
              </Field>
            </div>
            <div className="grid gap-5 sm:grid-cols-2">
              <Field label="First period starts">
                <DatePicker value={form.firstPeriodStart} onChange={(value) => patch({ firstPeriodStart: value })} />
              </Field>
              <Field label="Voting opens">
                <Select
                  value={form.publishLeadDays.toString()}
                  onValueChange={(value) => patch({ publishLeadDays: Number(value) })}
                >
                  <SelectTrigger className="h-11 w-full" aria-label="Voting opens">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent position="popper">
                    {leadOptions.map((days) => (
                      <SelectItem key={days} value={days.toString()}>
                        {days} day{days === 1 ? "" : "s"} before
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
            </div>
            <Field label="Discord destination">
              <div className="rounded-lg border p-3">
                <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Server</div>
                <div className="mt-1 text-sm font-medium">{team.guildName || "Discord server"}</div>
                <Separator className="my-3" />
                {choosingChannel ? (
                  <div>
                    {channelsLoading ? (
                      <div className="flex h-11 items-center rounded-lg border px-3 text-sm text-muted-foreground">
                        Loading Discord channels…
                      </div>
                    ) : (
                      <Select required value={form.channelId} onValueChange={chooseChannel}>
                        <SelectTrigger className="h-11 w-full" aria-label="Discord channel">
                          <SelectValue placeholder="Choose a channel" />
                        </SelectTrigger>
                        <SelectContent position="popper">
                          {channels.map((channel) => (
                            <SelectItem key={channel.id} value={channel.id}>
                              #{channel.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    )}
                    {channelsError ? (
                      <div className="mt-2 flex items-center justify-between gap-3 text-sm text-destructive">
                        <span>{channelsError}</span>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() => setChannelsRevision((value) => value + 1)}
                        >
                          Retry
                        </Button>
                      </div>
                    ) : (
                      !channelsLoading &&
                      channels.length === 0 && (
                        <p className="mt-2 text-sm text-muted-foreground">
                          Raidy needs View Channel, Send Messages, and Embed Links permissions.
                        </p>
                      )
                    )}
                    {saved && (
                      <Button type="button" variant="ghost" size="sm" className="mt-2" onClick={cancelChannelChange}>
                        Cancel
                      </Button>
                    )}
                  </div>
                ) : (
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Channel</div>
                      <div className="mt-1 text-sm font-medium">
                        {form.channelName ? `#${form.channelName}` : "Saved channel"}
                      </div>
                    </div>
                    <Button type="button" variant="outline" size="sm" onClick={() => setChoosingChannel(true)}>
                      Change channel
                    </Button>
                  </div>
                )}
              </div>
            </Field>
            <Separator />
            <div className="flex justify-end">
              <Button disabled={saving || form.weekdays.length === 0 || !form.channelId}>
                {saving ? "Saving…" : saved ? "Replace schedule" : "Activate schedule"}
              </Button>
            </div>
          </CardContent>
        </Card>
        <SchedulePreview schedule={preview} error={previewError} />
      </form>
    </>
  );
}

export function SchedulePreview({ schedule, error }: { schedule: Schedule | null; error: string }) {
  return (
    <Card className="self-start xl:sticky xl:top-6">
      <CardHeader>
        <CardTitle>Generated timetable</CardTitle>
        <CardDescription>Times are generated and validated by the backend.</CardDescription>
      </CardHeader>
      <CardContent>
        {!schedule ? (
          <p className={`py-10 text-center text-sm ${error ? "text-destructive" : "text-muted-foreground"}`}>
            {error || "Complete the form to preview dates."}
          </p>
        ) : (
          <>
            <div className="space-y-3">
              {schedule.occurrences?.map((occurrence) => (
                <div key={occurrence.startsAt} className="rounded-lg bg-muted p-3">
                  <div className="font-medium">{formatDateTime(occurrence.startsAt, schedule.timezone)}</div>
                  <div className="mt-1 text-xs text-muted-foreground">
                    Until {formatTime(occurrence.endsAt, schedule.timezone)}
                  </div>
                </div>
              ))}
            </div>
            <Separator className="my-5" />
            <div className="text-sm">
              <span className="text-muted-foreground">First publication</span>
              <div className="mt-1 font-medium">{formatDateTime(schedule.nextPublishAt, schedule.timezone)}</div>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

export function defaultSchedule(timezone: string): Schedule {
  const date = new Date();
  const untilMonday = (8 - date.getDay()) % 7 || 7;
  date.setDate(date.getDate() + untilMonday);
  const localDate = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
  return {
    timezone,
    cadenceDays: 7,
    weekdays: [1],
    startTime: "21:00",
    endTime: "23:00",
    publishLeadDays: 3,
    firstPeriodStart: localDate,
    nextPublishAt: "",
    channelId: "",
    channelName: "",
    enabled: true
  };
}

export function schedulePayload(schedule: Schedule) {
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
