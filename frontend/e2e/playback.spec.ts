import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { newSignedInUser, summarizeText } from "./support/helpers";
import { installFakeSpeech, spoken } from "./support/speech";

// The player that reads a summary aloud and the one that plays a podcast share their controls and their settings.

// The stub AI service writes a summary of its own ("Stub summary", then lines about the subject and the opening words).
const SUMMARY = "Notes from the weekly meeting about the budget, the venue, the launch and the risks.";

async function openReader(page: Page) {
  await newSignedInUser(page);
  await summarizeText(page, SUMMARY);
  const card = page.getByTestId("document-card").first();
  await expect(card).toBeVisible();
  await card.getByRole("button", { name: "More" }).click();
  await card.getByRole("button", { name: "Read aloud" }).click();
  const reader = page.getByTestId("read-aloud");
  await expect(reader).toBeVisible();
  return { card, reader };
}

const speech = (page: Page) =>
  page.evaluate(() => (window as unknown as { __speech: { paused: number; resumed: number } }).__speech);

test.describe("reading a summary aloud", () => {
  test("it starts by itself and shows where it is, what it is reading, and how long is left", async ({ page }) => {
    await installFakeSpeech(page, { delay: 3000 });
    const { reader } = await openReader(page);

    await expect(reader.getByTestId("playback-progress")).toHaveText(/^1 \/ \d+$/);
    await expect(reader.getByTestId("now-reading")).toContainText("Stub summary");
    await expect(reader.getByTestId("playback-left")).toHaveText(/less than a minute left|about \d+ min left/);
    await expect(reader.getByRole("button", { name: "Pause" })).toBeVisible();
    const slider = reader.getByTestId("playback-position");
    await expect(slider).toHaveAttribute("min", "0");
    await expect(slider).toHaveAttribute("aria-valuetext", /^Part 1 of \d+$/);
  });

  test("it can be paused and resumed", async ({ page }) => {
    await installFakeSpeech(page, { delay: 3000 });
    const { reader } = await openReader(page);

    await reader.getByRole("button", { name: "Pause" }).click();
    await expect(reader.getByRole("button", { name: "Resume" })).toBeVisible();
    await reader.getByRole("button", { name: "Resume" }).click();
    await expect(reader.getByRole("button", { name: "Pause" })).toBeVisible();
    expect(await speech(page)).toMatchObject({ paused: 1, resumed: 1 });
  });

  test("next and previous move between the paragraphs", async ({ page }) => {
    await installFakeSpeech(page, { delay: 1500 });
    const { reader } = await openReader(page);
    const progress = reader.getByTestId("playback-progress");
    await expect(progress).toHaveText(/^1 \/ /);

    await reader.getByRole("button", { name: "Next", exact: true }).click();
    await expect(progress).toHaveText(/^2 \/ /);
    await reader.getByRole("button", { name: "Next", exact: true }).click();
    await expect(progress).toHaveText(/^3 \/ /);
    await reader.getByRole("button", { name: "Previous", exact: true }).click();
    await expect(progress).toHaveText(/^2 \/ /);

    const said = (await spoken(page)).map((s) => s.text);
    expect(said.some((text) => text.includes("Subject: pasted text"))).toBe(true);
    expect(said.some((text) => text.includes("Opening words"))).toBe(true);
  });

  test("the slider jumps to a paragraph", async ({ page }) => {
    await installFakeSpeech(page, { delay: 1500 });
    const { reader } = await openReader(page);
    const slider = reader.getByTestId("playback-position");
    await slider.focus();
    await page.keyboard.press("ArrowRight");
    await page.keyboard.press("ArrowRight");
    await expect(reader.getByTestId("playback-progress")).toHaveText(/^3 \/ /);
    await expect(reader.getByTestId("now-reading")).toContainText("Opening words");
  });

  test("a new speed is heard at once, and is remembered for next time and for the podcast", async ({ page }) => {
    await installFakeSpeech(page, { delay: 3000 });
    const { reader } = await openReader(page);

    const before = (await spoken(page)).length;
    await reader.getByLabel("Speed").selectOption("2");
    // the piece being read starts again at the new speed, without waiting for the next one
    await expect.poll(async () => (await spoken(page)).slice(before).some((s) => s.rate === 2)).toBe(true);

    await page.reload();
    await expect(page.getByTestId("document-card").first()).toBeVisible();
    const card = page.getByTestId("document-card").first();
    await card.getByRole("button", { name: "More" }).click();
    await card.getByRole("button", { name: "Read aloud" }).click();
    await expect(page.getByTestId("read-aloud").getByLabel("Speed")).toHaveValue("2");

    await page.getByRole("button", { name: "Listen as a podcast" }).click();
    await expect(page.getByTestId("podcast-title")).toBeVisible();
    await expect(page.getByTestId("podcast").getByLabel("Speed")).toHaveValue("2");
  });

  test("the voice that was chosen is remembered", async ({ page }) => {
    await installFakeSpeech(page, { delay: 300 });
    const { reader } = await openReader(page);
    await reader.getByLabel("Voice").selectOption("fake-sam");
    await page.reload();
    const card = page.getByTestId("document-card").first();
    await card.getByRole("button", { name: "More" }).click();
    await card.getByRole("button", { name: "Read aloud" }).click();
    await expect(page.getByTestId("read-aloud").getByLabel("Voice")).toHaveValue("fake-sam");
    await expect.poll(async () => (await spoken(page)).some((s) => s.voice === "fake-sam")).toBe(true);
  });

  test("closing the reader stops the voice", async ({ page }) => {
    await installFakeSpeech(page, { delay: 300 });
    const { reader } = await openReader(page);
    await reader.getByRole("button", { name: "Close the reader" }).click();
    await expect(page.getByTestId("read-aloud")).toHaveCount(0);
    const count = (await spoken(page)).length;
    await page.waitForTimeout(900);
    expect((await spoken(page)).length).toBe(count);
  });

  test("the reader has no accessibility violations", async ({ page }) => {
    await installFakeSpeech(page, { delay: 3000 });
    await openReader(page);
    const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
    expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  });
});

