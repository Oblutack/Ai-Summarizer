// Keep in sync with go-api/controllers/options.go and python-ai-service/prompts.py.
export const SUMMARY_STYLES = [
  { value: "default", label: "Standard" },
  { value: "bullets", label: "Bullet Points" },
  { value: "brief", label: "Executive Brief" },
  { value: "simple", label: "Explain Simply" },
  { value: "takeaways", label: "Takeaways + Actions" },
] as const;

export const LANGUAGES = [
  "English",
  "Spanish",
  "French",
  "German",
  "Italian",
  "Portuguese",
  "Dutch",
  "Polish",
  "Turkish",
  "Russian",
  "Serbian",
  "Croatian",
  "Bosnian",
  "Chinese",
  "Japanese",
] as const;

export const MAX_FILES = 5;
