import dynamic from "next/dynamic";

const EInkForm = dynamic(() => import("../components/EInkForm"), {
  ssr: false,
  loading: () => <p>Loading form...</p>,
});

export default function Home() {
  return (
    <main>
      <div className="max-w-5xl mx-auto mt-12 border-2 border-ink rounded-lg p-8">
        <EInkForm
          endpoint={`${process.env.NEXT_PUBLIC_API_URL}/public/summarize`}
        />
      </div>
    </main>
  );
}
