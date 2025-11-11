import type { Metadata } from "next";
import "./globals.css";
import Navbar from "../components/Navbar";
import Providers from "../components/Providers";

export const metadata: Metadata = {
  title: "AI Summarizer",
  description: "Summarize your documents with AI",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="font-bebas bg-canvas text-ink">
        <div className="fixed top-0 left-0 w-full h-full bg-[url('/noise.png')] opacity-10 pointer-events-none z-[-1] texture-div-for-pdf-export"></div>

        <Providers>
          {" "}
          {/* "Oblažemo" sve sa našom klijentskom komponentom */}
          <Navbar />
          <main>{children}</main>
        </Providers>
      </body>
    </html>
  );
}
