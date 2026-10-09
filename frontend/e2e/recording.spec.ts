import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { alertOf, newSignedInUser } from "./support/helpers";

// Recording a meeting from the browser. The test browser has a fake microphone (see playwright.config.ts).

const summarize = (page: Page) => page.getByRole("button", { name: "Summarize", exact: true }).click();

test.describe("recording in the browser", () => {
  test("the page lets itself use the microphone, and nothing else may, nor the camera or location", async ({ page }) => {
    const response = await page.goto("/login");
    expect(response?.headers()["permissions-policy"]).toBe("camera=(), microphone=(self), geolocation=()");
  });

  test("a recording is made, kept as a file, and summarized like an uploaded one", async ({ page }) => {
    await newSignedInUser(page);
    await page.getByTestId("record-button").click();
    await expect(page.getByTestId("recorder")).toBeVisible();
    await expect(page.getByRole("status").filter({ hasText: "Recording from the microphone" })).toBeAttached();

    // the clock runs
    await expect.poll(async () => (await page.getByTestId("recording-time").textContent()) !== "0:00").toBe(true);
    await page.getByRole("button", { name: "Stop and use the recording" }).click();

    // it is now an attached file, with a name made from the date and time
    await expect(page.getByTestId("recorder")).toHaveCount(0);
    const file = page.getByText(/^recording \d{4}-\d{2}-\d{2} \d{2}\.\d{2}\.webm$/);
    await expect(file).toBeVisible();
    await expect(page.getByTestId("audio-hint")).toBeVisible();

    await summarize(page);
    await expect(page.getByText(/Subject: recording \d{4}-\d{2}-\d{2} \d{2}\.\d{2}\.webm/)).toBeVisible();
    const card = page.getByTestId("document-card").first();
    await expect(card).toBeVisible();
    await card.getByRole("button", { name: "More" }).click();
    await card.getByRole("button", { name: "Play the recording" }).click();
    await expect(card.getByTestId("original-text-body")).toContainText("The stub transcript of recording");
  });

  test("a recording can be thrown away and nothing is attached", async ({ page }) => {
    await newSignedInUser(page);
    await page.getByTestId("record-button").click();
    await expect(page.getByTestId("recorder")).toBeVisible();
    await page.waitForTimeout(1200);
    await page.getByRole("button", { name: "Discard" }).click();
    await expect(page.getByTestId("recorder")).toHaveCount(0);
    await expect(page.getByTestId("record-button")).toBeVisible();
    await expect(page.getByTestId("audio-hint")).toHaveCount(0);
    await expect(page.getByRole("button", { name: /^Remove / })).toHaveCount(0);
  });

  test("the microphone is only offered to someone who is signed in", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator("#pdf-upload")).toBeAttached();
    await expect(page.getByTestId("record-button")).toHaveCount(0);
  });

  test("a blocked microphone is explained", async ({ page }) => {
    await page.addInitScript(() => {
      navigator.mediaDevices.getUserMedia = () => Promise.reject(new DOMException("denied", "NotAllowedError"));
    });
    await newSignedInUser(page);
    await page.getByTestId("record-button").click();
    await expect(alertOf(page)).toContainText("The microphone is blocked");
    await expect(page.getByTestId("record-button")).toBeVisible(); // it can be tried again
  });

  test("a browser that cannot record does not show the button", async ({ page }) => {
    await page.addInitScript(() => {
      // @ts-expect-error removing the API on purpose
      delete window.MediaRecorder;
    });
    await newSignedInUser(page);
    await expect(page.locator("#pdf-upload")).toBeAttached();
    await expect(page.getByTestId("record-button")).toHaveCount(0);
  });

  test("a recorder that fails to start says so and leaves the microphone free", async ({ page }) => {
    await page.addInitScript(() => {
      window.MediaRecorder = class {
        static isTypeSupported() {
          return true;
        }
        constructor() {
          throw new Error("not supported");
        }
      } as unknown as typeof MediaRecorder;
    });
    await newSignedInUser(page);
    await page.getByTestId("record-button").click();
    await expect(alertOf(page)).toContainText("Recording does not work in this browser");
  });

  test("a recording and a file can be combined", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles({ name: "agenda.pdf", mimeType: "application/pdf", buffer: Buffer.from("%PDF-1.4 agenda") });
    await page.getByTestId("record-button").click();
    await page.waitForTimeout(1200);
    await page.getByRole("button", { name: "Stop and use the recording" }).click();
    await expect(page.getByText("agenda.pdf")).toBeVisible();
    await expect(page.getByText(/^recording \d{4}/)).toBeVisible();
  });

  test("the recorder has no accessibility violations, idle or recording", async ({ page }) => {
    await newSignedInUser(page);
    const check = async () =>
      (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze()).violations.map((v) => `${v.id}: ${v.help}`);
    expect(await check()).toEqual([]);
    await page.getByTestId("record-button").click();
    await expect(page.getByTestId("recorder")).toBeVisible();
    expect(await check()).toEqual([]);
    await page.getByRole("button", { name: "Discard" }).click();
  });
});
