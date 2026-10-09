import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { alertOf, newSignedInUser, pdf } from "./support/helpers";

// Recordings are transcribed, then summarized like any document. They are for signed-in people.

const recording = (name: string, bytes = 2048) => ({ name, mimeType: "audio/mpeg", buffer: Buffer.alloc(bytes, 1) });
const summarize = (page: Page) => page.getByRole("button", { name: "Summarize", exact: true }).click();

function card(page: Page, title: string) {
  return page.getByTestId("document-card").filter({ has: page.getByRole("heading", { name: title }) });
}

test.describe("recordings", () => {
  test("a signed-in person attaches a recording, which is transcribed and summarized", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(recording("weekly sync.mp3"));
    await expect(page.getByText("weekly sync.mp3")).toBeVisible();
    await expect(page.getByTestId("audio-hint")).toHaveText(/transcribed first/);

    await summarize(page);
    await expect(page.getByText("Subject: weekly sync.mp3")).toBeVisible();
    await expect(card(page, "weekly sync.mp3")).toBeVisible();
  });

  test("the transcript can be read, with the time of each paragraph, and copied", async ({ page, context }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(recording("weekly sync.mp3"));
    await summarize(page);
    const saved = card(page, "weekly sync.mp3");
    await expect(saved).toBeVisible();

    await saved.getByRole("button", { name: "More" }).click();
    await saved.getByRole("button", { name: "Play the recording" }).click();
    const text = saved.getByTestId("original-text-body");
    await expect(saved.getByRole("heading", { name: "Transcript" })).toBeVisible();
    await expect(saved.getByTestId("recording-panel")).toBeVisible();
    await expect(text).toContainText("0:00 The stub transcript of weekly sync.mp3.");
    await expect(text).toContainText("0:30 Priya will send the budget by Friday.");

    await saved.getByTestId("original-text").getByRole("button", { name: /Copy/ }).click();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toContain("[0:30] Priya will send the budget by Friday.");

    await saved.getByRole("button", { name: "Hide the recording" }).click();
    await expect(saved.getByTestId("recording-panel")).toHaveCount(0);
  });

  test("a recording can be chatted with, since its transcript is its text", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(recording("weekly sync.mp3"));
    await summarize(page);
    const saved = card(page, "weekly sync.mp3");
    await expect(saved.getByRole("button", { name: "Chat With Document" })).toBeVisible();
    await expect(saved.getByRole("button", { name: "Check against the original" })).toBeVisible();
    await expect(saved.getByRole("button", { name: "Listen as a podcast" })).toBeVisible();
  });

  test("a document has its original text to read too", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#main-textarea").fill("Notes from the call. Priya will send the budget by Friday.");
    await summarize(page);
    const saved = page.getByTestId("document-card").first();
    await saved.getByRole("button", { name: "More" }).click();
    await saved.getByRole("button", { name: "Show the original text" }).click();
    await expect(saved.getByRole("heading", { name: "Original text" })).toBeVisible();
    await expect(saved.getByTestId("original-text-body")).toContainText("Priya will send the budget by Friday.");
  });

  test("someone who is not signed in is told to sign in, and nothing is attached", async ({ page }) => {
    await page.goto("/");
    await page.locator("#pdf-upload").setInputFiles(recording("meeting.mp3"));
    await expect(alertOf(page)).toHaveText("Sign in to summarize a recording.");
    await expect(page.getByText("meeting.mp3")).toHaveCount(0);

    // documents still work without an account, and a mixed pick keeps them
    await page.locator("#pdf-upload").setInputFiles([recording("meeting.mp3"), pdf("agenda.pdf")]);
    await expect(alertOf(page)).toHaveText("Sign in to summarize a recording.");
    await expect(page.getByText("agenda.pdf")).toBeVisible();
  });

  test("a recording over the size limit is refused before it is sent", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(recording("long meeting.mp3", 25 * 1024 * 1024 + 1));
    await expect(alertOf(page)).toHaveText("long meeting.mp3 is too large: a recording can be up to 25 MB.");
    await expect(page.getByRole("button", { name: "Remove long meeting.mp3" })).toHaveCount(0); // not in the file list
  });

  test("a recording can be combined with a PDF", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles([pdf("agenda.pdf"), recording("sync.mp3")]);
    await summarize(page);
    await expect(page.getByText("Subject: agenda.pdf, sync.mp3")).toBeVisible();
  });

  test("the file picker offers recordings", async ({ page }) => {
    await page.goto("/");
    const accept = await page.locator("#pdf-upload").getAttribute("accept");
    for (const extension of [".pdf", ".docx", ".pptx", ".mp3", ".m4a", ".wav"]) expect(accept).toContain(extension);
  });

  test("the transcript view has no accessibility violations", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(recording("weekly sync.mp3"));
    await summarize(page);
    const saved = card(page, "weekly sync.mp3");
    await saved.getByRole("button", { name: "More" }).click();
    await saved.getByRole("button", { name: "Play the recording" }).click();
    await expect(saved.getByTestId("original-text-body")).toBeVisible();
    const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
    expect(results.violations.map((v) => `${v.id}: ${v.help} -> ${v.nodes.map((n) => n.target.join(" ") + " " + (n.any[0]?.message ?? "")).join(" | ")}`)).toEqual([]);
  });
});
