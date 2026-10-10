import { expect, test, type Page } from "@playwright/test";
import { installFakeSpeech, spoken } from "./support/speech";
import { newSignedInUser, summarizeText } from "./support/helpers";

async function openPodcast(page: Page) {
  await newSignedInUser(page);
  await summarizeText(page, "A short document that two hosts will talk about.");
  await expect(page.locator("h3").first()).toBeVisible();
  await page.getByRole("button", { name: "Listen as a podcast" }).click();
  await expect(page.getByTestId("podcast-title")).toBeVisible();
}

test.describe("podcast mode", () => {
  test("a script is written, shown, and read aloud by two different voices", async ({ page }) => {
    // Slow enough that the highlight on each line lasts long enough to be seen.
    await installFakeSpeech(page, { delay: 250 });
    await openPodcast(page);

    await expect(page.getByTestId("podcast-title")).toHaveText("Stub episode about the document");
    await expect(page.locator("[data-testid^=podcast-turn-]")).toHaveCount(6);

    await page.getByRole("button", { name: "Play", exact: true }).click();
    // Each line becomes active as it is spoken, and the play button returns at the end.
    await expect(page.getByTestId("podcast-turn-0")).toHaveAttribute("data-active", "true");
    await expect(page.getByRole("button", { name: "Play", exact: true })).toBeVisible({ timeout: 10_000 });

    const lines = await spoken(page);
    expect(lines.map((l) => l.text)).toEqual([
      "So what is this document about?",
      "It is a stub document used in tests.",
      "What is the main point?",
      "That the player reads every line in order.",
      "Anything to remember?",
      "Yes: two voices, one conversation.",
    ]);
    // Host A and host B are given different voices.
    expect(lines.filter((_, i) => i % 2 === 0).every((l) => l.voice === "fake-alex")).toBe(true);
    expect(lines.filter((_, i) => i % 2 === 1).every((l) => l.voice === "fake-sam")).toBe(true);
  });

  test("it can be paused, resumed and stopped", async ({ page }) => {
    await installFakeSpeech(page, { delay: 3000 });
    await openPodcast(page);

    await page.getByRole("button", { name: "Play", exact: true }).click();
    await page.getByRole("button", { name: "Pause" }).click();
    await expect(page.getByRole("button", { name: "Resume" })).toBeVisible();
    await page.getByRole("button", { name: "Resume" }).click();
    await expect(page.getByRole("button", { name: "Pause" })).toBeVisible();
    expect(await page.evaluate(() => (window as unknown as { __speech: { paused: number; resumed: number } }).__speech)).toMatchObject({
      paused: 1,
      resumed: 1,
    });

    await page.getByRole("button", { name: "Stop" }).click();
    await expect(page.getByRole("button", { name: "Play", exact: true })).toBeVisible();
    await expect(page.locator('[data-active="true"]')).toHaveCount(0);
    const before = (await spoken(page)).length;
    await page.waitForTimeout(900);
    expect((await spoken(page)).length, "nothing is spoken after stopping").toBe(before);
  });

  test("clicking a line plays from there, at the chosen speed", async ({ page }) => {
    await installFakeSpeech(page, { delay: 3000 });
    await openPodcast(page);

    await page.getByTestId("podcast").getByLabel("Speed").selectOption("1.25");
    await page.getByTestId("podcast-turn-3").getByRole("button").click();
    await expect(page.getByTestId("podcast-turn-3")).toHaveAttribute("data-active", "true");
    const first = (await spoken(page))[0];
    expect(first.text).toBe("That the player reads every line in order.");
    await page.getByRole("button", { name: "Stop" }).click();
    expect(await page.evaluate(() => (window as unknown as { __spoken: { rate: number }[] }).__spoken[0].rate)).toBe(1.25);
  });

  test("the script is kept: opening it again does not write a new one", async ({ page }) => {
    await installFakeSpeech(page);
    const responses: { cached: boolean }[] = [];
    page.on("response", async (r) => {
      if (r.url().includes("/podcast") && r.request().method() === "POST") responses.push(await r.json());
    });
    await openPodcast(page);
    await page.getByRole("button", { name: "Close podcast" }).click();
    await page.getByRole("button", { name: "Listen as a podcast" }).click();
    await expect(page.getByTestId("podcast-title")).toBeVisible();

    expect(responses.map((r) => r.cached)).toEqual([false, true]);
  });

  test("a new one can be written in another language", async ({ page }) => {
    await installFakeSpeech(page);
    await openPodcast(page);
    await page.getByTestId("podcast").getByLabel("Language").selectOption("Spanish");
    await page.getByRole("button", { name: "Write a new one" }).click();
    await expect(page.getByTestId("podcast-title")).toHaveText("Stub episode about the document (Spanish)");
  });

  test("without speech voices the script can still be read and saved", async ({ page }) => {
    await installFakeSpeech(page, { voices: false });
    await openPodcast(page);

    await expect(page.getByRole("status").filter({ hasText: "no speech voices" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Play", exact: true })).toBeDisabled();
    await expect(page.locator("[data-testid^=podcast-turn-]")).toHaveCount(6);

    const download = page.waitForEvent("download");
    await page.getByRole("button", { name: "Save script" }).click();
    const file = await download;
    expect(file.suggestedFilename()).toMatch(/\.md$/);
    const { readFile } = await import("node:fs/promises");
    const text = await readFile((await file.path())!, "utf8");
    expect(text).toContain("# Stub episode about the document");
    expect(text).toContain("**Alex:** So what is this document about?");
    expect(text).toContain("**Sam:** It is a stub document used in tests.");
  });

  test("when the allowance is used up the user is told, and nothing breaks", async ({ page }) => {
    await installFakeSpeech(page);
    await newSignedInUser(page);
    await summarizeText(page, "A short document that two hosts will talk about.");
    await expect(page.locator("h3").first()).toBeVisible();
    await page.route("**/documents/*/podcast", (route) =>
      route.fulfill({ status: 429, json: { error: "You've reached today's limit (200). It resets at midnight UTC.", code: "quota_exceeded" } })
    );
    await page.getByRole("button", { name: "Listen as a podcast" }).click();
    await expect(page.getByRole("alert").filter({ hasText: "today's limit" })).toBeVisible();
    await expect(page.getByTestId("podcast-title")).toHaveCount(0);
  });
});
