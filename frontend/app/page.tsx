"use client";

import dynamic from "next/dynamic";
import { API_URL } from "../lib/api";
import { useT } from "../components/I18nProvider";

const EInkForm = dynamic(() => import("../components/EInkForm"), {
  ssr: false,
  loading: () => <LoadingText />,
});

function LoadingText() {
  return <p>{useT()("common.loadingForm")}</p>;
}

export default function Home() {
  return (
    <main>
      <div className="max-w-5xl mx-auto mt-12 border-2 border-ink rounded-lg p-8">
        <EInkForm
          endpoint={`${API_URL}/public/summarize`}
        />
      </div>
    </main>
  );
}
