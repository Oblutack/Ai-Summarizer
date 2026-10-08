import type { Config } from "tailwindcss";

// The colours are defined in globals.css (and swapped for the dark theme); the "R G B" channels let
// opacity modifiers such as text-ink/70 keep working.
const color = (name: string) => `rgb(var(--${name}) / <alpha-value>)`;

const config: Config = {
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        canvas: color("canvas"),
        surface: color("surface"),
        ink: color("ink"),
        accent: color("accent"),
        "accent-fg": color("accent-fg"),
        danger: color("danger"),
        "danger-fg": color("danger-fg"),
      },
      fontFamily: {
        // Titles only.
        display: ["Bebas Neue Pro Expanded Bold", "sans-serif"],
        // Interface text.
        sans: ["Inter Variable", "ui-sans-serif", "system-ui", "-apple-system", "Segoe UI", "sans-serif"],
        // Text people read at length.
        serif: ["Source Serif 4 Variable", "Georgia", "Cambria", "serif"],
      },
    },
  },
  plugins: [],
};
export default config;
