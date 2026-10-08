import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        // Both are defined in globals.css (and swapped for the dark theme); the channels let opacity
        // modifiers such as text-ink/50 keep working.
        canvas: "rgb(var(--canvas) / <alpha-value>)",
        ink: "rgb(var(--ink) / <alpha-value>)",
      },
      fontFamily: {
        'bebas': ["Bebas Neue Pro Expanded Bold", "sans-serif"],
      },
    },
  },
  plugins: [],
};
export default config;