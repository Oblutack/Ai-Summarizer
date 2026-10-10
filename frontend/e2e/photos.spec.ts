import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { alertOf, newSignedInUser, pdf } from "./support/helpers";
import { jpegFile, pngFile } from "./support/photos";

// Photos of pages: the text in them is read (OCR), then they are summarized like any document. Several photos are the
// pages of one document. They are for signed-in people.

const summarize = (page: Page) => page.getByRole("button", { name: "Summarize", exact: true }).click();

function card(page: Page, title: string) {
  return page.getByTestId("document-card").filter({ has: page.getByRole("heading", { name: title }) });
}

async function showOriginalText(saved: ReturnType<typeof card>) {
  // exact: other buttons of a card with several photos say "(+2 more photos)"
  await saved.getByRole("button", { name: "More", exact: true }).click();
  await saved.getByRole("button", { name: "Show the original text" }).click();
  return saved.getByTestId("original-text-body");
}

test.describe("photos of pages", () => {
  test("a signed-in person attaches a photo, sees it, and gets a summary of the text in it", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(jpegFile("lease page.jpg"));
    await expect(page.getByText("lease page.jpg")).toBeVisible();
    await expect(page.getByTestId("photo-thumbnail")).toHaveCount(1);
    await expect(page.getByTestId("photo-hint")).toHaveText(/text in a photo is read first/);

    await summarize(page);
    await expect(page.getByText("Subject: lease page.jpg")).toBeVisible();
    await expect(card(page, "lease page.jpg")).toBeVisible();
  });

  test("the text that was read is kept, to check against the photo", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(jpegFile("lease page.jpg"));
    await summarize(page);
    const saved = card(page, "lease page.jpg");
    await expect(saved).toBeVisible();

    const text = await showOriginalText(saved);
    await expect(text).toContainText("Text read from the photo lease page.jpg");
    await expect(saved.getByRole("button", { name: "Chat With Document" })).toBeVisible();
  });

  test("several photos are the pages of one document, in the order they were picked", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles([jpegFile("page 1.jpg"), jpegFile("page 2.jpg"), jpegFile("page 3.jpg")]);
    await expect(page.getByTestId("photo-thumbnail")).toHaveCount(3);
    await summarize(page);
    await expect(page.getByText("Subject: page 1.jpg (+2 more photos)")).toBeVisible();

    const text = await showOriginalText(card(page, "page 1.jpg (+2 more photos)"));
    await expect(text).toContainText(/Text read from the photo page 1\.jpg[\s\S]*page 2\.jpg[\s\S]*page 3\.jpg/);
  });

  test("a photo can be combined with a PDF", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles([pdf("agenda.pdf"), jpegFile("notes.png")]);
    await summarize(page);
    await expect(page.getByText("Subject: agenda.pdf, notes.png")).toBeVisible();
    await expect(card(page, "agenda.pdf, notes.png")).toBeVisible();
  });

  test("a big photo is made smaller before it is sent, and a small one is sent as it is", async ({ page }) => {
    test.setTimeout(90_000);
    await newSignedInUser(page);
    // wider than the 3000 pixels a photo is kept to, with enough noise that it does not squeeze down to nothing
    const big = pngFile("IMG_0042.png", 3600, 1800, 40);
    await page.locator("#pdf-upload").setInputFiles(big);
    await expect(page.getByText("IMG_0042.png")).toBeVisible(); // the list still shows the picked file
    await summarize(page);

    // what reached the service: a JPEG (it is named so) that is far smaller than the original
    await expect(page.getByText("Subject: IMG_0042.jpg")).toBeVisible();
    const reported = await (await showOriginalText(card(page, "IMG_0042.jpg"))).innerText();
    const sent = Number(/\((\d+) bytes\)/.exec(reported)?.[1]);
    expect(sent).toBeGreaterThan(0);
    expect(sent).toBeLessThan(big.buffer.length / 2);

    await page.goto("/dashboard"); // a fresh form
    const small = pngFile("small page.png", 800, 600);
    await page.locator("#pdf-upload").setInputFiles(small);
    await summarize(page);
    const text = await showOriginalText(card(page, "small page.png"));
    await expect(text).toContainText(`(${small.buffer.length} bytes)`);
  });

  test("a picture the browser cannot open is sent as it is", async ({ page }) => {
    await newSignedInUser(page);
    const broken = jpegFile("odd.jpg", 4 * 1024 * 1024); // big in bytes, but not a real picture
    await page.locator("#pdf-upload").setInputFiles(broken);
    await summarize(page);
    const text = await showOriginalText(card(page, "odd.jpg"));
    await expect(text).toContainText(`(${broken.buffer.length} bytes)`);
  });

  test("someone who is not signed in is told to sign in, and nothing is attached", async ({ page }) => {
    await page.goto("/");
    await page.locator("#pdf-upload").setInputFiles(jpegFile("page.jpg"));
    await expect(alertOf(page)).toHaveText("Sign in to summarize a photo.");
    await expect(page.getByText("page.jpg")).toHaveCount(0);

    // documents still work without an account, and a mixed pick keeps them
    await page.locator("#pdf-upload").setInputFiles([jpegFile("page.jpg"), pdf("agenda.pdf")]);
    await expect(alertOf(page)).toHaveText("Sign in to summarize a photo.");
    await expect(page.getByText("agenda.pdf")).toBeVisible();
    await expect(page.getByTestId("photo-thumbnail")).toHaveCount(0);
  });

  test("the file picker offers photos", async ({ page }) => {
    await newSignedInUser(page);
    const accept = await page.locator("#pdf-upload").getAttribute("accept");
    for (const extension of [".pdf", ".jpg", ".jpeg", ".png", ".webp", ".heic", ".heif"]) expect(accept).toContain(extension);
  });

  test("a picture format that cannot be read is not accepted", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles({ name: "photo.gif", mimeType: "image/gif", buffer: Buffer.alloc(100, 1) });
    await expect(alertOf(page)).toHaveText(/photos \(JPG, PNG, WebP, HEIC\) are supported/);
  });

  test("an iPhone picture (HEIC) is accepted and sent as it is, even though this browser cannot draw it", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles({ name: "IMG_0042.HEIC", mimeType: "image/heic", buffer: Buffer.alloc(100, 1) });
    await expect(page.getByText("IMG_0042.HEIC")).toBeVisible();
    await expect(alertOf(page)).toHaveCount(0);
    await summarize(page);
    await expect(page.getByText("Subject: IMG_0042.HEIC")).toBeVisible();
  });

  test("photos taken out of turn can be put in order before they are sent", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles([jpegFile("page 2.jpg"), jpegFile("page 1.jpg"), jpegFile("page 3.jpg")]);
    const names = () => page.locator("li span[title]").allInnerTexts();
    expect(await names()).toEqual(["page 2.jpg", "page 1.jpg", "page 3.jpg"]);
    await expect(page.getByRole("button", { name: "Move page 2.jpg up" })).toBeDisabled();
    await expect(page.getByRole("button", { name: "Move page 3.jpg down" })).toBeDisabled();
    await page.getByRole("button", { name: "Move page 1.jpg up" }).click();
    expect(await names()).toEqual(["page 1.jpg", "page 2.jpg", "page 3.jpg"]);
    await summarize(page);
    const text = await showOriginalText(card(page, "page 1.jpg (+2 more photos)"));
    await expect(text).toContainText(/photo page 1\.jpg[\s\S]*page 2\.jpg[\s\S]*page 3\.jpg/);
  });

  test("a single file has no move buttons", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(jpegFile("only.jpg"));
    await expect(page.getByRole("button", { name: /Move only\.jpg/ })).toHaveCount(0);
  });

  test("the form with a photo attached has no accessibility violations", async ({ page }) => {
    await newSignedInUser(page);
    await page.locator("#pdf-upload").setInputFiles(jpegFile("lease page.jpg"));
    await expect(page.getByTestId("photo-thumbnail")).toBeVisible();
    const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
    expect(results.violations).toEqual([]);
  });
});

