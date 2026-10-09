import { expect, test } from "@playwright/test";
import { alertOf, newSignedInUser, pdf } from "./support/helpers";

const docx = (name: string) => ({
  name,
  mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  buffer: Buffer.from(`PK pretend ${name}`),
});
const pptx = (name: string) => ({ name, mimeType: "application/octet-stream", buffer: Buffer.from(`PK pretend ${name}`) });

const summarize = (page: import("@playwright/test").Page) => page.getByRole("button", { name: "Summarize", exact: true }).click();

test.describe("web links", () => {
  test("a web address in the box is recognised, and the page is what gets summarized", async ({ page }) => {
    await page.goto("/");
    await page.locator("#main-textarea").fill("  https://news.example/long-read ");
    await expect(page.getByTestId("link-detected")).toHaveText("Web link found. Inkling will read the page when you press Summarize.");

    await summarize(page);
    await expect(page.getByText("Subject: Page from news.example")).toBeVisible();
    await expect(page.getByTestId("summary-length")).toBeVisible();
  });

  test("a sentence that contains an address is ordinary text", async ({ page }) => {
    await page.goto("/");
    await page.locator("#main-textarea").fill("Read https://news.example/long-read before the meeting, it explains everything.");
    await expect(page.getByTestId("link-detected")).toHaveCount(0);

    await summarize(page);
    await expect(page.getByText("Subject: pasted text")).toBeVisible();
  });

  test("an address that cannot be read is explained, and the box stays usable", async ({ page }) => {
    await page.goto("/");
    await page.locator("#main-textarea").fill("http://private.example/admin");
    await summarize(page);
    await expect(alertOf(page)).toContainText("not on the public internet");
    await expect(page.locator("#main-textarea")).toHaveValue("http://private.example/admin");
    await expect(page.getByRole("button", { name: "Summarize", exact: true })).toBeEnabled();

    await page.locator("#main-textarea").fill("https://empty.example/app");
    await summarize(page);
    await expect(alertOf(page)).toContainText("could not find readable text");
  });

  test("a signed-in person's link is saved under the page title and can be chatted with", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#main-textarea").fill("https://news.example/long-read");
    await summarize(page);

    const card = page.getByTestId("document-card");
    await expect(card).toHaveCount(1);
    await expect(card.getByRole("heading", { name: "Page from news.example" })).toBeVisible();
    // The page text was kept, so the document tools are there (a link has no original file to open).
    await expect(card.getByRole("button", { name: "Chat With Document" })).toBeVisible();
    await expect(card.getByRole("button", { name: "Check against the original" })).toBeVisible();
  });

  test("cancelling a slow link stops it", async ({ page }) => {
    await page.goto("/");
    await page.locator("#main-textarea").fill("https://slow.example/very-long");
    await summarize(page);
    await page.getByRole("button", { name: "Cancel" }).click();
    await expect(alertOf(page)).toHaveText("Summarization was cancelled.");
  });
});

test.describe("Word and PowerPoint files", () => {
  test("a Word file is attached and summarized", async ({ page }) => {
    await page.goto("/");
    await page.locator("#pdf-upload").setInputFiles(docx("lease-agreement.docx"));
    await expect(page.getByText("lease-agreement.docx")).toBeVisible();
    await summarize(page);
    await expect(page.getByText("Subject: lease-agreement.docx")).toBeVisible();
  });

  test("a PDF, a Word file and a presentation can be combined", async ({ page }) => {
    await page.goto("/");
    await page.locator("#pdf-upload").setInputFiles([pdf("a.pdf"), docx("b.docx"), pptx("c.pptx")]);
    await summarize(page);
    await expect(page.getByText("Subject: a.pdf, b.docx, c.pptx")).toBeVisible();
  });

  test("other file types are turned away with the reason, and the good ones stay", async ({ page }) => {
    await page.goto("/");
    await page.locator("#pdf-upload").setInputFiles([
      { name: "notes.txt", mimeType: "text/plain", buffer: Buffer.from("hello") },
      docx("kept.docx"),
    ]);
    await expect(alertOf(page)).toHaveText("Only PDF, Word (.docx), PowerPoint (.pptx), audio files and photos (JPG, PNG, WebP) are supported.");
    await expect(page.getByText("kept.docx")).toBeVisible();
    await expect(page.getByText("notes.txt")).toHaveCount(0);
  });

  test("the file picker offers documents, recordings and photos", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator("#pdf-upload")).toHaveAttribute("accept", ".pdf,.docx,.pptx,.mp3,.mpga,.mpeg,.m4a,.mp4,.wav,.ogg,.flac,.webm,.jpg,.jpeg,.png,.webp");
  });
});
