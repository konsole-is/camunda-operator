import path from "node:path";
import { type Locator, type Page, test } from "@playwright/test";
import { DOCS_IMAGES_DIR, DOCS_PROJECT } from "../docs-images";
import { expect } from "./fixture";
import { switchTheme } from "./themes";

/**
 * docShot saves the page, or options.target, in the light theme as
 * <name>.png for the guide, in the docs project only. In any other project
 * it returns at once, so the scenario checks stay as they are. It waits for
 * options.ready and the fonts first, and leaves the page dark. name must be
 * a string literal, because the check of e2e/docs-images.ts reads it from
 * the source.
 */
export async function docShot(
  page: Page,
  name: string,
  options: { ready: Locator; target?: Locator; mask?: Locator[] },
): Promise<void> {
  if (test.info().project.name !== DOCS_PROJECT) return;
  const url = new URL(page.url());
  await test.step(
    `docs image ${name} at ${url.pathname}${url.search}`,
    async () => {
      await expect(options.ready).toBeVisible();
      await page.evaluate(() => document.fonts.ready.then(() => undefined));
      await switchTheme(page, "light");
      try {
        await (options.target ?? page).screenshot({
          path: path.resolve(DOCS_IMAGES_DIR, `${name}.png`),
          animations: "disabled",
          caret: "hide",
          mask: options.mask,
          maskColor: options.mask ? await panelColor(page) : undefined,
        });
      } finally {
        await switchTheme(page, "dark");
      }
    },
    { timeout: 30_000 },
  );
}

// A mask in the color of a panel reads as an empty field, not as a broken
// image.
function panelColor(page: Page): Promise<string> {
  return page.evaluate(() => {
    const probe = document.createElement("span");
    probe.style.color = "var(--panel-2)";
    document.body.append(probe);
    const color = getComputedStyle(probe).color;
    probe.remove();
    return color;
  });
}
