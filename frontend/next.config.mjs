// next.config.mjs

const isProd = process.env.NODE_ENV === "production";

// Headers that don't depend on the request. The Content-Security-Policy is set per request in
// middleware.ts because it carries a nonce.
const securityHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
  // Only meaningful (and only sent) over https in production.
  ...(isProd
    ? [{ key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains" }]
    : []),
];

/** @type {import('next').NextConfig} */
const config = {
  // The end-to-end tests build into their own directory so they never clash with `pnpm dev`.
  distDir: process.env.NEXT_DIST_DIR || ".next",
  poweredByHeader: false, // don't advertise the framework
  async headers() {
    return [
      { source: "/:path*", headers: securityHeaders },
      // The service worker must be re-checked on every visit, or an updated one would never arrive.
      { source: "/sw.js", headers: [{ key: "Cache-Control", value: "no-cache, max-age=0" }] },
    ];
  },
};

export default config;
