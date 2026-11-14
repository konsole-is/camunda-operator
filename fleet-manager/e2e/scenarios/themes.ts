import type { Page } from "@playwright/test";
import { blockingViolations } from "../axe";
import { PAGES_DIR, shot } from "../shot";
import { expect } from "./fixture";

/** Theme is a theme that a page scenario checks. */
export type Theme = "dark" | "light";

/**
 * inBothThemes runs check on the page in the dark theme and then in the light
 * theme, and leaves the page dark. The page must follow the theme of the
 * system, which it does without a theme cookie, and the context must start
 * dark, as playwright.scenarios.config.ts sets.
 */
export async function inBothThemes(
  page: Page,
  check: (theme: Theme) => Promise<void>,
): Promise<void> {
  await check("dark");
  await switchTheme(page, "light");
  try {
    await check("light");
  } finally {
    await switchTheme(page, "dark");
  }
}

/**
 * checkPage fails the test on a blocking axe violation in either theme, and
 * saves a screenshot of the page in each theme into PAGES_DIR.
 */
export async function checkPage(page: Page, name: string): Promise<void> {
  await inBothThemes(page, async (theme) => {
    expect(
      await blockingViolations(page),
      `axe violations in the ${theme} theme`,
    ).toEqual([]);
    await shot(page, name, { dir: PAGES_DIR, theme });
  });
}

async function switchTheme(page: Page, theme: Theme): Promise<void> {
  await page.emulateMedia({ colorScheme: theme });
  // The controls fade to their new colors. axe and a screenshot during the
  // fade read colors of neither theme.
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a instanceof CSSTransition)
        .map((a) => a.finished.catch(() => undefined)),
    ),
  );
}
