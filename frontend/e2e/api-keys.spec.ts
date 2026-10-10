import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { API, newSignedInUser } from "./support/helpers";

// API keys: made on the account page, shown once, used by a program with "Authorization: Bearer ink_...", ended there.

async function openKeys(page: Page) {
  await page.getByRole("link", { name: "Account" }).click();
  const section = page.getByTestId("api-keys");
  await expect(section).toBeVisible();
  return section;
}

async function makeKey(page: Page, name: string): Promise<string> {
  const section = page.getByTestId("api-keys");
  await section.getByLabel("Key name (optional)").fill(name);
  await section.getByRole("button", { name: "Create key" }).click();
  const shown = page.getByTestId("new-api-key-value");
  await expect(shown).toHaveText(/^ink_/);
  return (await shown.innerText()).trim();
}

const summarizeWith = (page: Page, key: string) =>
  page.request.post(`${API}/v1/summarize-text`, {
    headers: { Authorization: `Bearer ${key}`, "Content-Type": "application/json" },
    data: { text: "Heat pumps are efficient. Sales rose in 2024." },
  });

test.describe("API keys", () => {
  test("a key is made, shown once, and works from a program", async ({ page }) => {
    await newSignedInUser(page);
    const section = await openKeys(page);
    await expect(section.getByText("You have no API keys yet.")).toBeVisible();
    await expect(section.getByTestId("api-example")).toContainText("/v1/summarize-text");

    const key = await makeKey(page, "nightly report");
    expect(key).toMatch(/^ink_[A-Za-z0-9_-]{43}$/);
    await expect(section.getByText("it is shown only once")).toBeVisible();
    const row = section.getByTestId("api-key-row");
    await expect(row).toHaveCount(1);
    await expect(row).toContainText("nightly report");
    await expect(row).toContainText(`${key.slice(0, 8)}…`);
    await expect(row).toContainText("Never used");
    await expect(row).not.toContainText(key);

    // a program (no browser session) summarizes with it
    const call = await page.context().request.post(`${API}/v1/summarize-text`, {
      headers: { Authorization: `Bearer ${key}`, "Content-Type": "application/json" },
      data: { text: "Heat pumps are efficient. Sales rose in 2024." },
    });
    expect(call.status()).toBe(200);
    expect((await call.json()).summary).toContain("Stub summary");

    // once it is dismissed or the page is reloaded the key is gone for good, and the list says it was used
    await section.getByRole("button", { name: "I have copied it" }).click();
    await expect(page.getByTestId("new-api-key")).toHaveCount(0);
    await page.reload();
    await expect(page.getByTestId("api-key-row")).toContainText(/Last used/);
    await expect(page.getByTestId("api-keys")).not.toContainText(key);
    await expect(page.getByTestId("new-api-key")).toHaveCount(0);
  });

  test("the new key can be copied", async ({ page, context }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
    await newSignedInUser(page);
    await openKeys(page);
    const key = await makeKey(page, "");
    await page.getByTestId("new-api-key").getByRole("button", { name: "Copy" }).click();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(key);
    await expect(page.getByTestId("api-key-row")).toContainText("API key"); // the default name
  });

  test("revoking asks first, and a revoked key stops working at once", async ({ page }) => {
    await newSignedInUser(page);
    const section = await openKeys(page);
    const key = await makeKey(page, "short lived");
    await expect.poll(async () => (await summarizeWith(page, key)).status()).toBe(200);

    await section.getByRole("button", { name: "Revoke short lived" }).click();
    await expect(section.getByText("Programs using it stop working at once.")).toBeVisible();
    await section.getByRole("button", { name: "Cancel" }).click();
    expect((await summarizeWith(page, key)).status()).toBe(200); // nothing happened

    await section.getByRole("button", { name: "Revoke short lived" }).click();
    await section.getByRole("button", { name: "Yes, revoke" }).click();
    const row = section.getByTestId("api-key-row");
    await expect(row).toContainText("Revoked");
    await expect(row.getByRole("button", { name: /Revoke/ })).toHaveCount(0);
    const refused = await summarizeWith(page, key);
    expect(refused.status()).toBe(401);
    expect((await refused.json()).code).toBe("invalid_api_key");
  });

  test("at most five live keys, and revoking one makes room", async ({ page }) => {
    await newSignedInUser(page);
    const section = await openKeys(page);
    for (let i = 1; i <= 5; i++) {
      await makeKey(page, `key ${i}`);
      await section.getByRole("button", { name: "I have copied it" }).click();
    }
    await expect(section.getByText("You have 5 keys, the most allowed.")).toBeVisible();
    await expect(section.getByLabel("Key name (optional)")).toHaveCount(0);

    await section.getByRole("button", { name: "Revoke key 1" }).click();
    await section.getByRole("button", { name: "Yes, revoke" }).click();
    await expect(section.getByLabel("Key name (optional)")).toBeVisible();
  });

  test("the keys section has no accessibility violations, with a new key showing", async ({ page }) => {
    await newSignedInUser(page);
    await openKeys(page);
    await makeKey(page, "checked");
    await expect(page.getByTestId("new-api-key")).toBeVisible();
    const results = await new AxeBuilder({ page }).include('[data-testid="api-keys"]').withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
    expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  });

  test("a key cannot do what only a signed-in person can", async ({ page, playwright }) => {
    await newSignedInUser(page);
    await openKeys(page);
    const key = await makeKey(page, "narrow");
    // a program: no cookies, only the key
    const program = await playwright.request.newContext({ extraHTTPHeaders: { Authorization: `Bearer ${key}` } });
    for (const path of ["/documents", "/auth/me", "/account/api-keys", "/account/export"]) {
      expect((await program.get(`${API}${path}`)).status(), path).toBe(401);
    }
    expect((await program.get(`${API}/v1/usage`)).status()).toBe(200);
    await program.dispose();
  });
});
