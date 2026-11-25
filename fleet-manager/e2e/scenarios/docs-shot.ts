import path from "node:path";
import { type Locator, type Page, test } from "@playwright/test";
import { DOCS_IMAGES_DIR, DOCS_PROJECT } from "../docs-images";
import { expect } from "./fixture";
import { switchTheme } from "./themes";

const HIDDEN = "data-docs-hidden";

/** DocShotOptions say what docShot waits for and what the image holds. */
export type DocShotOptions = {
  /** ready is visible when the page shows the state of the image. */
  ready: Locator;
  /** target is the element of the image. Without it, the image starts at the top of the page. */
  target?: Locator;
  /** until ends the image below this element, when the image is taller than it needs to be. The image must fit in the viewport. */
  until?: Locator;
  /** hide makes the text of each element transparent, so it reads as empty. */
  hide?: Locator[];
  /** prepare runs first, in the docs project only, so the scenario checks keep their steps. */
  prepare?: () => Promise<void>;
};

/**
 * docShot saves an image for the guide as <name>.png, in the light theme
 * and without the pointer or the focus on a control, in the docs project
 * only. In any other project it returns at once, so the scenario checks
 * stay as they are. It leaves the page dark. name must be a string
 * literal, because the check of e2e/docs-images.ts reads it from the
 * source.
 */
export async function docShot(
  page: Page,
  name: string,
  options: DocShotOptions,
): Promise<void> {
  if (test.info().project.name !== DOCS_PROJECT) return;
  const url = new URL(page.url());
  await test.step(
    `docs image ${name} at ${url.pathname}${url.search}`,
    async () => {
      await options.prepare?.();
      await page.mouse.move(0, 0);
      await page.evaluate(() =>
        (document.activeElement as HTMLElement)?.blur(),
      );
      if (options.target) {
        await options.target.evaluate((el) =>
          el.scrollIntoView({ block: "start" }),
        );
      } else {
        await page.evaluate(() => window.scrollTo(0, 0));
      }
      await expect(options.ready).toBeVisible();
      await page.evaluate(() => document.fonts.ready.then(() => undefined));
      for (const element of options.hide ?? []) {
        await element.evaluateAll(
          (els, attribute) =>
            els.forEach((el) => el.setAttribute(attribute, "")),
          HIDDEN,
        );
      }
      await switchTheme(page, "light");
      try {
        const shot = {
          path: path.resolve(DOCS_IMAGES_DIR, `${name}.png`),
          animations: "disabled" as const,
          caret: "hide" as const,
          style: `[${HIDDEN}] { color: transparent !important; }`,
        };
        if (options.until) {
          await page.screenshot({
            ...shot,
            clip: await clipUntil(page, options.until, options.target),
          });
        } else {
          await (options.target ?? page).screenshot(shot);
        }
      } finally {
        await switchTheme(page, "dark");
      }
    },
    { timeout: 30_000 },
  );
}

// clipUntil returns the part of the viewport from the top of target, or of
// the page, to 16 px below until.
async function clipUntil(page: Page, until: Locator, target?: Locator) {
  const viewport = page.viewportSize()!;
  const top = target ? await target.boundingBox() : undefined;
  const end = await until.boundingBox();
  if (!end || (target && !top)) throw new Error("The image has no box");
  const x = top?.x ?? 0;
  const y = top?.y ?? 0;
  const height = Math.ceil(end.y + end.height + 16 - y);
  if (y + height > viewport.height) {
    throw new Error(`The image needs ${height} px, more than the viewport`);
  }
  return { x, y, width: top?.width ?? viewport.width, height };
}
