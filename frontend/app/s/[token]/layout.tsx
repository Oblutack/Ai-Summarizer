import type { Metadata } from "next";

// A shared summary is for the people who were given the link, not for search engines.
export const metadata: Metadata = {
  title: "Shared summary | Inkling",
  robots: { index: false, follow: false },
};

export default function SharedLayout({ children }: { children: React.ReactNode }) {
  return children;
}
