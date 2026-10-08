import { expect, test, type Page } from "@playwright/test";
import { newSignedInUser, summarizeText } from "./support/helpers";

// A collection is the set of documents that share a tag; the library chat can be pointed at one.

async function saveDoc(page: Page, text: string, expectedCards: number) {
  await summarizeText(page, text);
  await expect(page.getByTestId("document-card")).toHaveCount(expectedCards);
}

function cardOf(page: Page, title: string) {
  return page.getByTestId("document-card").filter({ has: page.getByRole("heading", { name: title }) });
}

async function tag(page: Page, title: string, name: string) {
  await cardOf(page, title).getByLabel("Add a tag").fill(name);
  await page.keyboard.press("Enter");
  await expect(cardOf(page, title).getByTestId("tags")).toContainText(name.toLowerCase());
}

async function ask(page: Page, question: string) {
  const library = page.getByTestId("library");
  await library.getByPlaceholder("Type a question").fill(question);
  await page.keyboard.press("Enter");
}

test.describe("asking a collection", () => {
  test("without tags the chat explains how to make a collection", async ({ page }) => {
    await newSignedInUser(page);
    await saveDoc(page, "Alpha notes\nBananas contain potassium.", 1);
    const library = page.getByTestId("library");
    await expect(library.getByText(/give your saved summaries tags/i)).toBeVisible();
    await expect(library.getByLabel("Search in")).toHaveCount(0);
  });

  test("a question can be asked of one tag's documents only", async ({ page }) => {
    await newSignedInUser(page);
    await saveDoc(page, "Alpha notes\nBananas contain potassium and ripen in warm kitchens.", 1);
    await saveDoc(page, "Bravo plans\nBananas are shipped green and ripened with gas.", 2);
    await saveDoc(page, "Charlie log\nBananas were also mentioned in the shipping log.", 3);
    await tag(page, "Alpha notes", "Fruit course");
    await tag(page, "Bravo plans", "Fruit course");

    const library = page.getByTestId("library");
    const scope = library.getByLabel("Search in");
    await expect(scope.locator("option")).toHaveText(["All your documents", "fruit course (2)"]);

    // Everything: all three documents are searched.
    await ask(page, "Where do bananas come from?");
    await expect(library.getByText(/from 3 passage/)).toBeVisible();

    // The collection: two documents, and the conversation starts again.
    await scope.selectOption("fruit course");
    await expect(library.getByRole("heading", { name: "Ask “fruit course”" })).toBeVisible();
    await expect(library.getByText("Ask about the 2 documents tagged “fruit course”...")).toBeVisible();
    await expect(library.getByText(/from 3 passage/)).toHaveCount(0);
    await ask(page, "Where do bananas come from?");
    await expect(library.getByText(/from 2 passage/)).toBeVisible();

    await scope.selectOption("");
    await expect(library.getByRole("heading", { name: "Ask All Your Documents" })).toBeVisible();
    await expect(library.getByText("Ask about anything you have saved...")).toBeVisible();
  });

  test("choosing a tag to filter the list points the question at that collection", async ({ page }) => {
    await newSignedInUser(page);
    await saveDoc(page, "Alpha notes\nBananas contain potassium.", 1);
    await saveDoc(page, "Bravo plans\nBananas are shipped green.", 2);
    await tag(page, "Bravo plans", "Shipping");

    await page.getByRole("group", { name: "Filter by tag" }).getByRole("button", { name: "shipping (1)" }).click();
    const library = page.getByTestId("library");
    await expect(library.getByLabel("Search in")).toHaveValue("shipping");

    await ask(page, "How are bananas shipped?");
    await expect(library.getByText(/from 1 passage/)).toBeVisible();

    await page.getByRole("group", { name: "Filter by tag" }).getByRole("button", { name: "All" }).click();
    await expect(library.getByLabel("Search in")).toHaveValue("");
  });

  test("a collection whose last tag is removed falls back to everything", async ({ page }) => {
    await newSignedInUser(page);
    await saveDoc(page, "Alpha notes\nBananas contain potassium.", 1);
    await tag(page, "Alpha notes", "Temp");
    const library = page.getByTestId("library");
    await library.getByLabel("Search in").selectOption("temp");

    await cardOf(page, "Alpha notes").getByRole("button", { name: "Remove the tag temp" }).click();
    await expect(library.getByLabel("Search in")).toHaveCount(0);
    await ask(page, "Do bananas contain potassium?");
    await expect(library.getByText(/from 1 passage/)).toBeVisible();
  });
});
