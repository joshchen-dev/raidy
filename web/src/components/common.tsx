import { X } from "lucide-react";
import { type Member } from "@/api";
import { Alert, AlertAction, AlertDescription } from "@/components/ui/alert";
import { Avatar as UserAvatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

export function Wordmark({ className }: { className?: string }) {
  return <span className={cn("font-semibold tracking-tight text-primary", className)}>raidy</span>;
}

export function Loading() {
  return <div className="grid min-h-screen place-items-center text-sm text-muted-foreground">Loading…</div>;
}

export function ErrorBanner({ text, onClose }: { text: string; onClose: () => void }) {
  return (
    <Alert variant="destructive" className="mb-6">
      <AlertDescription>{text}</AlertDescription>
      <AlertAction>
        <Button variant="ghost" size="icon-sm" aria-label="Dismiss error" onClick={onClose}>
          <X />
        </Button>
      </AlertAction>
    </Alert>
  );
}

export function PageHeader({
  title,
  detail,
  actions
}: {
  title: string;
  detail?: React.ReactNode;
  actions?: React.ReactNode;
}) {
  return (
    <div className="mb-8 flex flex-wrap items-end justify-between gap-4">
      <div className="min-w-0">
        <h1 className="text-xl font-semibold tracking-tight">{title}</h1>
        {detail && <p className="mt-1 text-sm text-muted-foreground">{detail}</p>}
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </div>
  );
}

/** A titled settings block separated by a hairline instead of a card. */
export function Section({
  title,
  description,
  children,
  className
}: {
  title: string;
  description?: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section className={cn("grid gap-6 border-t py-8 first:border-t-0 first:pt-0 md:grid-cols-[220px_1fr]", className)}>
      <div>
        <h2 className="text-sm font-medium">{title}</h2>
        {description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
      </div>
      <div className="min-w-0 space-y-5">{children}</div>
    </section>
  );
}

export function LoadingPanel() {
  return (
    <div className="space-y-3 py-6" aria-busy="true" aria-label="Loading">
      <Skeleton className="h-5 w-48" />
      <Skeleton className="h-4 w-72" />
      <Skeleton className="mt-6 h-40 w-full" />
    </div>
  );
}

export function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div>
      <Label className="mb-1.5">{label}</Label>
      {children}
      {hint && <p className="mt-1.5 text-xs text-muted-foreground">{hint}</p>}
    </div>
  );
}

export function Avatar({ member, className }: { member: Member; className?: string }) {
  return (
    <UserAvatar className={cn("size-6", className)}>
      <AvatarImage src={member.avatar} alt="" />
      <AvatarFallback className="text-[10px]">{member.name.slice(0, 1).toUpperCase()}</AvatarFallback>
    </UserAvatar>
  );
}

const statusStyles: Record<string, string> = {
  confirmed: "bg-available-soft text-available",
  attention_required: "bg-unavailable-soft text-unavailable",
  cancelled: "bg-muted text-muted-foreground line-through",
  proposed: "border bg-background text-muted-foreground"
};

const statusLabels: Record<string, string> = {
  confirmed: "Confirmed",
  attention_required: "Needs attention",
  cancelled: "Cancelled",
  proposed: "Proposed"
};

export function StatusChip({ status }: { status: string }) {
  return (
    <span
      className={cn(
        "inline-flex h-5 items-center rounded px-1.5 text-[11px] font-medium",
        statusStyles[status] ?? statusStyles.proposed
      )}
    >
      {statusLabels[status] ?? status}
    </span>
  );
}
