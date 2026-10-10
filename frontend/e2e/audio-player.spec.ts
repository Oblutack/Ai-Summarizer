import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { newSignedInUser } from "./support/helpers";
import { wavFile } from "./support/audio";

// The player for recordings: before they are summarized (a recording made here, or a file picked), and after.

const player = (page: Page) => page.getByTestId("audio-player").first();
// The state of a player's sound. Scoped to a saved card's panel when one is given: after a summary the form above
// still holds its own preview of the file that was attached.
const audioState = (page: Page, scope = "audio") =>
  page.locator(scope).first().evaluate((a: HTMLAudioElement) => ({ time: a.currentTime, rate: a.playbackRate, paused: a.paused, duration: a.duration }));

test.describe("playing a recording before it is summarized", () => {
  test("a picked file can be played, paused, skipped and moved through", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(wavFile("sync.wav", 30));
    await expect(player(page)).toBeVisible();
    await expect(page.getByTestId("audio-time").first()).toHaveText("0:00 / 0:30");

    await player(page).getByRole("button", { name: "Play", exact: true }).click();
    await expect.poll(async () => (await audioState(page)).time).toBeGreaterThan(0.2);
    await player(page).getByRole("button", { name: "Pause", exact: true }).click();
    expect((await audioState(page)).paused).toBe(true);

    // ten seconds forward, ten back
    await player(page).getByRole("button", { name: "Forward 10 seconds" }).click();
    await expect.poll(async () => Math.round((await audioState(page)).time)).toBeGreaterThanOrEqual(10);
    await expect(page.getByTestId("audio-time").first()).toHaveText(/^0:1\d \/ 0:30$/);
    await player(page).getByRole("button", { name: "Back 10 seconds" }).click();
    await expect.poll(async () => (await audioState(page)).time).toBeLessThan(2);

    // the slider goes anywhere
    const slider = player(page).getByTestId("audio-position");
    await slider.focus();
    await page.keyboard.press("Home"); // from the very start: the sound may have played a little before it was paused
    for (let i = 0; i < 5; i++) await page.keyboard.press("ArrowRight");
    await expect.poll(async () => Math.round((await audioState(page)).time)).toBe(5);
    await expect(slider).toHaveAttribute("aria-valuetext", "0:05 of 0:30");
  });

  test("the speed applies to the sound and is remembered", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(wavFile("sync.wav", 10));
    await expect(player(page)).toBeVisible();
    await player(page).getByLabel("Speed").selectOption("1.5");
    expect((await audioState(page)).rate).toBe(1.5);

    await page.reload();
    await expect(page.getByRole("heading", { name: "Your Dashboard" })).toBeVisible();
    await page.locator("#pdf-upload").setInputFiles(wavFile("again.wav", 10));
    await expect(player(page).getByLabel("Speed")).toHaveValue("1.5");
    expect((await audioState(page)).rate).toBe(1.5);
  });

  test("a recording made here has a length, and can be played back before it is used", async ({ page }) => {
    await newSignedInUser(page);
    await page.getByTestId("record-button").click();
    await page.waitForTimeout(2200);
    await page.getByRole("button", { name: "Stop and use the recording" }).click();
    await expect(player(page)).toBeVisible();
    // the browser reports no length for what it records; the player works it out
    await expect(page.getByTestId("audio-time").first()).toHaveText(/^0:00 \/ 0:0[1-9]$/);
    await player(page).getByRole("button", { name: "Play", exact: true }).click();
    await expect.poll(async () => (await audioState(page)).time).toBeGreaterThan(0.2);
  });

  test("a file that cannot be played says so, and can still be summarized", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles({ name: "broken.mp3", mimeType: "audio/mpeg", buffer: Buffer.alloc(2048, 1) });
    await expect(page.getByText("This recording could not be played in your browser.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Summarize", exact: true })).toBeEnabled();
  });

  test("removing the file removes its player and stops the sound", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(wavFile("sync.wav", 30));
    await player(page).getByRole("button", { name: "Play", exact: true }).click();
    await expect.poll(async () => (await audioState(page)).paused).toBe(false);
    await page.getByRole("button", { name: "Remove sync.wav" }).click();
    await expect(page.getByTestId("audio-player")).toHaveCount(0);
    await expect(page.locator("audio")).toHaveCount(0);
  });

  test("the player has no accessibility violations", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(wavFile("sync.wav", 30));
    await expect(player(page)).toBeVisible();
    const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
    expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  });
});

