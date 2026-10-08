"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import axios from "axios";
import Markdown from "markdown-to-jsx";
import { API_URL } from "../../../lib/api";
import { createMarkdownOptions } from "../../../lib/markdown";
import { tidyMarkdown } from "../../../lib/markdownText";
import CopyButton from "../../../components/CopyButton";
import { useT } from "../../../components/I18nProvider";
import { SkeletonLine } from "../../../components/Skeleton";

const markdownOptions = createMarkdownOptions();

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
          <h1 className="text-3xl">{t("shared.missingTitle")}</h1>
          <p className="muted mt-3 text-lg">{t("shared.missing")}</p>
        </div>
      )}

      {state.status === "error" && (
        <p className="text-center text-lg font-medium text-danger" role="alert">
          {t("shared.error")}
        </p>
      )}

      {state.status === "ready" && (
        <article>
          <p className="text-sm font-semibold text-ink/70">{t("shared.label")}</p>
          <h1 className="mt-1 font-sans text-3xl font-bold tracking-normal md:text-4xl" data-testid="shared-title">
            {state.shared.title}
          </h1>
          <p className="muted">{new Date(state.shared.createdAt).toLocaleDateString()}</p>
          <div className="mt-2 flex gap-4">
            <CopyButton variant="link" what={t("copy.whatSummary")} text={state.shared.summary} />
          </div>
          <hr className="my-5 border-t border-ink/15" />
          <div data-testid="shared-summary" className="reading">
            <Markdown options={markdownOptions}>{tidyMarkdown(state.shared.summary)}</Markdown>
          </div>
        </article>
      )}

      <p className="mt-10 border-t border-ink/15 pt-4 text-center text-sm text-ink/70">
        {t("shared.madeWith")}{" "}
        <Link href="/" className="underline underline-offset-4">
          Inkling
        </Link>
      </p>
    </div>
  );
}
