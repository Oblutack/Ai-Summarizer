import EInkForm from "../components/EInkForm";
import DownloadButton from "../components/DownloadButton";

export default function Home() {
  return (
    <main>
      <div className="max-w-5xl mx-auto mt-12 border-2 border-ink rounded-lg p-8">
        <EInkForm endpoint="http://localhost:8080/public/summarize" />
      </div>
    </main>
  );
}
