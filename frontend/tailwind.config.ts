import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        'canvas': 'rgba(230, 229, 214, 1)',
        'ink': '#222222', 
      },
      fontFamily: {
        'bebas': ["Bebas Neue Pro Expanded Bold", "sans-serif"],
      },
    },
  },
  plugins: [],
};
export default config;