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
  button: "btn btn-secondary btn-sm",
  link: "btn btn-quiet btn-sm",
  icon: "flex h-10 w-10 items-center justify-center rounded-lg text-ink/80 hover:bg-ink/10 hover:text-ink",
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
        {variant === "icon" ? <CopyIcon className="h-5 w-5" /> : label}
      </button>
      {/* Says the result aloud for screen readers, whatever the button looks like. */}
      <span role="status" className="sr-only">
        {state === "copied" ? t("copy.doneSr", { what }) : state === "failed" ? t("copy.failSr", { what }) : ""}
      </span>
    </>
  );
}
