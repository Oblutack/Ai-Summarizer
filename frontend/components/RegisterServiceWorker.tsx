"use client";
import { useEffect } from "react";

// Registers the small service worker that makes the site installable. Only in production: a worker
// in development would get in the way of hot reloading.
export default function RegisterServiceWorker() {
  useEffect(() => {
    if (process.env.NODE_ENV !== "production" || !("serviceWorker" in navigator)) return;
    navigator.serviceWorker.register("/sw.js").catch((error) => {
      // Being installable is a bonus: a failure must never get in the way of the app.
      console.warn("The service worker could not be registered", error);
    });
  }, []);
  return null;
}
