import { expect, type Locator, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

export const API = "http://localhost:18080";
export const PASSWORD = "Correct-Horse-9";

// Every test gets its own account so tests can run in parallel and in any order.
export function uniqueEmail(label = "user"): string {
  const id = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
  return `${label}-${id}@e2e.example`;
}

export async function signUp(page: Page, email: string, password = PASSWORD) {
  await page.goto("/signup");
  await page.locator("#email").fill(email);
  await page.locator("#password").fill(password);
  await page.getByRole("button", { name: "Sign Up", exact: true }).click();
  await expect(page.getByText("Check your email")).toBeVisible();
}

export async function logIn(page: Page, email: string, password = PASSWORD) {
  await page.goto("/login");
  await page.locator("#email").fill(email);
  await page.locator("#password").fill(password);
  await page.getByRole("button", { name: "Sign In", exact: true }).click();
}

// Creates an account and leaves the page signed in on the dashboard.
export async function newSignedInUser(page: Page, label = "user"): Promise<string> {
  const email = uniqueEmail(label);
  await signUp(page, email);
  await logIn(page, email);
  await expect(page).toHaveURL(/\/dashboard/);
  await expect(page.getByRole("heading", { name: "Your Dashboard" })).toBeVisible();
  return email;
}

// The Go API runs with MAIL_PROVIDER=log, which writes each email to its log instead of sending it.
// This finds the link in the most recent email with the given subject for the given address.
export async function emailedLink(email: string, subject: string): Promise<string> {
  const logPath = join(__dirname, "..", ".tmp", "api.log");
  for (let attempt = 0; attempt < 40; attempt++) {
    const lines = readFileSync(logPath, "utf8").split("\n").reverse();
    for (const line of lines) {
      if (!line.includes("MAIL_PROVIDER=log")) continue;
      try {
        const entry = JSON.parse(line);
        if (entry.to === email && entry.subject === subject) {
          const link = String(entry.body).match(/https?:\/\/\S+/)?.[0];
          if (link) return link;
        }
      } catch {
        // not a JSON line
      }
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error(`No "${subject}" email for ${email} found in the API log`);
}

// Types or pastes text into the summarizer and runs it.
export async function summarizeText(page: Page, text: string) {
  await page.locator("#main-textarea").fill(text);
  await page.getByRole("button", { name: "Summarize", exact: true }).click();
}

// The bytes only need to carry the .pdf name: the stub AI service never opens them.
export function pdf(name: string) {
  return { name, mimeType: "application/pdf", buffer: Buffer.from(`%PDF-1.4 ${name}`) };
}

// Error messages. Next.js also keeps an empty role="alert" element for route announcements, which
// a plain getByRole("alert") would match as well.
export function alertOf(page: Page): Locator {
  return page.locator('[role="alert"]:not(#__next-route-announcer__)');
}
