import { expect, test } from "@playwright/test";
import { alertOf, logIn, newSignedInUser, PASSWORD, summarizeText, uniqueEmail } from "./support/helpers";

test.describe("account page", () => {
  test("shows the profile and today's usage, which counts summaries", async ({ page }) => {
    const email = await newSignedInUser(page);
    await page.getByRole("link", { name: "Account" }).click();
    await expect(page.locator("p", { hasText: `Email: ${email}` })).toBeVisible();
    await expect(page.getByText("0 / 50")).toBeVisible();

    await page.getByRole("link", { name: "Dashboard" }).click();
    await summarizeText(page, "One short paragraph to summarize for the usage counter.");
    await expect(page.getByText("Stub summary").first()).toBeVisible();

    await page.getByRole("link", { name: "Account" }).click();
    await expect(page.getByText("1 / 50")).toBeVisible();
  });

  test("changing the password keeps this device and signs out the others", async ({ page, browser }) => {
    const email = await newSignedInUser(page);

    // A second device signs in with the same account.
    const other = await browser.newContext();
    const otherPage = await other.newPage();
    await logIn(otherPage, email);
    await expect(otherPage).toHaveURL(/\/dashboard/);

    await page.getByRole("link", { name: "Account" }).click();
    await page.locator("#current-password").fill(PASSWORD);
    await page.locator("#new-password").fill("Changed-Pass-42");
    await page.getByRole("button", { name: "Change password" }).click();
    await expect(page.getByText(/password was changed|password changed/i)).toBeVisible();

    // This device is still signed in; the other one was signed out.
    await page.reload();
    await expect(page.getByRole("heading", { name: "Your Account" })).toBeVisible();
    await otherPage.reload();
    await expect(otherPage).toHaveURL(/\/login/);
    await other.close();

    // And the new password is the one that works.
    await page.getByRole("button", { name: "Logout" }).click();
    await logIn(page, email, PASSWORD);
    await expect(alertOf(page)).toBeVisible();
    await logIn(page, email, "Changed-Pass-42");
    await expect(page).toHaveURL(/\/dashboard/);
  });

  test("a wrong current password is refused without signing the user out", async ({ page }) => {
    await newSignedInUser(page);
    await page.getByRole("link", { name: "Account" }).click();
    await page.locator("#current-password").fill("Not-My-Password-1");
    await page.locator("#new-password").fill("Changed-Pass-42");
    await page.getByRole("button", { name: "Change password" }).click();
    await expect(alertOf(page)).toContainText(/incorrect|wrong|current/i);
    await expect(page.getByRole("heading", { name: "Your Account" })).toBeVisible();
  });

  test("another device can be signed out remotely", async ({ page, browser }) => {
    const email = await newSignedInUser(page);
    const other = await browser.newContext();
    const otherPage = await other.newPage();
    await logIn(otherPage, email);
    await expect(otherPage).toHaveURL(/\/dashboard/);

    await page.getByRole("link", { name: "Account" }).click();
    await expect(page.getByText("This device")).toBeVisible();
    await page.getByRole("button", { name: "Sign out", exact: true }).click();
    // Reload the other device only once the sign-out has gone through (its row leaves the list).
    await expect(page.getByRole("button", { name: "Sign out", exact: true })).toHaveCount(0);

    await otherPage.reload();
    await expect(otherPage).toHaveURL(/\/login/);
    await other.close();
  });

  test("sign out everywhere ends this session too", async ({ page }) => {
    await newSignedInUser(page);
    await page.getByRole("link", { name: "Account" }).click();
    await page.getByRole("button", { name: "Sign out everywhere" }).click();
    await expect(page).toHaveURL(/\/login/);
    await page.goto("/dashboard");
    await expect(page).toHaveURL(/\/login/);
  });

  test("the data export contains the account and its documents", async ({ page }) => {
    const email = await newSignedInUser(page);
    await summarizeText(page, "Text that will appear in the export of this account.");
    await expect(page.locator("h3").first()).toBeVisible();

    await page.getByRole("link", { name: "Account" }).click();
    const download = page.waitForEvent("download");
    await page.getByRole("button", { name: "Download my data" }).click();
    const file = await download;
    expect(file.suggestedFilename()).toBe("inkling-export.json");

    const { readFile } = await import("node:fs/promises");
    const data = JSON.parse(await readFile((await file.path())!, "utf8"));
    expect(data.account.email).toBe(email);
    expect(data.documents).toHaveLength(1);
    expect(data.documents[0].sourceText).toContain("appear in the export");
  });

  test("deleting the account needs the email and password, then removes everything", async ({ page }) => {
    const email = await newSignedInUser(page);
    await page.getByRole("link", { name: "Account" }).click();

    // Wrong confirmation: refused, account intact.
    await page.locator("#confirm-email").fill("someone-else@e2e.example");
    await page.locator("#delete-password").fill(PASSWORD);
    await page.getByRole("button", { name: "Delete my account" }).click();
    await expect(alertOf(page)).toBeVisible();
    await expect(page.getByRole("heading", { name: "Your Account" })).toBeVisible();

    // Wrong password: refused as well.
    await page.locator("#confirm-email").fill(email);
    await page.locator("#delete-password").fill("Wrong-Password-1");
    await page.getByRole("button", { name: "Delete my account" }).click();
    await expect(alertOf(page)).toContainText(/incorrect/i);
    await expect(page.getByRole("heading", { name: "Your Account" })).toBeVisible();

    // Correct details: gone.
    await page.locator("#delete-password").fill(PASSWORD);
    await page.getByRole("button", { name: "Delete my account" }).click();
    await expect(page).toHaveURL(/localhost:13000\/?$/);

    await logIn(page, email);
    await expect(alertOf(page)).toBeVisible();
    await expect(page).toHaveURL(/\/login/);
  });

  test("two accounts never see each other's documents", async ({ page, browser }) => {
    await newSignedInUser(page, "alice");
    await summarizeText(page, "Alice private notes about the merger.");
    await expect(page.locator("h3").first()).toBeVisible();

    const other = await browser.newContext();
    const bobPage = await other.newPage();
    const bob = uniqueEmail("bob");
    const { signUp } = await import("./support/helpers");
    await signUp(bobPage, bob);
    await logIn(bobPage, bob);
    await expect(bobPage).toHaveURL(/\/dashboard/);
    await expect(bobPage.getByText("You have no saved documents yet.")).toBeVisible();
    await other.close();
  });
});
