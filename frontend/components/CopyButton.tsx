"use client";
import { useEffect, useRef, useState } from "react";
import { copyText } from "../lib/clipboard";
import { CopyIcon } from "./Icon";
import { useT } from "./I18nProvider";

const FEEDBACK_MS = 1800;

interface CopyButtonProps {
  // Read when the button is pressed, so a long text is not rebuilt on every render.
  text: string | (() => string);
  // What is being copied, for screen readers: "summary", "answer".
  what: string;
  // "button" is the large outlined button, "link" a small text button, "icon" an icon only.
  variant?: "button" | "link" | "icon";
}

const LOOKS = {
  button:
    "bg-canvas text-ink text-xl uppercase font-bold py-2 px-6 rounded-md border-2 border-ink hover:bg-ink hover:text-canvas",
  link: "text-sm uppercase tracking-widest text-ink/60 hover:text-ink underline underline-offset-2",
  icon: "text-ink hover:opacity-70",
} as const;

export default function CopyButton({ text, what, variant = "button" }: CopyButtonProps) {
  const [state, setState] = useState<"idle" | "copied" | "failed">("idle");
  const t = useT();
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  const copy = async () => {
    const ok = await copyText(typeof text === "function" ? text() : text);
    setState(ok ? "copied" : "failed");
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setState("idle"), FEEDBACK_MS);
  };

  const label = state === "copied" ? t("copy.copied") : state === "failed" ? t("copy.failed") : t("copy.copy");
  return (
    <>
      <button
        type="button"
        onClick={copy}
        title={t("copy.verb", { what })}
        aria-label={variant === "icon" ? t("copy.verb", { what }) : undefined}
        className={LOOKS[variant]}
      >
        {variant === "icon" ? <CopyIcon className="w-7 h-7" /> : label}
      </button>
      {/* Says the result aloud for screen readers, whatever the button looks like. */}
      <span role="status" className="sr-only">
        {state === "copied" ? t("copy.doneSr", { what }) : state === "failed" ? t("copy.failSr", { what }) : ""}
      </span>
    </>
  );
}
