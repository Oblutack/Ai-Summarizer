import { expect, test, type Page } from "@playwright/test";
import { alertOf, newSignedInUser, summarizeText } from "./support/helpers";

// One briefing on a whole collection (the documents that share a tag).

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

// A signed-in person with two documents in the collection "course".
async function withCourse(page: Page, name = "course") {
  await newSignedInUser(page);
  await saveDoc(page, "Alpha lecture\nCells have a nucleus.", 1);
  await saveDoc(page, "Bravo lecture\nPlants make glucose.", 2);
  await tag(page, "Alpha lecture", name);
  await tag(page, "Bravo lecture", name);
  const library = page.getByTestId("library");
  await library.getByLabel("Search in").selectOption(name);
  return library;
}

test.describe("the overview of a collection", () => {
  test("it is offered only for a collection of at least two documents", async ({ page }) => {
    await newSignedInUser(page);
    await saveDoc(page, "Alpha lecture\nCells have a nucleus.", 1);
    await tag(page, "Alpha lecture", "solo");
    const library = page.getByTestId("library");
    await expect(library.getByTestId("overview")).toHaveCount(0);

    await library.getByLabel("Search in").selectOption("solo");
    await expect(library.getByTestId("overview")).toHaveCount(0);

    await saveDoc(page, "Bravo lecture\nPlants make glucose.", 2);
    await tag(page, "Bravo lecture", "solo");
    await expect(library.getByTestId("overview")).toBeVisible();
  });

  test("it is written from the collection's documents and can be copied", async ({ page, context }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
    const library = await withCourse(page);
    await expect(library.getByText("One briefing on all 2 documents")).toBeVisible();

    await library.getByRole("button", { name: "Write an overview" }).click();
    const text = library.getByTestId("overview-text");
    await expect(text.getByRole("heading", { name: "Stub overview" })).toBeVisible();
    await expect(text).toContainText("Collection: course");
    await expect(text).toContainText("Documents: 2");
    await expect(text).toContainText("Alpha lecture / Bravo lecture");
    await expect(library.getByRole("heading", { name: "Overview of “course”" })).toBeVisible();
    await expect(library.getByText("Written from the saved summaries of these 2 documents.")).toBeVisible();

    await library.getByTestId("overview").getByRole("button", { name: /Copy/ }).click();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toContain("Documents: 2");

    // It is shown, not saved: the list of saved summaries is unchanged.
    await expect(page.getByTestId("document-card")).toHaveCount(2);
    await page.reload();
    await expect(page.getByTestId("library").getByTestId("overview-text")).toHaveCount(0);
  });

  test("it can be written again, and uses the chosen style and language", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#summary-style").selectOption({ label: "Bullet Points" });
    await page.locator("#summary-language").selectOption({ label: "Spanish" });
    await saveDoc(page, "Alpha lecture\nCells have a nucleus.", 1);
    await saveDoc(page, "Bravo lecture\nPlants make glucose.", 2);
    await tag(page, "Alpha lecture", "course");
    await tag(page, "Bravo lecture", "course");
    const library = page.getByTestId("library");
    await library.getByLabel("Search in").selectOption("course");

    await library.getByRole("button", { name: "Write an overview" }).click();
    await expect(library.getByTestId("overview-text")).toContainText("Options: bullets, 300 words, Spanish");
    await expect(library.getByRole("button", { name: "Write it again" })).toBeVisible();
    await library.getByRole("button", { name: "Write it again" }).click();
    await expect(library.getByTestId("overview-text")).toContainText("Documents: 2");
  });

  test("changing the collection leaves the old overview behind", async ({ page }) => {
    const library = await withCourse(page);
    await library.getByRole("button", { name: "Write an overview" }).click();
    await expect(library.getByTestId("overview-text")).toContainText("Documents: 2");

    await library.getByLabel("Search in").selectOption("");
    await expect(library.getByTestId("overview")).toHaveCount(0);
    await library.getByLabel("Search in").selectOption("course");
    await expect(library.getByTestId("overview-text")).toHaveCount(0);
    await expect(library.getByRole("button", { name: "Write an overview" })).toBeVisible();
  });

  test("a failure is explained and it can be tried again", async ({ page }) => {
    const library = await withCourse(page, "failing course");
    await library.getByRole("button", { name: "Write an overview" }).click();
    await expect(alertOf(page)).toContainText("language model is unavailable");
    await expect(library.getByRole("button", { name: "Write an overview" })).toBeEnabled();
  });

  test("a slow overview can be cancelled", async ({ page }) => {
    const library = await withCourse(page, "slow course");
    await library.getByRole("button", { name: "Write an overview" }).click();
    await library.getByRole("button", { name: "Cancel" }).click();
    await expect(alertOf(page)).toHaveText("Summarization was cancelled.");
    await expect(library.getByRole("button", { name: /Write (an overview|it again)/ })).toBeVisible();
  });
});
