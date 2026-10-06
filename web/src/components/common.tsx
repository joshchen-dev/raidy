import { Plus, X } from "lucide-react";
import { type Member } from "@/api";
import { Alert, AlertAction, AlertDescription } from "@/components/ui/alert";
import { Avatar as UserAvatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { type ThemePreference } from "@/lib/theme";

export function ThemeSelect({
  value,
  onChange
}: {
  value: ThemePreference;
  onChange: (value: ThemePreference) => void;
}) {
  return (
    <Select value={value} onValueChange={(next) => onChange(next as ThemePreference)}>
      <SelectTrigger className="h-11 w-28" aria-label="Color theme">
        <SelectValue />
      </SelectTrigger>
      <SelectContent position="popper">
        <SelectItem value="system">System</SelectItem>
        <SelectItem value="light">Light</SelectItem>
        <SelectItem value="dark">Dark</SelectItem>
      </SelectContent>
    </Select>
  );
}

export function Loading() {
  return <div className="grid min-h-screen place-items-center text-sm text-muted-foreground">Loading Raidy…</div>;
}

export function ErrorBanner({ text, onClose }: { text: string; onClose: () => void }) {
  return (
    <Alert variant="destructive" className="mb-5">
      <AlertDescription>{text}</AlertDescription>
      <AlertAction>
        <Button variant="ghost" size="icon-sm" aria-label="Dismiss error" onClick={onClose}>
          <X />
        </Button>
      </AlertAction>
    </Alert>
  );
}

export function EmptyState({ onCreate }: { onCreate: () => void }) {
  return (
    <Card className="py-12 text-center">
      <CardContent>
        <h1 className="text-2xl font-semibold">Create your first team</h1>
        <p className="mx-auto mt-2 max-w-md text-muted-foreground">
          Choose a Discord server, add up to seven teammates, then configure your recurring timetable.
        </p>
        <Button className="mt-6" onClick={onCreate}>
          <Plus />
          Create team
        </Button>
      </CardContent>
    </Card>
  );
}

export function SectionTitle({ title, detail }: { title: string; detail: string }) {
  return (
    <div className="mb-6">
      <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
      <p className="mt-1 text-sm text-muted-foreground">{detail}</p>
    </div>
  );
}

export function LoadingPanel() {
  return (
    <Card>
      <CardContent className="space-y-3 py-10">
        <Skeleton className="mx-auto h-4 w-32" />
        <Skeleton className="mx-auto h-4 w-52" />
      </CardContent>
    </Card>
  );
}

export function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <Label className="mb-1.5">{label}</Label>
      {children}
    </div>
  );
}

export function Avatar({ member }: { member: Member }) {
  return (
    <UserAvatar>
      <AvatarImage src={member.avatar} alt="" />
      <AvatarFallback>{member.name.slice(0, 1).toUpperCase()}</AvatarFallback>
    </UserAvatar>
  );
}

export function Status({ status }: { status: string }) {
  const label = status.replaceAll("_", " ");
  return (
    <Badge variant={status === "attention_required" ? "destructive" : "secondary"} className="mt-1 capitalize">
      {label}
    </Badge>
  );
}