test.describe("the podcast player", () => {
  async function openPodcast(page: Page) {
    await newSignedInUser(page);
    await summarizeText(page, "A short document that two hosts will talk about.");
    await page.getByRole("button", { name: "Listen as a podcast" }).click();
    await expect(page.getByTestId("podcast-title")).toBeVisible();
    return page.getByTestId("podcast");
  }

  test("it has the same skip, slider and time-left controls", async ({ page }) => {
    await installFakeSpeech(page, { delay: 1500 });
    const podcast = await openPodcast(page);
    await expect(podcast.getByTestId("playback-progress")).toHaveText("1 / 6");

    await podcast.getByRole("button", { name: "Play", exact: true }).click();
    await expect(podcast.getByTestId("podcast-turn-0")).toHaveAttribute("data-active", "true");
    await expect(podcast.getByTestId("playback-left")).toBeVisible();

    await podcast.getByRole("button", { name: "Next", exact: true }).click();
    await expect(podcast.getByTestId("podcast-turn-1")).toHaveAttribute("data-active", "true");
    await expect(podcast.getByTestId("playback-progress")).toHaveText("2 / 6");
    await podcast.getByRole("button", { name: "Previous", exact: true }).click();
    await expect(podcast.getByTestId("podcast-turn-0")).toHaveAttribute("data-active", "true");

    await podcast.getByTestId("playback-position").focus();
    await page.keyboard.press("ArrowRight");
    await page.keyboard.press("ArrowRight");
    await page.keyboard.press("ArrowRight");
    await expect(podcast.getByTestId("podcast-turn-3")).toHaveAttribute("data-active", "true");
  });

  test("the current line is marked for assistive technology too", async ({ page }) => {
    await installFakeSpeech(page, { delay: 1500 });
    const podcast = await openPodcast(page);
    await podcast.getByRole("button", { name: "Play", exact: true }).click();
    await expect(podcast.getByTestId("podcast-turn-0")).toHaveAttribute("aria-current", "true");
    await expect(podcast.getByTestId("podcast-turn-1")).not.toHaveAttribute("aria-current", "true");
  });

  test("the voices chosen for the hosts are remembered", async ({ page }) => {
    await installFakeSpeech(page, { delay: 200 });
    const podcast = await openPodcast(page);
    await podcast.getByLabel("Alex's voice").selectOption("fake-sam");
    await podcast.getByLabel("Sam's voice").selectOption("fake-alex");
    await page.reload();
    await page.getByRole("button", { name: "Listen as a podcast" }).click();
    await expect(page.getByTestId("podcast-title")).toBeVisible();
    await expect(page.getByTestId("podcast").getByLabel("Alex's voice")).toHaveValue("fake-sam");
    await expect(page.getByTestId("podcast").getByLabel("Sam's voice")).toHaveValue("fake-alex");
  });
});
