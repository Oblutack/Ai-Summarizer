"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import axios from "axios";
import Markdown from "markdown-to-jsx";
import { API_URL } from "../../../lib/api";
import { createMarkdownOptions } from "../../../lib/markdown";
import CopyButton from "../../../components/CopyButton";
import { useT } from "../../../components/I18nProvider";
import { SkeletonLine } from "../../../components/Skeleton";

const markdownOptions = createMarkdownOptions("text-2xl");

interface Shared {
  title: string;
  summary: string;
  createdAt: string;
}

type State = { status: "loading" } | { status: "missing" } | { status: "error" } | { status: "ready"; shared: Shared };

// A summary someone shared by link: read-only, no account needed. It shows the title and the summary only.
export default function SharedSummaryPage() {
  const { token } = useParams<{ token: string }>();
  const t = useT();
  const [state, setState] = useState<State>({ status: "loading" });

  useEffect(() => {
    let cancelled = false;
    axios
      .get<Shared>(`${API_URL}/shared/${encodeURIComponent(token)}`)
      .then((response) => !cancelled && setState({ status: "ready", shared: response.data }))
      .catch((err) => {
        if (cancelled) return;
        setState({ status: axios.isAxiosError(err) && err.response?.status === 404 ? "missing" : "error" });
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  return (
    <div className="max-w-3xl mx-auto mt-8 mb-16 px-4">
      {state.status === "loading" && (
        <div role="status" className="space-y-4">
          <span className="sr-only">{t("shared.loading")}</span>
          <SkeletonLine className="h-9 w-2/3" />
          <SkeletonLine className="w-full" />
          <SkeletonLine className="w-11/12" />
          <SkeletonLine className="w-4/5" />
        </div>
      )}

      {state.status === "missing" && (
        <div className="text-center" role="alert">
          <h1 className="text-3xl uppercase tracking-widest">{t("shared.missingTitle")}</h1>
          <p className="mt-4 text-xl text-ink/70">{t("shared.missing")}</p>
        </div>
      )}

      {state.status === "error" && (
        <p className="text-center text-xl text-red-500" role="alert">
          {t("shared.error")}
        </p>
      )}

      {state.status === "ready" && (
        <article>
          <p className="text-base uppercase tracking-widest text-ink/60">{t("shared.label")}</p>
          <h1 className="mt-1 text-4xl font-bold tracking-wider" data-testid="shared-title">
            {state.shared.title}
          </h1>
          <p className="text-lg text-ink/70">{new Date(state.shared.createdAt).toLocaleDateString()}</p>
          <div className="mt-2 flex gap-4">
            <CopyButton variant="link" what={t("copy.whatSummary")} text={state.shared.summary} />
          </div>
          <hr className="my-4 border-t border-dashed border-ink/50" />
          <div data-testid="shared-summary">
            <Markdown options={markdownOptions}>{state.shared.summary}</Markdown>
          </div>
        </article>
      )}

      <p className="mt-10 border-t border-dashed border-ink/50 pt-4 text-center text-lg tracking-widest text-ink/70">
        {t("shared.madeWith")}{" "}
        <Link href="/" className="underline underline-offset-4">
          Inkling
        </Link>
      </p>
    </div>
  );
}
