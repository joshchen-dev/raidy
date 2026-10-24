import { CalendarClock, CalendarSync, Globe, MessageSquareText, MousePointerClick, ShieldCheck } from "lucide-react";
import { Wordmark } from "@/components/common";
import { Button } from "@/components/ui/button";
import { HeroDemo } from "./hero-demo";

const github = "https://github.com/joshchen-dev/raidy";

const features = [
  {
    icon: MousePointerClick,
    title: "One tap per night",
    text: "Members mark the nights they can make. Any night left blank counts as a no."
  },
  {
    icon: Globe,
    title: "Everyone's own timezone",
    text: "Raid times show in each member's local time, with the team's time alongside."
  },
  {
    icon: MessageSquareText,
    title: "A Discord post that keeps up",
    text: "One message per week, edited in place as votes come in. No channel spam."
  },
  {
    icon: ShieldCheck,
    title: "The leader makes the call",
    text: "Confirm or cancel a night. A confirmed night is flagged if someone drops out."
  },
  {
    icon: CalendarSync,
    title: "Move the raid, keep the votes",
    text: "Change the start time and the open nights move with it."
  },
  {
    icon: CalendarClock,
    title: "Runs on schedule",
    text: "Weekly or every two weeks, posted a few days ahead without anyone remembering."
  }
];

const steps = [
  { title: "Create your team", text: "Sign in with Discord and pick up to eight members from your server." },
  { title: "Set the schedule", text: "Choose raid days, a time and a channel." },
  { title: "Vote and lock it in", text: "Members tap their nights. The leader confirms." }
];

const questions = [
  {
    q: "Is it free?",
    a: "Yes. Raidy is open source under the MIT license, and you run it yourself with Docker or Kubernetes."
  },
  {
    q: "What can the bot do in my server?",
    a: "Only what it needs to post: view channels, send messages, embed links and read message history. Members vote on the web, not in Discord."
  },
  {
    q: "How do timezones work?",
    a: "The leader sets the team's timezone once. Everyone else sees raid times converted to their own, on the web and in Discord."
  },
  {
    q: "Is it only for Final Fantasy XIV?",
    a: "It was built for FFXIV statics, but any group that meets on fixed nights can use it."
  }
];

