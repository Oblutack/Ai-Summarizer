import { NextRequest, NextResponse } from "next/server";

// A fresh nonce per request lets Next's own inline bootstrap scripts run while any injected
// script (XSS) is blocked: the browser only runs scripts that carry the nonce, plus scripts those
// trusted scripts load ('strict-dynamic', which is how Google sign-in and Turnstile get in).
export function middleware(request: NextRequest) {
  const nonce = Buffer.from(crypto.randomUUID()).toString("base64");
  const isDev = process.env.NODE_ENV === "development";

  let apiOrigin = "";
  try {
    apiOrigin = new URL(process.env.NEXT_PUBLIC_API_URL ?? "").origin;
  } catch {
    // NEXT_PUBLIC_API_URL unset or malformed: leave the API origin out of the policy.
  }

  const google = "https://accounts.google.com";
  const turnstile = "https://challenges.cloudflare.com";

  const csp = [
    "default-src 'self'",
    // 'unsafe-eval' is only for the dev server's hot reload.
    `script-src 'self' 'nonce-${nonce}' 'strict-dynamic' ${google} ${turnstile}${isDev ? " 'unsafe-eval'" : ""}`,
    // Inline style attributes (React, framer-motion, Tailwind arbitrary values) can't carry a nonce.
    `style-src 'self' 'unsafe-inline' ${google}`,
    "img-src 'self' data: blob:",
    "font-src 'self' data:",
    `connect-src 'self' ${apiOrigin} ${google} ${turnstile}${isDev ? " ws:" : ""}`.replace(/\s+/g, " ").trim(),
    `frame-src ${google} ${turnstile}`,
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "frame-ancestors 'none'",
  ].join("; ");

  // Next reads the nonce from the request's CSP header and applies it to its scripts.
  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-nonce", nonce);
  requestHeaders.set("Content-Security-Policy", csp);

  const response = NextResponse.next({ request: { headers: requestHeaders } });
  response.headers.set("Content-Security-Policy", csp);
  return response;
}

export const config = {
  matcher: [
    {
      // Pages only: not static assets or prefetches.
      source: "/((?!_next/static|_next/image|favicon.ico|noise.png).*)",
      missing: [
        { type: "header", key: "next-router-prefetch" },
        { type: "header", key: "purpose", value: "prefetch" },
      ],
    },
  ],
};
