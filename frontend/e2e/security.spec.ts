import { expect, test } from "@playwright/test";
import { API, newSignedInUser, summarizeText } from "./support/helpers";

test.describe("browser security", () => {
  test("every page ships a nonce-based CSP and the other security headers", async ({ request }) => {
    for (const path of ["/", "/login", "/signup"]) {
      const response = await request.get(path);
      const csp = response.headers()["content-security-policy"] ?? "";
      expect(csp, path).toMatch(/script-src[^;]*'nonce-[A-Za-z0-9+/=_-]+'/);
      expect(csp, path).toContain("'strict-dynamic'");
      // Scripts must carry the nonce: no inline or eval fallbacks (styles may be inline).
      const scriptSrc = csp.split(";").find((d) => d.trim().startsWith("script-src")) ?? "";
      expect(scriptSrc, path).not.toContain("'unsafe-inline'");
      expect(scriptSrc, path).not.toContain("'unsafe-eval'");
      expect(response.headers()["x-frame-options"]).toBe("DENY");
      expect(response.headers()["x-content-type-options"]).toBe("nosniff");
      expect(response.headers()["x-powered-by"]).toBeUndefined();
    }
  });

  test("each response gets a fresh nonce", async ({ request }) => {
    const nonce = async () =>
      (await request.get("/login")).headers()["content-security-policy"].match(/'nonce-([^']+)'/)![1];
    expect(await nonce()).not.toBe(await nonce());
  });

  test("the main flows run without a CSP violation or script error", async ({ page }) => {
    const problems: string[] = [];
    page.on("console", (msg) => {
      // "Failed to load resource" is the browser noting an HTTP status: the anonymous /auth/me probe
      // answers 401 by design, and Google rejects the fake client id (as does its own logger).
      // Real errors still fail the test.
      const noise = msg.text().startsWith("Failed to load resource") || msg.text().startsWith("[GSI_LOGGER]");
      if (msg.type() === "error" && !noise) problems.push(msg.text());
    });
    page.on("pageerror", (err) => problems.push(String(err)));
    await page.addInitScript(() => {
      document.addEventListener("securitypolicyviolation", (e) =>
        console.error(`CSP violation: ${e.violatedDirective} ${e.blockedURI}`)
      );
    });

    await page.goto("/");
    await summarizeText(page, "A short text to exercise the whole summarizing flow.");
    await expect(page.getByText("Stub summary")).toBeVisible();
    await newSignedInUser(page);
    await summarizeText(page, "Another short text, this time signed in and saved.");
    await expect(page.locator("h3").first()).toBeVisible();
    await page.getByRole("link", { name: "Account" }).click();
    await expect(page.getByRole("heading", { name: "Your Account" })).toBeVisible();

    expect(problems).toEqual([]);
  });

  test("the session cookie is httpOnly and invisible to scripts", async ({ page, context }) => {
    await newSignedInUser(page);
    const cookie = (await context.cookies()).find((c) => c.name === "session");
    expect(cookie, "a session cookie is set").toBeTruthy();
    expect(cookie!.httpOnly).toBe(true);
    expect(cookie!.sameSite).toBe("Lax");

    expect(await page.evaluate(() => document.cookie)).not.toContain("session");
    // Nothing sensitive in web storage either (older versions kept a token there).
    const stored = await page.evaluate(() => JSON.stringify({ ...localStorage, ...sessionStorage }));
    expect(stored).not.toMatch(/token|session/i);
  });

  test("the API refuses state-changing requests from another origin", async ({ playwright }) => {
    const api = await playwright.request.newContext();
    const forged = await api.post(`${API}/signup`, {
      headers: { Origin: "https://evil.example", "Content-Type": "application/json" },
      data: { email: "x@e2e.example", password: "Correct-Horse-9" },
    });
    expect(forged.status()).toBe(403);
    const normal = await api.post(`${API}/login`, {
      headers: { Origin: "http://localhost:13000", "Content-Type": "application/json" },
      data: { email: "nobody@e2e.example", password: "Correct-Horse-9" },
    });
    expect(normal.status()).toBe(401);
    await api.dispose();
  });

  test("anonymous requests to protected API routes get a machine-readable 401", async ({ playwright }) => {
    const api = await playwright.request.newContext();
    const response = await api.get(`${API}/documents`);
    expect(response.status()).toBe(401);
    expect((await response.json()).code).toBe("unauthenticated");
    await api.dispose();
  });
});
