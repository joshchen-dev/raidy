import { useState } from "react";
import { LogOut, Monitor, Moon, Plus, Sun } from "lucide-react";
import { type User } from "@/api";
import { Avatar } from "@/components/common";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Separator } from "@/components/ui/separator";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { type ThemePreference } from "@/lib/theme";

export function ThemeToggle({
  value,
  onChange
}: {
  value: ThemePreference;
  onChange: (value: ThemePreference) => void;
}) {
  return (
    <ToggleGroup
      type="single"
      variant="outline"
      size="sm"
      value={value}
      onValueChange={(next) => next && onChange(next as ThemePreference)}
      aria-label="Color theme"
    >
      <ToggleGroupItem value="system" aria-label="System theme">
        <Monitor />
      </ToggleGroupItem>
      <ToggleGroupItem value="light" aria-label="Light theme">
        <Sun />
      </ToggleGroupItem>
      <ToggleGroupItem value="dark" aria-label="Dark theme">
        <Moon />
      </ToggleGroupItem>
    </ToggleGroup>
  );
}

function avatarURL(user: User) {
  return user.avatar ? `https://cdn.discordapp.com/avatars/${user.id}/${user.avatar}.png?size=64` : undefined;
}

export function UserMenu({
  user,
  theme,
  onThemeChange,
  onNewTeam,
  onSignOut
}: {
  user: User;
  theme: ThemePreference;
  onThemeChange: (value: ThemePreference) => void;
  onNewTeam: () => void;
  onSignOut: () => void;
}) {
  const [open, setOpen] = useState(false);
  const name = user.globalName || user.username;
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" className="rounded-full" aria-label="Account menu">
          <Avatar member={{ id: user.id, name, avatar: avatarURL(user) }} className="size-7" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-60 p-0">
        <div className="px-3 py-2.5">
          <div className="truncate text-sm font-medium">{name}</div>
          <div className="truncate text-xs text-muted-foreground">@{user.username}</div>
        </div>
        <Separator />
        <div className="flex items-center justify-between px-3 py-2.5">
          <span className="text-sm">Theme</span>
          <ThemeToggle value={theme} onChange={onThemeChange} />
        </div>
        <Separator />
        <div className="p-1">
          <Button
            variant="ghost"
            className="w-full justify-start"
            onClick={() => {
              setOpen(false);
              onNewTeam();
            }}
          >
            <Plus />
            New team
          </Button>
          <Button variant="ghost" className="w-full justify-start" onClick={onSignOut}>
            <LogOut />
            Sign out
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
