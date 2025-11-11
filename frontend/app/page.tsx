import dynamic from 'next/dynamic';

// Dinamički importujemo komponentu sa isključenim SSR-om
const EInkForm = dynamic(() => import('../components/EInkForm'), { 
  ssr: false,
  loading: () => <p>Loading form...</p> // Opciono: poruka dok se komponenta učitava
});

export default function Home() {
  return (
    <main>
      <div className="max-w-5xl mx-auto mt-12 border-2 border-ink rounded-lg p-8">
        <EInkForm endpoint="http://localhost:8080/public/summarize" />
      </div>
    </main>
  );
}