test.describe("playing a saved recording", () => {
  const SAVED = '[data-testid="recording-panel"] audio';
  const summarize = (page: Page) => page.getByRole("button", { name: "Summarize", exact: true }).click();

  async function saveRecording(page: Page, seconds = 60) {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(wavFile("weekly sync.wav", seconds));
    await summarize(page);
    const card = page.getByTestId("document-card").filter({ has: page.getByRole("heading", { name: "weekly sync.wav" }) });
    await expect(card).toBeVisible();
    await card.getByRole("button", { name: "More" }).click();
    return card;
  }

  test("the recording is kept and can be played from the saved summary", async ({ page }) => {
    const card = await saveRecording(page);
    await card.getByRole("button", { name: "Play the recording" }).click();
    const panel = card.getByTestId("recording-panel");
    await expect(panel.getByTestId("audio-time")).toHaveText("0:00 / 1:00");

    await panel.getByRole("button", { name: "Play", exact: true }).click();
    await expect.poll(async () => (await audioState(page, SAVED)).time).toBeGreaterThan(0.2);
    await panel.getByLabel("Speed").selectOption("2");
    expect((await audioState(page, SAVED)).rate).toBe(2);
  });

  test("a time in the transcript starts the recording from there", async ({ page }) => {
    const card = await saveRecording(page);
    await card.getByRole("button", { name: "Play the recording" }).click();
    const panel = card.getByTestId("recording-panel");
    await expect(panel.getByTestId("audio-time")).toHaveText("0:00 / 1:00");

    await panel.getByRole("button", { name: "Play from 0:30" }).click();
    await expect.poll(async () => Math.round((await audioState(page, SAVED)).time)).toBeGreaterThanOrEqual(30);
    expect((await audioState(page, SAVED)).paused).toBe(false);
  });

  test("a cited passage of a recording can be played from the chat", async ({ page }) => {
    const card = await saveRecording(page);
    await card.getByRole("button", { name: "Chat With Document" }).click();
    const chat = card.getByTestId("document-chat");
    await chat.getByPlaceholder("Type a question").fill("Who sends the budget?");
    await page.keyboard.press("Enter");
    await chat.getByRole("button", { name: "Show source 1" }).click();
    await expect(chat.getByTestId("sources")).toContainText("Priya will send the budget by Friday.");
    // there is no page to open in a PDF viewer, but the moment it was said can be played
    await expect(chat.getByRole("button", { name: /Open page/ })).toHaveCount(0);

    await expect(card.getByTestId("recording-panel")).toHaveCount(0);
    await chat.getByTestId("play-from").click();
    const panel = card.getByTestId("recording-panel");
    await expect(panel).toBeVisible();
    await expect.poll(async () => Math.round((await audioState(page, SAVED)).time)).toBeGreaterThanOrEqual(30);
    expect((await audioState(page, SAVED)).paused).toBe(false);
  });

  test("a document that was not a recording has no player, but its text can be read", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#main-textarea").fill("Notes from the call. Priya will send the budget by Friday.");
    await summarize(page);
    const card = page.getByTestId("document-card").first();
    await card.getByRole("button", { name: "More" }).click();
    await expect(card.getByRole("button", { name: "Play the recording" })).toHaveCount(0);
    await expect(card.getByRole("button", { name: "Show the original text" })).toBeVisible();
  });

  test("a PDF's citations still open the PDF viewer, not the player", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles({ name: "report.pdf", mimeType: "application/pdf", buffer: Buffer.from("%PDF-1.4 report") });
    await summarize(page);
    const card = page.getByTestId("document-card").first();
    await expect(card).toBeVisible();
    await card.getByRole("button", { name: "More" }).click();
    await expect(card.getByRole("button", { name: "Play the recording" })).toHaveCount(0);
  });

  test("the saved player has no accessibility violations", async ({ page }) => {
    const card = await saveRecording(page);
    await card.getByRole("button", { name: "Play the recording" }).click();
    await expect(card.getByTestId("audio-time")).toHaveText("0:00 / 1:00");
    const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
    expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  });
});
