import { useEffect, useState } from "react";
import { toast } from "sonner";
import { api, type Channel, type Schedule, type Team } from "@/api";
import { Field, LoadingPanel, PageHeader, Section } from "@/components/common";
import { DatePicker, TimeSelect, TimezoneCombobox } from "@/components/pickers";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { describeSchedule, formatDateTime, formatDay, formatTime, message } from "@/lib/format";

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
      toast.success(saved ? "Schedule saved" : "Schedule activated");
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
      toast.success(enabled ? "Automatic posting resumed" : "Automatic posting paused");
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
      toast.success(name === "publish" ? "Next timetable posted to Discord" : "Timetable message restored");
      await onChanged();
    } catch (reason) {
      onError(message(reason));
    } finally {
      setPending(null);
    }
  }

  if (!team.isLeader)
    return <PageHeader title="Schedule" detail="Only the team leader can change the recurring schedule." />;
  if (loading) return <LoadingPanel />;

  const dirty = !saved || JSON.stringify(schedulePayload(form)) !== JSON.stringify(schedulePayload(saved));
  const canSave = !saving && form.weekdays.length > 0 && Boolean(form.channelId);

  return (
    <>
      <PageHeader
        title="Schedule"
        detail={
          saved
            ? `${describeSchedule(saved)} · ${saved.enabled ? "posting automatically" : "automatic posting paused"}`
            : "Set the raid days once. Raidy opens voting and announces the dates in Discord before every period."
        }
        actions={
          saved && (
            <>
              <Button variant="outline" size="sm" disabled={pending !== null} onClick={() => action("publish")}>
                {pending === "publish" ? "Posting…" : "Post next period now"}
              </Button>
              <Button variant="outline" size="sm" disabled={pending !== null} onClick={() => action("republish")}>
                {pending === "republish" ? "Restoring…" : "Restore message"}
              </Button>
              <Button variant="ghost" size="sm" disabled={pending !== null} onClick={() => setEnabled(!saved.enabled)}>
                {saved.enabled ? "Pause" : "Resume"}
              </Button>
            </>
          )
        }
      />
      <form className="grid gap-x-12 lg:grid-cols-[minmax(0,1fr)_300px]" onSubmit={submit}>
        <div>
          <Section
            title="When"
            description={
              saved
                ? "A new time or timezone also moves upcoming dates in open periods and keeps their votes. Day changes start with the next period."
                : "One time slot, repeated on the days you pick."
            }
          >
            <fieldset>
              <Label asChild>
                <legend>Raid days</legend>
              </Label>
              <ToggleGroup
                type="multiple"
                variant="outline"
                value={form.weekdays.map(String)}
                onValueChange={(values) => patch({ weekdays: values.map(Number).sort() })}
                className="mt-1.5 flex w-full flex-wrap gap-1.5"
              >
                {weekdays.map((label, day) => (
                  <ToggleGroupItem
                    key={label}
                    value={day.toString()}
                    aria-label={label}
                    className="h-9 min-w-12 flex-1 data-[state=on]:border-primary data-[state=on]:bg-primary/10 data-[state=on]:text-primary"
                  >
                    {label}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            </fieldset>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Starts">
                <TimeSelect
                  label="Start time"
                  value={form.startTime}
                  onChange={(value) => patch({ startTime: value })}
                />
              </Field>
              <Field label="Ends">
                <TimeSelect label="End time" value={form.endTime} onChange={(value) => patch({ endTime: value })} />
              </Field>
            </div>
            <Field label="Timezone">
              <TimezoneCombobox value={form.timezone} onChange={(value) => patch({ timezone: value })} />
            </Field>
          </Section>

          <Section title="Voting" description="Each period opens its own availability vote, announced in Discord.">
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Repeats">
                <Select
                  value={form.cadenceDays.toString()}
                  onValueChange={(value) => patch({ cadenceDays: Number(value) })}
                >
                  <SelectTrigger className="h-10 w-full" aria-label="Cadence">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent position="popper">
                    <SelectItem value="7">Every week</SelectItem>
                    <SelectItem value="14">Every two weeks</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <Field label="Vote opens">
                <Select
                  value={form.publishLeadDays.toString()}
                  onValueChange={(value) => patch({ publishLeadDays: Number(value) })}
                >
                  <SelectTrigger className="h-10 w-full" aria-label="Voting opens">
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
            <Field label="First period starts">
              <DatePicker value={form.firstPeriodStart} onChange={(value) => patch({ firstPeriodStart: value })} />
            </Field>
          </Section>

          <Section title="Discord" description={`Posts go to ${team.guildName || "the team's server"}.`}>
            {choosingChannel ? (
              <Field label="Channel">
                {channelsLoading ? (
                  <div className="flex h-10 items-center text-sm text-muted-foreground">Loading channels…</div>
                ) : (
                  <Select required value={form.channelId} onValueChange={chooseChannel}>
                    <SelectTrigger className="h-10 w-full" aria-label="Discord channel">
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
                  <div className="mt-2 flex items-center gap-3 text-sm text-destructive">
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
                      No channel allows Raidy to post. It needs View Channel, Send Messages, and Embed Links.
                    </p>
                  )
                )}
                {saved && (
                  <Button type="button" variant="ghost" size="sm" className="mt-2 -ml-2" onClick={cancelChannelChange}>
                    Keep #{saved.channelName || "current channel"}
                  </Button>
                )}
              </Field>
            ) : (
              <div className="flex items-center justify-between gap-3">
                <div className="text-sm">
                  <span className="text-muted-foreground">Channel </span>
                  <span className="font-medium">{form.channelName ? `#${form.channelName}` : "Saved channel"}</span>
                </div>
                <Button type="button" variant="outline" size="sm" onClick={() => setChoosingChannel(true)}>
                  Change
                </Button>
              </div>
            )}
          </Section>
        </div>

        <SchedulePreview schedule={preview} error={previewError} />

        {dirty && (
          <div className="sticky bottom-4 z-20 mt-6 flex items-center justify-between gap-4 rounded-md border bg-popover px-4 py-3 shadow-sm lg:col-span-2">
            <span className="text-sm">{saved ? "You have unsaved changes" : "Review the preview, then activate"}</span>
            <div className="flex gap-2">
              {saved && (
                <Button type="button" variant="ghost" size="sm" onClick={() => setForm(saved)}>
                  Discard
                </Button>
              )}
              <Button size="sm" disabled={!canSave}>
                {saving ? "Saving…" : saved ? "Save schedule" : "Activate schedule"}
              </Button>
            </div>
          </div>
        )}
      </form>
    </>
  );
}

export function SchedulePreview({ schedule, error }: { schedule: Schedule | null; error: string }) {
  return (
    <aside className="self-start border-t pt-8 lg:sticky lg:top-6 lg:border-t-0 lg:pt-0" aria-label="Preview">
      <h2 className="text-sm font-medium">First period</h2>
      {!schedule ? (
        <p className={`mt-3 text-sm ${error ? "text-destructive" : "text-muted-foreground"}`}>
          {error || "Pick at least one day and a channel to preview dates."}
        </p>
      ) : (
        <>
          <ol className="mt-3 divide-y rounded-md border">
            {schedule.occurrences?.map((occurrence) => (
              <li key={occurrence.startsAt} className="flex items-center justify-between px-3 py-2 text-sm">
                <span>{formatDay(occurrence.startsAt, schedule.timezone)}</span>
                <span className="font-mono text-xs text-muted-foreground">
                  {formatTime(occurrence.startsAt, schedule.timezone)}–
                  {formatTime(occurrence.endsAt, schedule.timezone)}
                </span>
              </li>
            ))}
          </ol>
          <p className="mt-3 text-sm text-muted-foreground">
            Vote posts{" "}
            <span className="text-foreground">{formatDateTime(schedule.nextPublishAt, schedule.timezone)}</span>
          </p>
        </>
      )}
    </aside>
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
