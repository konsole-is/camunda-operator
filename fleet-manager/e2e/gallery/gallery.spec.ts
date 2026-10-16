import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import type { Shot } from "./gallery-index";

const OUT = path.resolve("ui-gallery");
const WIDTHS = [1440, 390];
const BLOCKING = new Set(["serious", "critical"]);

for (const width of WIDTHS) {
  test(`every story at ${width} px`, async ({ page, baseURL }) => {
    const appHost = new URL(baseURL!).host;
    const foreign: string[] = [];
    page.on("request", (request) => {
      const url = new URL(request.url());
      if (
        (url.protocol === "http:" || url.protocol === "https:") &&
        url.host !== appHost
      )
        foreign.push(request.url());
    });
    await page.setViewportSize({ width, height: 900 });

    await page.goto("/dev/gallery");
    const stories = await page
      .locator("a[data-story-id]")
      .evaluateAll((links) =>
        links.map((a) => ({
          id: a.getAttribute("data-story-id") ?? "",
          component: a.getAttribute("data-component") ?? "",
          name: a.getAttribute("data-story-name") ?? "",
        })),
      );
    expect(stories.length).toBeGreaterThan(0);
    // Each story costs a page load, an axe run, and a screenshot.
    test.setTimeout(Math.max(300_000, stories.length * 10_000));

    const shots: Shot[] = [];
    const manifest = path.join(OUT, `manifest-${width}.json`);
    await mkdir(OUT, { recursive: true });
    for (const story of stories) {
      await test.step(story.id, async () => {
        await page.goto(`/dev/gallery/${story.id}`);
        await page.evaluate(() => document.fonts.ready);

        const overflow = await page.evaluate(
          () =>
            document.documentElement.scrollWidth -
            document.documentElement.clientWidth,
        );
        expect
          .soft(overflow, `story ${story.id} scrolls sideways at ${width} px`)
          .toBeLessThanOrEqual(0);

        const { violations } = await new AxeBuilder({ page }).analyze();
        const blocking = violations
          .filter((v) => BLOCKING.has(v.impact ?? ""))
          .map(
            (v) =>
              `rule ${v.id} (${v.impact}) at ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`,
          );
        expect
          .soft(blocking, `axe violations in story ${story.id} at ${width} px`)
          .toEqual([]);

        const file = `${story.id}-${width}.png`;
        await page.screenshot({ path: path.join(OUT, file), fullPage: true });
        shots.push({ ...story, width, file });
        // A later story can throw. The index teardown then still lists every
        // screenshot that was taken.
        await writeFile(manifest, JSON.stringify(shots, null, 2));
      });
    }

    expect(foreign, "requests to a host other than the app").toEqual([]);
  });
}
