import type { MetadataRoute } from "next";

// What a browser needs to offer "Install Inkling": a name, icons and how to open it.
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "Inkling",
    short_name: "Inkling",
    description: "Read, question and verify your documents. Every answer shows its source.",
    start_url: "/",
    scope: "/",
    display: "standalone",
    background_color: "#e6e5d6",
    theme_color: "#e6e5d6",
    icons: [
      { src: "/icons/icon-192.png", sizes: "192x192", type: "image/png" },
      { src: "/icons/icon-512.png", sizes: "512x512", type: "image/png" },
      { src: "/icons/icon-maskable-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
  };
}