export function ProductPage() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="mx-auto flex h-16 max-w-6xl items-center gap-6 px-4 sm:px-6">
        <a href="/" aria-label="Raidy home">
          <Wordmark className="text-xl" />
        </a>
        <nav className="hidden gap-6 text-sm text-muted-foreground sm:flex" aria-label="Sections">
          <a className="hover:text-foreground" href="#features">
            Features
          </a>
          <a className="hover:text-foreground" href="#how-it-works">
            How it works
          </a>
          <a className="hover:text-foreground" href="#faq">
            FAQ
          </a>
        </nav>
        <Button asChild size="sm" className="ml-auto">
          <a href="demo/">Try the demo</a>
        </Button>
      </header>

      <main>
        <section className="mx-auto max-w-6xl px-4 pt-12 pb-20 sm:px-6 sm:pt-20">
          <h1 className="max-w-3xl text-4xl font-semibold tracking-[-0.03em] text-balance sm:text-6xl sm:leading-[1.02]">
            See which nights your whole static can raid.
          </h1>
          <p className="mt-5 max-w-xl text-lg text-muted-foreground">
            Your raid team votes on the web. Raidy keeps one post in your Discord server up to date.
          </p>
          <div className="mt-8 flex flex-wrap gap-3">
            <Button asChild size="lg">
              <a href="demo/">Try the live demo</a>
            </Button>
            <Button asChild size="lg" variant="outline">
              <a href={github}>View on GitHub</a>
            </Button>
          </div>
          <div className="mt-14">
            <HeroDemo />
          </div>
        </section>

        <section id="features" className="border-t">
          <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6">
            <h2 className="max-w-xl text-3xl font-semibold tracking-tight text-balance">
              Less chasing replies, more raiding.
            </h2>
            <ul className="mt-12 grid gap-x-12 gap-y-10 sm:grid-cols-2 lg:grid-cols-3">
              {features.map(({ icon: Icon, title, text }) => (
                <li key={title} className="max-w-sm">
                  <Icon className="size-5 text-primary" aria-hidden="true" />
                  <h3 className="mt-3 font-medium">{title}</h3>
                  <p className="mt-1 text-muted-foreground">{text}</p>
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section id="how-it-works" className="border-t bg-muted/60">
          <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6">
            <h2 className="text-3xl font-semibold tracking-tight">Set up in three steps</h2>
            <ol className="mt-12 grid gap-10 sm:grid-cols-3">
              {steps.map((step, index) => (
                <li key={step.title}>
                  <span className="font-mono text-4xl font-light text-primary tabular-nums">{index + 1}</span>
                  <h3 className="mt-3 font-medium">{step.title}</h3>
                  <p className="mt-1 max-w-xs text-muted-foreground">{step.text}</p>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section aria-labelledby="inside" className="border-t">
          <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6">
            <h2 id="inside" className="text-3xl font-semibold tracking-tight">
              Inside the app
            </h2>
            <div className="mt-12 grid items-end gap-6 lg:grid-cols-[minmax(0,1fr)_280px]">
              <Screen name="schedule" alt="The schedule editor with raid days, times and a preview of the first week" />
              <Screen name="mobile" alt="The availability grid on a phone" narrow />
            </div>
          </div>
        </section>

        <section className="border-t">
          <div className="mx-auto grid max-w-6xl gap-8 px-4 py-20 sm:px-6 lg:grid-cols-2">
            <div>
              <h2 className="text-3xl font-semibold tracking-tight">Free and open source</h2>
              <p className="mt-4 max-w-md text-muted-foreground">
                Run Raidy on your own server with Docker or Kubernetes. Built with Go, PostgreSQL and React.
              </p>
            </div>
            <div className="flex flex-wrap items-start gap-3 lg:justify-end">
              <Button asChild size="lg" variant="outline">
                <a href={github}>View on GitHub</a>
              </Button>
              <Button asChild size="lg" variant="ghost">
                <a href={`${github}#run-locally`}>Setup guide</a>
              </Button>
            </div>
          </div>
        </section>

        <section id="faq" className="border-t">
          <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6">
            <h2 className="text-3xl font-semibold tracking-tight">Questions</h2>
            <div className="mt-10 max-w-3xl divide-y border-y">
              {questions.map(({ q, a }) => (
                <details key={q} className="group py-4">
                  <summary className="flex cursor-pointer list-none items-center justify-between gap-4 font-medium [&::-webkit-details-marker]:hidden">
                    {q}
                    <span
                      aria-hidden="true"
                      className="text-xl leading-none font-light text-muted-foreground transition-transform group-open:rotate-45 motion-reduce:transition-none"
                    >
                      +
                    </span>
                  </summary>
                  <p className="mt-3 max-w-2xl text-muted-foreground">{a}</p>
                </details>
              ))}
            </div>
          </div>
        </section>
      </main>

      <footer className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 gap-y-2 px-4 py-8 text-sm text-muted-foreground sm:px-6">
        <Wordmark />
        <a className="hover:text-foreground" href={github}>
          GitHub
        </a>
        <a className="hover:text-foreground" href={`${github}/blob/main/LICENSE`}>
          MIT License
        </a>
      </footer>
    </div>
  );
}

/** A screenshot of the real app, in the viewer's color scheme. */
function Screen({ name, alt, narrow = false }: { name: string; alt: string; narrow?: boolean }) {
  return (
    <picture className={narrow ? "mx-auto block w-full max-w-[280px]" : "block"}>
      <source srcSet={`screens/${name}-dark.png`} media="(prefers-color-scheme: dark)" />
      <img
        src={`screens/${name}-light.png`}
        alt={alt}
        loading="lazy"
        className="w-full rounded-lg border bg-card shadow-[0_24px_48px_-24px_rgb(28_25_23/0.25)]"
      />
    </picture>
  );
}
