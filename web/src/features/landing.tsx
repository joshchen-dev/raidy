import { api } from "@/api";
import { Button } from "@/components/ui/button";
import { ThemeSelect } from "@/components/common";
import { type ThemePreference } from "@/lib/theme";

export function Landing({
  error,
  theme,
  onThemeChange
}: {
  error: string;
  theme: ThemePreference;
  onThemeChange: (value: ThemePreference) => void;
}) {
  return (
    <main className="relative grid min-h-screen place-items-center px-5">
      <div className="absolute right-4 top-4">
        <ThemeSelect value={theme} onChange={onThemeChange} />
      </div>
      <section className="w-full max-w-md text-center">
        <div className="mx-auto mb-6 grid size-14 place-items-center rounded-xl bg-[#6957d8] text-2xl font-bold text-white">
          R
        </div>
        <p className="mb-3 text-sm font-semibold uppercase tracking-[.18em] text-primary">Discord raid scheduling</p>
        <h1 className="text-4xl font-semibold tracking-tight">Plan the raid. Keep the party together.</h1>
        <p className="mx-auto mt-4 max-w-sm text-muted-foreground">
          Configure your static team and recurring timetable here. Availability voting stays where your group already
          is—Discord.
        </p>
        {error && <p className="mt-5 text-sm text-destructive">{error}</p>}
        <Button asChild size="lg" className="mt-8 w-full">
          <a href="/api/auth/login">Continue with Discord</a>
        </Button>
      </section>
    </main>
  );
}
