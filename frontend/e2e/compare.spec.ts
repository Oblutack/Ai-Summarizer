import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Locator, type Page } from "@playwright/test";
import { newSignedInUser, summarizeText } from "./support/helpers";

// Comparing two saved documents: pick the other one, say which is newer, read what changed.
// (The AI service here is a stub that sets sentence number i of one text against sentence number i of the other.)

const ONE = "Agreement version one. Payment is due within 30 days. Delivery takes 10 days. Records are kept.";
const TWO = "Agreement version two. Payment is due within 60 days. Delivery takes 10 days. Records are kept. Audits happen yearly.";

function card(page: Page, title: string): Locator {
  return page.getByTestId("document-card").filter({ has: page.getByRole("heading", { name: title }) });
}

// Saves a document and waits until it is in the list (`copies`: how many documents with this title there should be then).
async function save(page: Page, text: string, title: string, copies = 1) {
  await page.goto("/dashboard");
  await summarizeText(page, text);
  await expect(card(page, title)).toHaveCount(copies);
}

async function openCompare(page: Page, title: string) {
  const saved = card(page, title).first();
  await saved.getByRole("button", { name: "More" }).click();
  await saved.getByRole("button", { name: "Compare with another document" }).click();
  return saved.getByTestId("compare");
}

async function pickOther(panel: Locator, search: string) {
  await panel.getByRole("searchbox", { name: "Find the other document" }).fill(search);
  const select = panel.getByRole("combobox", { name: "Other document", exact: true });
  await expect(select.locator("option").filter({ hasText: search })).not.toHaveCount(0);
  await select.selectOption({ index: 1 });
}

