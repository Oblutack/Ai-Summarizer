"use client";
import { useEffect, useState } from "react";
import { applyTheme, currentTheme, saveTheme, type Theme } from "../lib/theme";
import { useT } from "./I18nProvider";

// Switches between the light and the dark page. It shows the theme a click leads to.
export default function ThemeToggle() {
  // Unknown until the browser has been asked (the server cannot know), so nothing is shown before then.
  const [theme, setTheme] = useState<Theme | null>(null);
  const t = useT();

  useEffect(() => setTheme(currentTheme()), []);

  if (!theme) return <span className="h-11 w-11" aria-hidden="true" />;
  const next: Theme = theme === "dark" ? "light" : "dark";
  const label = t(next === "dark" ? "theme.switchToDark" : "theme.switchToLight");

  return (
    <button
      type="button"
      onClick={() => {
        applyTheme(next);
        saveTheme(next);
        setTheme(next);
      }}
      className="flex h-11 w-11 items-center justify-center rounded-lg hover:bg-ink/10"
      aria-label={label}
      title={label}
    >
      <svg viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
        {next === "dark" ? (
          <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
        ) : (
          <>
            <circle cx="12" cy="12" r="4" />
            <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
          </>
        )}
      </svg>
    </button>
  );
}
