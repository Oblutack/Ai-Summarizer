"use client";
import { useEffect, useState } from "react";
import { applyTheme, currentTheme, saveTheme, type Theme } from "../lib/theme";
import { useT } from "./I18nProvider";

// Switches between the light and the dark page. The label says where a click leads.
export default function ThemeToggle() {
  // Unknown until the browser has been asked (the server cannot know), so nothing is shown before then.
  const [theme, setTheme] = useState<Theme | null>(null);
  const t = useT();

  useEffect(() => setTheme(currentTheme()), []);

  if (!theme) return null;
  const next: Theme = theme === "dark" ? "light" : "dark";

  return (
    <button
      type="button"
      onClick={() => {
        applyTheme(next);
        saveTheme(next);
        setTheme(next);
      }}
      className="hover:opacity-70"
      aria-label={t(next === "dark" ? "theme.switchToDark" : "theme.switchToLight")}
      title={t(next === "dark" ? "theme.switchToDark" : "theme.switchToLight")}
    >
      {t(next === "dark" ? "theme.dark" : "theme.light")}
    </button>
  );
}
