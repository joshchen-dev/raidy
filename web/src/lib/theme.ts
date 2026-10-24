import { useEffect, useState } from "react";

export type Theme = "light" | "dark";

export type ThemePreference = Theme | "system";

export function applyTheme(theme: Theme) {
  document.documentElement.classList.toggle("dark", theme === "dark");
  document.documentElement.dataset.theme = theme;
  document
    .querySelector<HTMLMetaElement>('meta[name="theme-color"]')
    ?.setAttribute("content", theme === "dark" ? "#121110" : "#fafaf9");
}

/**
 * The viewer's theme choice (System, Light or Dark), saved per browser and
 * shared by every page on the site, and the theme it resolves to.
 */
export function useTheme() {
  const [preference, setPreference] = useState<ThemePreference>(() => {
    const saved = document.documentElement.dataset.themePreference;
    return saved === "light" || saved === "dark" ? saved : "system";
  });
  const [resolved, setResolved] = useState<Theme>("light");

  useEffect(() => {
    const media = matchMedia("(prefers-color-scheme: dark)");
    const update = () => {
      const theme = preference === "system" ? (media.matches ? "dark" : "light") : preference;
      applyTheme(theme);
      setResolved(theme);
    };
    update();
    if (preference === "system") media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, [preference]);

  function choose(next: ThemePreference) {
    document.documentElement.dataset.themePreference = next;
    try {
      localStorage.setItem("raidy-theme", next);
    } catch {
      /* Theme still applies for this page. */
    }
    setPreference(next);
  }

  return { preference, resolved, choose };
}
