import { ExternalLink } from "lucide-react";
import { cn } from "@/lib/utils";

export type PostNight = { id: number; startsAt: string; available: number; status: string };

// Mirrors statusIcon and statusLabel in backend/internal/discord/poll.go.
const icons: Record<string, string> = {
  proposed: "🗓️",
  confirmed: "✅",
  cancelled: "❌",
  attention_required: "⚠️"
};
const labels: Record<string, string> = {
  proposed: "Proposed",
  confirmed: "Confirmed",
  cancelled: "Cancelled",
  attention_required: "Attention required"
};

/** Discord renders <t:unix:F> in each reader's own locale and timezone. */
function discordTime(value: string) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: "full", timeStyle: "short" }).format(new Date(value));
}

/**
 * A faithful copy of the timetable post Raidy keeps in Discord. It is always
 * drawn in Discord's dark theme, like the real client most people use.
 */
export function DiscordPost({
  team,
  period,
  timezone,
  nights,
  total,
  changed
}: {
  team: string;
  period: string;
  timezone: string;
  nights: PostNight[];
  total: number;
  changed: number | null;
}) {
  return (
    <figure
      aria-label="The Raidy post in Discord"
      className="rounded-lg bg-[#313338] p-4 font-sans text-[15px] leading-snug text-[#dbdee1] shadow-[0_1px_0_#1e1f22]"
    >
      <div className="flex gap-3">
        <div
          aria-hidden="true"
          className="grid size-10 shrink-0 place-items-center rounded-full bg-[#0f6e66] text-sm font-semibold text-white"
        >
          r
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-baseline gap-1.5">
            <span className="font-medium text-[#f2f3f5]">Raidy</span>
            <span className="rounded-[3px] bg-[#5865f2] px-1 text-[10px] leading-4 font-semibold text-white">APP</span>
            <span className="text-xs text-[#949ba4]">Today at 9:00</span>
          </div>
          <div className="mt-1.5 max-w-md rounded border-l-4 border-[#1e1f22] bg-[#2b2d31] py-2.5 pr-4 pl-3">
            <div className="font-semibold text-[#f2f3f5]">{team} raid timetable</div>
            <div className="mt-1 text-sm">
              {period} ·{" "}
              <code className="rounded bg-[#1e1f22] px-1 py-px font-mono text-[13px] text-[#dbdee1]">{timezone}</code>
            </div>
            <dl className="mt-3 space-y-2.5 text-sm">
              {nights.map((night) => (
                <div key={night.id}>
                  <dt className="font-semibold text-[#f2f3f5]">
                    {icons[night.status]} {discordTime(night.startsAt)}
                  </dt>
                  <dd
                    className={cn(
                      "w-fit rounded px-0.5 transition-colors duration-700 motion-reduce:transition-none",
                      changed === night.id && "bg-[#5865f2]/30"
                    )}
                  >
                    <strong className="text-[#f2f3f5]">
                      {night.available}/{total} available
                    </strong>{" "}
                    · {labels[night.status]}
                  </dd>
                </div>
              ))}
            </dl>
            <div className="mt-3 text-xs text-[#b5bac1]">
              Times are shown in your timezone. Vote and manage dates in Raidy.
            </div>
          </div>
          <a
            href="demo/"
            className="mt-2 inline-flex h-8 items-center gap-1.5 rounded-[3px] bg-[#4e5058] px-4 text-sm font-medium text-white hover:bg-[#6d6f78]"
          >
            Open in Raidy
            <ExternalLink className="size-3.5" aria-hidden="true" />
          </a>
        </div>
      </div>
    </figure>
  );
}