test.describe("on a phone", () => {
  test.use({ hasTouch: true, isMobile: true, viewport: { width: 390, height: 844 } });

  test("a signed-in person can take a photo, which joins the list", async ({ page }) => {
    await newSignedInUser(page);
    await expect(page.getByText("Take a photo")).toBeVisible();
    // it asks for a JPEG, PNG or WebP (not image/*), so an iPhone hands over a JPEG and not a HEIC picture
    const camera = page.locator("#photo-capture");
    await expect(camera).toHaveAttribute("capture", "environment");
    await expect(camera).toHaveAttribute("accept", "image/jpeg,image/png,image/webp");
    await page.locator("#photo-capture").setInputFiles(jpegFile("image.jpg"));
    await expect(page.getByText("image.jpg")).toBeVisible();
    await expect(page.getByTestId("photo-thumbnail")).toHaveCount(1);
    // pages can be added one after the other
    await page.locator("#photo-capture").setInputFiles(jpegFile("image.jpg", 4096));
    await expect(page.getByTestId("photo-thumbnail")).toHaveCount(2);
  });

  test("someone who is not signed in does not get the camera button", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByText("Take a photo")).toHaveCount(0);
  });
});

test.describe("on a computer", () => {
  test("there is no camera button, since the Attach button already offers the picture files", async ({ page }) => {
    await newSignedInUser(page);
    await expect(page.getByText("Take a photo")).toHaveCount(0);
    await expect(page.locator("#photo-capture")).toHaveCount(0);
  });
});
