import type { Metadata } from "next";
import { headers } from "next/headers";
import "./globals.css";
import Navbar from "../components/Navbar";
import Providers from "../components/Providers";
import VerifyBanner from "../components/VerifyBanner";

export const metadata: Metadata = {
  title: "AI Summarizer",
  description: "Summarize your documents with AI",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  // Reading the per-request nonce makes every page render dynamically, which a nonce-based
  // Content-Security-Policy requires (a pre-rendered page has no nonce to put on its scripts).
  const nonce = headers().get("x-nonce") ?? undefined;

  return (
    <html lang="en">
      <body className="font-bebas bg-canvas text-ink">
        <div className="fixed top-0 left-0 w-full h-full bg-[url('/noise.png')] opacity-10 pointer-events-none z-[-1] texture-div-for-pdf-export"></div>

        <Providers nonce={nonce}>
          <Navbar />
          <VerifyBanner />
          <main>{children}</main>
        </Providers>
      </body>
    </html>
  );
}
