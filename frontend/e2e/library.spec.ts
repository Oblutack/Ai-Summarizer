import { expect, test } from "@playwright/test";
import { newSignedInUser, realPdf, summarizeText } from "./support/helpers";

async function saveText(page: import("@playwright/test").Page, text: string, expectedCards: number) {
  await summarizeText(page, text);
  await expect(page.locator("h3")).toHaveCount(expectedCards);
}

test.describe("asking across all documents", () => {
  test("the library chat only appears once there is something saved", async ({ page }) => {
    await newSignedInUser(page);
    await expect(page.getByText("You have no saved documents yet.")).toBeVisible();
    await expect(page.getByTestId("library")).toHaveCount(0);

    await saveText(page, "A first document about bananas and potassium.", 1);
    await expect(page.getByTestId("library")).toBeVisible();
  });

  test("a question is answered from the matching document and cites it", async ({ page }) => {
    await newSignedInUser(page);
    await saveText(page, "Apples are crisp and grown in orchards across the valley.", 1);
    await saveText(page, "Bananas contain potassium and ripen quickly in warm kitchens.", 2);

    const library = page.getByTestId("library");
    await library.getByPlaceholder("Type a question").fill("How much potassium do bananas have?");
    await page.keyboard.press("Enter");

    await expect(library.getByText(/Stub library answer to "How much potassium do bananas have\?" from 1 passage/)).toBeVisible();
    await library.getByRole("button", { name: "Show source 1" }).click();
    // Only the banana document was searched and cited, and the source names it with its date.
    await expect(library.getByText("Bananas contain potassium and ripen quickly in warm kitchens.")).toBeVisible();
    await expect(library.getByText("Apples are crisp")).toHaveCount(0);
    await expect(library.getByTestId("sources")).toContainText(/Bananas contain potassium and ripen quickly in warm.* \(\d{1,2} \w{3} \d{4}\)/);
    // Pasted text has no original to open.
    await expect(library.getByRole("button", { name: /Open page/ })).toHaveCount(0);
  });

  test("when nothing matches, the answer says so", async ({ page }) => {
    await newSignedInUser(page);
    await saveText(page, "A document about gardening and compost heaps.", 1);

    const library = page.getByTestId("library");
    await library.getByPlaceholder("Type a question").fill("Explain quantum chromodynamics");
    await page.keyboard.press("Enter");
    await expect(library.getByText("I couldn't find anything about that in your saved documents.")).toBeVisible();
    await expect(library.getByTestId("sources")).toHaveCount(0);
  });

  test("a cited page of a PDF opens in the original from the library chat", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles({
      name: "handbook.pdf",
      mimeType: "application/pdf",
      buffer: realPdf(["First page of the handbook.", "Stub passage that the answer cites. More text follows."]),
    });
    await page.getByRole("button", { name: "Summarize", exact: true }).click();
    await expect(page.locator("h3", { hasText: "handbook.pdf" })).toBeVisible();

    const library = page.getByTestId("library");
    await library.getByPlaceholder("Type a question").fill("What does the stub passage say?");
    await page.keyboard.press("Enter");
    await library.getByRole("button", { name: "Show source 1" }).click();
    await expect(library.getByTestId("sources")).toContainText("handbook.pdf");
    await library.getByRole("button", { name: "Open page 2 in the document" }).click();

    const viewer = page.getByTestId("pdf-viewer");
    await expect(viewer.getByTestId("pdf-page")).toHaveText("Page 2 of 2");
    await expect(page.getByTestId("pdf-highlight").first()).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(viewer).toBeHidden();
  });

  test("one person's library never reaches into another's", async ({ page, browser }) => {
    await newSignedInUser(page, "alice");
    await saveText(page, "Alice's private notes about the zeppelin acquisition plans.", 1);

    const other = await browser.newContext();
    const bob = await other.newPage();
    await newSignedInUser(bob, "bob");
    await saveText(bob, "Bob keeps recipes for sourdough bread.", 1);
    const library = bob.getByTestId("library");
    await library.getByPlaceholder("Type a question").fill("zeppelin acquisition plans");
    await bob.keyboard.press("Enter");
    await expect(library.getByText("I couldn't find anything about that in your saved documents.")).toBeVisible();
    await other.close();
  });
});
