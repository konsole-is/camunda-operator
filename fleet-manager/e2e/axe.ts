import AxeBuilder from "@axe-core/playwright";
import type { Page } from "@playwright/test";

/** BLOCKING_IMPACTS are the axe impacts that fail a Playwright test. */
export const BLOCKING_IMPACTS: ReadonlySet<string> = new Set([
  "serious",
  "critical",
]);

/**
 * blockingViolations runs axe on the page as it is now and returns one line
 * for each violation of a blocking impact, "rule <id> (<impact>) at
 * <targets>". An empty list means that the page passes.
 */
export async function blockingViolations(page: Page): Promise<string[]> {
  const { violations } = await new AxeBuilder({ page }).analyze();
  return violations
    .filter((v) => BLOCKING_IMPACTS.has(v.impact ?? ""))
    .map(
      (v) =>
        `rule ${v.id} (${v.impact}) at ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`,
    );
}
