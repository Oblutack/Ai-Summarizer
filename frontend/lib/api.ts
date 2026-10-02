import axios from "axios";

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "";
export const TURNSTILE_SITE_KEY = process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY ?? "";

// The session lives in an httpOnly cookie that scripts can't read, so the browser must be told to
// send it with every request to the API.
axios.defaults.withCredentials = true;

// Header carrying the Cloudflare Turnstile token on endpoints that require a human check.
export const TURNSTILE_HEADER = "X-Turnstile-Token";

// The server's own message when it sent one (it is written for end users), else the fallback.
export function apiError(err: unknown, fallback: string): string {
  if (axios.isAxiosError(err)) {
    const message = err.response?.data?.error;
    if (typeof message === "string" && message) return message;
    if (!err.response) return "Could not reach the server. Check your connection and try again.";
  }
  return fallback;
}