test.describe("comparing two documents", () => {
  test("two versions are compared and every change is shown with its exact text", async ({ page }) => {
    await newSignedInUser(page);
    await save(page, ONE, "Agreement version one");
    await save(page, TWO, "Agreement version two");

    const panel = await openCompare(page, "Agreement version two");
    await expect(panel.getByRole("button", { name: "Compare", exact: true })).toBeDisabled();
    await pickOther(panel, "version one");
    await panel.getByRole("button", { name: "Compare", exact: true }).click();

    const result = panel.getByTestId("comparison");
    await expect(result.getByRole("heading", { name: /^Agreement version one.* → Agreement version two/ })).toBeVisible();
    await expect(result).toContainText("2 changed, 1 added, 0 removed, 0 moved");
    await expect(panel.getByTestId("bottom-line")).toContainText("Stub bottom line (same language): 3 changes.");

    const changes = result.getByTestId("change");
    await expect(changes).toHaveCount(3);
    const payment = changes.filter({ hasText: "within 60 days" }).first();
    await expect(payment).toHaveAttribute("data-importance", "high");
    await expect(payment).toContainText("Changed");
    await expect(payment).toContainText("High importance");
    await expect(payment.getByTestId("change-numbers")).toContainText("Numbers removed: 30");
    await expect(payment.getByTestId("change-numbers")).toContainText("Numbers added: 60");
    // the words that went are struck through, the words that came are underlined, and both are said to a screen reader
    await expect(payment.locator("del")).toContainText("Payment is due within 30 days.");
    await expect(payment.locator("ins")).toContainText("Payment is due within 60 days.");
    await expect(payment.locator("del .sr-only")).toHaveText("removed: ");
    await expect(payment.locator("ins .sr-only")).toHaveText("added: ");

    const added = changes.filter({ hasText: "Audits happen yearly." });
    await expect(added).toHaveAttribute("data-kind", "added");
    await expect(added.getByText("Audits happen yearly.")).toBeVisible();
    await expect(added.getByText("Before", { exact: true })).toHaveCount(0);
  });

  test("the changes can be filtered by importance", async ({ page }) => {
    await newSignedInUser(page);
    await save(page, ONE, "Agreement version one");
    await save(page, TWO, "Agreement version two");
    const panel = await openCompare(page, "Agreement version two");
    await pickOther(panel, "version one");
    await panel.getByRole("button", { name: "Compare", exact: true }).click();
    const changes = panel.getByTestId("change");
    await expect(changes).toHaveCount(3);

    await panel.getByRole("button", { name: "High importance (1)" }).click();
    await expect(changes).toHaveCount(1);
    await expect(changes.first()).toContainText("within 60 days");
    await expect(panel.getByRole("button", { name: "High importance (1)" })).toHaveAttribute("aria-pressed", "true");

    await panel.getByRole("button", { name: "Low (1)" }).click();
    await expect(changes).toHaveCount(1);
    await expect(changes.first()).toContainText("Audits happen yearly.");

    await panel.getByRole("button", { name: "All (3)" }).click();
    await expect(changes).toHaveCount(3);
  });

  test("it matters which document is the newer one", async ({ page }) => {
    await newSignedInUser(page);
    await save(page, ONE, "Agreement version one");
    await save(page, TWO, "Agreement version two");
    const panel = await openCompare(page, "Agreement version two");
    await pickOther(panel, "version one");
    await panel.getByLabel("This document is the older one").check();
    await panel.getByRole("button", { name: "Compare", exact: true }).click();

    // read the other way round, what was added is now removed
    await expect(panel.getByRole("heading", { name: /^Agreement version two.* → Agreement version one/ })).toBeVisible();
    await expect(panel.getByTestId("comparison")).toContainText("2 changed, 0 added, 1 removed, 0 moved");
    await expect(panel.getByTestId("change").filter({ hasText: "Audits happen yearly." })).toHaveAttribute("data-kind", "removed");
  });

  test("the explanations can be asked for in another language", async ({ page }) => {
    await newSignedInUser(page);
    await save(page, ONE, "Agreement version one");
    await save(page, TWO, "Agreement version two");
    const panel = await openCompare(page, "Agreement version two");
    await pickOther(panel, "version one");
    await panel.getByRole("combobox", { name: "Write the explanations in" }).selectOption("German");
    await panel.getByRole("button", { name: "Compare", exact: true }).click();
    await expect(panel.getByTestId("bottom-line")).toContainText("(German)");
  });

  test("the comparison can be copied as a report", async ({ page, context }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
    await newSignedInUser(page);
    await save(page, ONE, "Agreement version one");
    await save(page, TWO, "Agreement version two");
    const panel = await openCompare(page, "Agreement version two");
    await pickOther(panel, "version one");
    await panel.getByRole("button", { name: "Compare", exact: true }).click();
    await expect(panel.getByTestId("comparison")).toBeVisible();
    await panel.getByRole("button", { name: "Copy" }).click();
    const copied = await page.evaluate(() => navigator.clipboard.readText());
    expect(copied).toMatch(/^# Comparison: Agreement version one.* → Agreement version two/);
    expect(copied).toContain("Numbers: removed 30; added 60");
    expect(copied).toContain("> Payment is due within 60 days.");
  });

  test("documents that say the same are reported as having no differences", async ({ page }) => {
    await newSignedInUser(page);
    await save(page, ONE, "Agreement version one");
    await save(page, ONE, "Agreement version one", 2);
    const panel = await openCompare(page, "Agreement version one");
    await panel.getByRole("searchbox", { name: "Find the other document" }).fill("version one");
    const select = panel.getByRole("combobox", { name: "Other document", exact: true });
    await expect(select.locator("option")).toHaveCount(2);
    await select.selectOption({ index: 1 });
    await panel.getByRole("button", { name: "Compare", exact: true }).click();
    await expect(panel.getByTestId("compare-identical")).toContainText("No differences found");
  });

  test("a document with nothing to compare it with says so", async ({ page }) => {
    await newSignedInUser(page);
    await save(page, ONE, "Agreement version one");
    const panel = await openCompare(page, "Agreement version one");
    await expect(panel.getByText("No other document with saved text matches.")).toBeVisible();
    await expect(panel.getByRole("button", { name: "Compare", exact: true })).toBeDisabled();
  });

  test("only my own documents can be picked", async ({ page, browser }) => {
    await newSignedInUser(page);
    await save(page, ONE, "Agreement version one");
    const other = await browser.newContext();
    const otherPage = await other.newPage();
    await newSignedInUser(otherPage);
    await save(otherPage, TWO, "Agreement version two");
    const panel = await openCompare(otherPage, "Agreement version two");
    await panel.getByRole("searchbox", { name: "Find the other document" }).fill("version one");
    await expect(panel.getByText("No other document with saved text matches.")).toBeVisible();
    await other.close();
  });

  test("a comparison that fails shows an error and can be tried again", async ({ page }) => {
    await newSignedInUser(page);
    await save(page, "Boom contract. Payment is due within 30 days.", "Boom contract");
    await save(page, "Fine contract. Payment is due within 60 days.", "Fine contract");
    const panel = await openCompare(page, "Fine contract");
    await pickOther(panel, "Boom");
    await panel.getByRole("button", { name: "Compare", exact: true }).click();
    await expect(panel.getByRole("alert")).toBeVisible();
    await expect(panel.getByTestId("comparison")).toHaveCount(0);
    await expect(panel.getByRole("button", { name: "Compare", exact: true })).toBeEnabled();
  });

  test("the comparison has no accessibility violations", async ({ page }) => {
    await newSignedInUser(page);
    await save(page, ONE, "Agreement version one");
    await save(page, TWO, "Agreement version two");
    const panel = await openCompare(page, "Agreement version two");
    await pickOther(panel, "version one");
    await panel.getByRole("button", { name: "Compare", exact: true }).click();
    await expect(panel.getByTestId("comparison")).toBeVisible();
    const results = await new AxeBuilder({ page }).include('[data-testid="compare"]').withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
    expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  });
});
