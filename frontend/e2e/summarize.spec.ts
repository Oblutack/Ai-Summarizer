import { expect, test } from "@playwright/test";
import { alertOf, newSignedInUser, pdf, summarizeText } from "./support/helpers";

const ARTICLE = "The council approved a new bike lane network funded by a regional grant. Work starts in March.";

test.describe("summarizing without an account", () => {
  test("pasted text streams into the output, then offers a PDF export", async ({ page }) => {
    await page.goto("/");
    await summarizeText(page, ARTICLE);

    await expect(page.getByRole("button", { name: "Cancel" })).toBeVisible();
    await expect(page.getByText("Stub summary")).toBeVisible();
    await expect(page.getByText("Opening words: The council approved a new bike lane")).toBeVisible();
    await expect(page.getByRole("button", { name: "Save as PDF" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Cancel" })).toHaveCount(0);
  });

  test("Save as PDF downloads a real PDF of the summary", async ({ page }) => {
    await page.goto("/");
    await summarizeText(page, ARTICLE);
    await expect(page.getByRole("button", { name: "Save as PDF" })).toBeVisible();

    const download = page.waitForEvent("download");
    await page.getByRole("button", { name: "Save as PDF" }).click();
    const file = await download;
    expect(file.suggestedFilename()).toMatch(/-summary\.pdf$/);

    const { readFile } = await import("node:fs/promises");
    const bytes = await readFile((await file.path())!);
    expect(bytes.subarray(0, 5).toString("latin1")).toBe("%PDF-");
    expect(bytes.length).toBeGreaterThan(1000);
  });

  test("an uploaded PDF is summarized, and the page limit appears", async ({ page }) => {
    await page.goto("/");
    await page.locator("#pdf-upload").setInputFiles(pdf("annual-report.pdf"));
    await expect(page.getByText("annual-report.pdf")).toBeVisible();
    await expect(page.getByRole("spinbutton", { name: "Page limit" })).toBeVisible();

    await page.getByRole("button", { name: "Summarize", exact: true }).click();
    await expect(page.getByText("Subject: annual-report.pdf")).toBeVisible();
  });

  test("several PDFs are combined into one summary", async ({ page }) => {
    await page.goto("/");
    await page.locator("#pdf-upload").setInputFiles([pdf("a.pdf"), pdf("b.pdf")]);
    await page.getByRole("button", { name: "Summarize", exact: true }).click();
    await expect(page.getByText("Subject: a.pdf, b.pdf")).toBeVisible();
  });

  test("a file removed from the list is not sent", async ({ page }) => {
    await page.goto("/");
    await page.locator("#pdf-upload").setInputFiles([pdf("keep.pdf"), pdf("drop.pdf")]);
    await page.getByRole("button", { name: "Remove drop.pdf" }).click();
    await page.getByRole("button", { name: "Summarize", exact: true }).click();
    await expect(page.getByText("Subject: keep.pdf")).toBeVisible();
    await expect(page.getByText(/drop\.pdf/)).toHaveCount(0);
  });

  test("a model failure is explained and the form stays usable", async ({ page }) => {
    await page.goto("/");
    await summarizeText(page, `${ARTICLE} FAIL_ME`);
    await expect(page.getByText(/language model is unavailable/i)).toBeVisible();
    await expect(page.getByRole("button", { name: "Summarize", exact: true })).toBeEnabled();

    // Editing the text and trying again works.
    await summarizeText(page, ARTICLE);
    await expect(page.getByText("Stub summary")).toBeVisible();
  });

  test("Cancel stops a running summary", async ({ page }) => {
    await page.goto("/");
    await summarizeText(page, `${ARTICLE} SLOW_ME`);
    await page.getByRole("button", { name: "Cancel" }).click();
    await expect(page.getByRole("button", { name: "Summarize", exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Cancel" })).toHaveCount(0);
    // The user is told it was cancelled, rather than being left with an empty box.
    await expect(alertOf(page)).toHaveText("Summarization was cancelled.");
  });

  test("the word-count slider is replaced by a page limit once a PDF is attached", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator("#word-count")).toBeEnabled();
    await page.locator("#pdf-upload").setInputFiles(pdf("x.pdf"));
    await page.getByRole("button", { name: "Increase page limit" }).click();
    await expect(page.locator("#word-count")).toBeDisabled();
  });
});

test.describe("summarizing while signed in", () => {
  test("a summary is saved to the history, survives a reload, and can be deleted", async ({ page }) => {
    await newSignedInUser(page);
    await expect(page.getByText("You have no saved documents yet.")).toBeVisible();

    await page.locator("#pdf-upload").setInputFiles(pdf("quarterly.pdf"));
    await page.getByRole("button", { name: "Summarize", exact: true }).click();
    await expect(page.getByText("Subject: quarterly.pdf").first()).toBeVisible();

    const card = page.locator("h3", { hasText: "quarterly.pdf" });
    await expect(card).toBeVisible();

    await page.reload();
    await expect(page.locator("h3", { hasText: "quarterly.pdf" })).toBeVisible();

    // A saved summary can be exported too.
    const download = page.waitForEvent("download");
    await page.getByRole("button", { name: "Save quarterly.pdf as PDF" }).click();
    expect((await download).suggestedFilename()).toMatch(/\.pdf$/);

    page.once("dialog", (dialog) => dialog.accept());
    await page.getByRole("button", { name: "Delete quarterly.pdf" }).click();
    await expect(page.locator("h3", { hasText: "quarterly.pdf" })).toHaveCount(0);
    await page.reload();
    await expect(page.getByText("You have no saved documents yet.")).toBeVisible();
  });

  test("chat answers questions about a saved document", async ({ page }) => {
    await newSignedInUser(page);
    await summarizeTextAndWaitForCard(page);

    await page.getByRole("button", { name: "Chat With Document" }).first().click();
    await page.getByPlaceholder("Type a question").fill("When does work start?");
    await page.keyboard.press("Enter");

    await expect(page.getByText("When does work start?").first()).toBeVisible();
    await expect(page.getByText(/Stub answer to "When does work start\?"/)).toBeVisible();

    // A second question keeps the conversation going.
    await page.getByPlaceholder("Type a question").fill("And who pays?");
    await page.keyboard.press("Enter");
    await expect(page.getByText(/Stub answer to "And who pays\?"/)).toBeVisible();
  });

  test("answers cite their sources: a chip opens the passage and its page", async ({ page }) => {
    await newSignedInUser(page);
    await summarizeTextAndWaitForCard(page);
    await page.getByRole("button", { name: "Chat With Document" }).first().click();
    await page.getByPlaceholder("Type a question").fill("When does work start?");
    await page.keyboard.press("Enter");

    // The answer carries a clickable marker, and the source list starts collapsed.
    const chip = page.getByRole("button", { name: "Show source 1" });
    await expect(chip).toBeVisible();
    const sources = page.getByTestId("sources");
    await expect(sources).toContainText("Page 2");
    await expect(page.getByText("Stub passage that the answer cites.")).toBeHidden();

    await chip.click();
    await expect(page.getByText("Stub passage that the answer cites.")).toBeVisible();

    // It can be closed again from the list itself.
    await sources.getByText("Page 2").click();
    await expect(page.getByText("Stub passage that the answer cites.")).toBeHidden();
  });

  test("a citation marker that matches no source is never turned into a link", async ({ page }) => {
    await newSignedInUser(page);
    await summarizeTextAndWaitForCard(page);
    await page.getByRole("button", { name: "Chat With Document" }).first().click();
    await page.getByPlaceholder("Type a question").fill("INVENTED reference please");
    await page.keyboard.press("Enter");

    await expect(page.getByRole("button", { name: "Show source 1" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Show source 9" })).toHaveCount(0);
    await expect(page.getByText("[9]")).toBeVisible();
  });

  test("an answer without citations shows no source list", async ({ page }) => {
    await newSignedInUser(page);
    await summarizeTextAndWaitForCard(page);
    await page.getByRole("button", { name: "Chat With Document" }).first().click();
    await page.getByPlaceholder("Type a question").fill("NOCITE what is this?");
    await page.keyboard.press("Enter");

    await expect(page.getByText(/Stub answer to "NOCITE what is this\?"/)).toBeVisible();
    await expect(page.getByTestId("sources")).toHaveCount(0);
    await expect(page.getByRole("button", { name: /Show source/ })).toHaveCount(0);
  });

  test("history shows newest first", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(pdf("first.pdf"));
    await page.getByRole("button", { name: "Summarize", exact: true }).click();
    await expect(page.locator("h3", { hasText: "first.pdf" })).toBeVisible();

    await page.getByRole("button", { name: "Remove first.pdf" }).click();
    await page.locator("#pdf-upload").setInputFiles(pdf("second.pdf"));
    await page.getByRole("button", { name: "Summarize", exact: true }).click();
    await expect(page.locator("h3", { hasText: "second.pdf" })).toBeVisible();

    const titles = await page.locator("h3").allTextContents();
    expect(titles.findIndex((t) => t.includes("second.pdf"))).toBeLessThan(
      titles.findIndex((t) => t.includes("first.pdf"))
    );
  });
});

async function summarizeTextAndWaitForCard(page: import("@playwright/test").Page) {
  await page.locator("#main-textarea").fill(ARTICLE);
  await page.getByRole("button", { name: "Summarize", exact: true }).click();
  await expect(page.locator("h3").first()).toBeVisible();
}
