import type { Metadata } from "next";
import "./globals.css";
import Navbar from "../components/Navbar";
import { AuthProvider } from "../contexts/AuthContext";

export const metadata: Metadata = {
  title: "AI Summarizer",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      {/* Primjenjujemo naše custom klase, uključujući font */}

      <body className="font-bebas bg-canvas text-ink">
        <AuthProvider>
          <Navbar />
          <main>{children}</main>
        </AuthProvider>
      </body>
    </html>
  );
}
