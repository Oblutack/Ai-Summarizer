import type { Metadata, Viewport } from "next";
import { headers } from "next/headers";
import "./globals.css";
import { THEME_SCRIPT } from "../lib/theme";
import Navbar from "../components/Navbar";
import Providers from "../components/Providers";
import RegisterServiceWorker from "../components/RegisterServiceWorker";
import VerifyBanner from "../components/VerifyBanner";

export const metadata: Metadata = {
  title: "Inkling",
  description: "Read, question and verify your documents. Every answer shows its source.",
};

// The browser's own bars take the colour of the page.
export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#e6e5d6" },
    { media: "(prefers-color-scheme: dark)", color: "#1e1e1b" },
  ],
};

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  // Reading the per-request nonce makes every page render dynamically, which a nonce-based
  // Content-Security-Policy requires (a pre-rendered page has no nonce to put on its scripts).
  const nonce = (await headers()).get("x-nonce") ?? undefined;

  return (
    // The theme script sets data-theme before React hydrates, so the attribute may differ from the server's.
    <html lang="en" suppressHydrationWarning>
      <head>
        <script nonce={nonce} dangerouslySetInnerHTML={{ __html: THEME_SCRIPT }} />
      </head>
      <body className="font-bebas bg-canvas text-ink">
        <div className="fixed top-0 left-0 w-full h-full bg-[url('/noise.png')] opacity-10 pointer-events-none z-[-1] texture-div-for-pdf-export"></div>

        <RegisterServiceWorker />
        <Providers nonce={nonce}>
          <Navbar />
          <VerifyBanner />
          <main>{children}</main>
        </Providers>
      </body>
    </html>
  );
}
