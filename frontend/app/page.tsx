import SummarizeForm from "../components/SummarizeForm";

export default function Home() {
  return (
    <div className="text-center">
      <h1 className="text-4xl font-bold my-8">
        AI Document Summarizer
      </h1>
      <p className="mb-8 text-lg text-gray-600">
        Upload a PDF and get a concise summary in seconds. <br />
        Log in to save your summary history.
      </p>
      <SummarizeForm endpoint="http://localhost:8080/public/summarize" />
    </div>
  );
}