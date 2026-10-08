import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { newSignedInUser, summarizeText } from "./support/helpers";

// Automated checks against the WCAG 2 A and AA rules (contrast, names for buttons and fields, headings,
// landmarks, and so on). They cannot replace testing with a keyboard and a screen reader, but they catch
// the mistakes that are easy to make and easy to miss.

async function violations(page: Page) {
  const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
  return results.violations.map((v) => `${v.id} (${v.impact}): ${v.help} -> ${v.nodes.slice(0, 3).map((n) => n.target.join(" ")).join(" | ")}`);
}

for (const scheme of ["light", "dark"] as const) {
  test.describe(`${scheme} theme`, () => {
    test.use({ colorScheme: scheme });

    for (const path of ["/", "/login", "/signup", "/forgot-password", "/reset-password", "/s/" + "a".repeat(43)]) {
      test(`${path} has no accessibility violations`, async ({ page }) => {
        await page.goto(path);
        await page.waitForLoadState("networkidle");
        await expect(page.locator("h1").first()).toBeVisible();
        expect(await violations(page)).toEqual([]);
      });
    }

    test("the dashboard, with a saved summary and its panels open, has no accessibility violations", async ({ page }) => {
      await newSignedInUser(page);
      await summarizeText(page, "Alpha notes\nSome words about alpha.");
      const card = page.getByTestId("document-card");
      await expect(card).toHaveCount(1);
      await card.getByRole("button", { name: "More" }).click();
      await card.getByRole("button", { name: "Chat With Document" }).click();
      await expect(page.getByTestId("suggestions")).toBeVisible();
      await card.getByRole("button", { name: "Study" }).click();
      await card.getByRole("button", { name: "Flashcards" }).click();
      await expect(page.getByTestId("flashcard")).toBeVisible();
      await card.getByRole("button", { name: "Check against the original" }).click();
      await expect(page.getByTestId("proof")).toBeVisible();
      expect(await violations(page)).toEqual([]);
    });

    test("the account page has no accessibility violations", async ({ page }) => {
      await newSignedInUser(page);
      await page.getByRole("link", { name: "Account" }).click();
      await expect(page.getByRole("heading", { name: "Your Account" })).toBeVisible();
      await page.waitForLoadState("networkidle");
      expect(await violations(page)).toEqual([]);
    });
  });
}

test.describe("on a phone", () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test("the menu opens, lists the links and has no accessibility violations", async ({ page }) => {
    await page.goto("/");
    const menu = page.getByRole("button", { name: "Menu" });
    await expect(menu).toHaveAttribute("aria-expanded", "false");
    await menu.click();
    await expect(page.getByRole("button", { name: "Close the menu" })).toHaveAttribute("aria-expanded", "true");
    await expect(page.locator("#mobile-menu").getByRole("link", { name: "Login" })).toBeVisible();
    expect(await violations(page)).toEqual([]);

    await page.keyboard.press("Escape");
    await expect(page.locator("#mobile-menu")).toHaveCount(0);
  });

  test("nothing scrolls sideways", async ({ page }) => {
    for (const path of ["/", "/login"]) {
      await page.goto(path);
      await page.waitForLoadState("networkidle");
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
      expect(overflow, path).toBeLessThanOrEqual(0);
    }
  });
});

test("the skip link is the first thing a keyboard reaches and it jumps to the content", async ({ page }) => {
  await page.goto("/login");
  await page.keyboard.press("Tab");
  const skip = page.getByRole("link", { name: "Skip to the content" });
  await expect(skip).toBeFocused();
  await expect(skip).toBeVisible();
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/#main$/);
});
