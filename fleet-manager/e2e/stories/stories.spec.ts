import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import {
  type APIRequestContext,
  type Page,
  expect,
  test,
} from "@playwright/test";
import { blockingViolations } from "../axe";
import type { Shot } from "./stories-index";

const OUT = path.resolve("ui-stories");
const WIDTHS = [1440, 390];

// The events of the Storybook channel that mean that a story failed: its
// render threw, its play function threw, it could not be shown, its stories
// file did not load (storyMissing), or the preview did not load.
const FAILURES = [
  "storyThrewException",
  "playFunctionThrewException",
  "storyErrored",
  "storyMissing",
  "configError",
];

// A story that does not load never sends storyFinished.
const DONE = ["storyFinished", "docsRendered", "storyMissing", "configError"];

type Entry = { id: string; title: string; name: string; type: string };
type StoryEvent = { name: string; message: string };

// recordStoryEvents makes each page that loads next keep the events of its
// Storybook channel, because the body classes do not show a failed play
// function. errorsOf then waits for a DONE event and returns one line for
// each failure. storyMissing carries no message, so its line holds the
// console errors of the page, where Storybook logs why the file failed.
async function recordStoryEvents(page: Page) {
  const consoleErrors: string[] = [];
  page.on("console", (m) => {
    if (m.type() === "error") consoleErrors.push(m.text());
  });
  await page.addInitScript((failures) => {
    type Channel = { emit: (name: string, ...args: unknown[]) => void };
    const events: { name: string; message: string }[] = [];
    let channel: Channel | undefined;
    Object.assign(globalThis, { __storyEvents: events });
    Object.defineProperty(globalThis, "__STORYBOOK_ADDONS_CHANNEL__", {
      configurable: true,
      get: () => channel,
      set: (value: Channel) => {
        channel = value;
        const emit = value.emit.bind(value);
        value.emit = (name, ...args) => {
          const detail = args[0] as
            { message?: string; description?: string } | undefined;
          events.push({
            name,
            message: failures.includes(name)
              ? String(detail?.message ?? detail?.description ?? "")
              : "",
          });
          emit(name, ...args);
        };
      },
    });
  }, FAILURES);
  return {
    async open(url: string) {
      consoleErrors.length = 0;
      await page.goto(url);
    },
    async errorsOf(): Promise<string[]> {
      await page.waitForFunction(
        (done) =>
          (
            globalThis as unknown as { __storyEvents: StoryEvent[] }
          ).__storyEvents.some((e) => done.includes(e.name)),
        DONE,
      );
      const events = await page.evaluate(
        () =>
          (globalThis as unknown as { __storyEvents: StoryEvent[] })
            .__storyEvents,
      );
      return events
        .filter((e) => FAILURES.includes(e.name))
        .map((e) => `${e.name}: ${e.message || consoleErrors.join("\n")}`);
    },
  };
}

async function storiesOf(request: APIRequestContext): Promise<Entry[]> {
  const index = (await (await request.get("index.json")).json()) as {
    entries: Record<string, Entry>;
  };
  return Object.values(index.entries).filter((e) => e.type === "story");
}

test("the Storybook opens a story under a sub-path", async ({
  page,
  request,
}) => {
  const [first] = await storiesOf(request);
  expect(first).toBeDefined();
  const failed: string[] = [];
  page.on("response", (response) => {
    if (response.status() >= 400)
      failed.push(`${response.status()} ${response.url()}`);
  });
  await page.goto(`./?path=/story/${first.id}`);
  // The preview frame sets sb-show-main on its body once the story shows.
  await expect(
    page.frameLocator("#storybook-preview-iframe").locator("body"),
  ).toHaveClass(/sb-show-main/);
  expect(failed).toEqual([]);
});

// The dark run keeps the file names that it had before the light theme.
const THEMES = [
  { theme: "dark", suffix: "", title: "" },
  { theme: "light", suffix: "-light", title: " in the light theme" },
] as const;

// A Docs page renders every story of its stories file, so the check opens
// one Docs page, not each of them.
for (const { theme, title } of THEMES)
  test(`the Docs page of a component${title}`, async ({ page }) => {
    const storybook = await recordStoryEvents(page);
    await storybook.open(
      `iframe.html?id=ui-button--docs&viewMode=docs&globals=theme:${theme}`,
    );
    expect.soft(await storybook.errorsOf(), "the Docs page throws").toEqual([]);
    await expect(
      page.getByRole("heading", { level: 1, name: "Button" }),
    ).toBeVisible();
    expect
      .soft(await page.evaluate(() => document.documentElement.className))
      .toBe(theme);
    expect(
      await blockingViolations(page),
      `axe violations in the Docs page${title}`,
    ).toEqual([]);
  });

for (const { theme, suffix, title } of THEMES)
  for (const width of WIDTHS) {
    test(`every story at ${width} px${title}`, async ({
      page,
      request,
      baseURL,
    }) => {
      // A request outside the Storybook folder goes to another host or to a
      // path of the app, such as /api/watch: the story reads live data.
      const outside: string[] = [];
      page.on("request", (r) => {
        const url = r.url();
        if (url.startsWith("http") && !url.startsWith(baseURL!))
          outside.push(url);
      });
      const stories = await storiesOf(request);
      expect(stories.length).toBeGreaterThan(0);
      // Each story costs a page load, an axe run, and a screenshot.
      test.setTimeout(Math.max(300_000, stories.length * 2 * 2_000));

      const storybook = await recordStoryEvents(page);
      await page.setViewportSize({ width, height: 900 });

      const shots: Shot[] = [];
      const manifest = path.join(OUT, `manifest-${width}${suffix}.json`);
      await mkdir(OUT, { recursive: true });
      for (const story of stories) {
        await test.step(story.id, async () => {
          await storybook.open(
            `iframe.html?id=${story.id}&viewMode=story&globals=theme:${theme}`,
          );
          expect
            .soft(await storybook.errorsOf(), `story ${story.id} throws`)
            .toEqual([]);

          expect
            .soft(
              await page.evaluate(() => document.documentElement.className),
              `story ${story.id} is not in the ${theme} theme`,
            )
            .toBe(theme);

          await page.evaluate(() => document.fonts.ready);
          const overflow = await page.evaluate(
            () =>
              document.documentElement.scrollWidth -
              document.documentElement.clientWidth,
          );
          expect
            .soft(overflow, `story ${story.id} scrolls sideways at ${width} px`)
            .toBeLessThanOrEqual(0);

          expect
            .soft(
              await blockingViolations(page),
              `axe violations in story ${story.id} at ${width} px`,
            )
            .toEqual([]);

          const file = `${story.id}-${width}${suffix}.png`;
          await page.screenshot({ path: path.join(OUT, file), fullPage: true });
          shots.push({
            id: story.id,
            component: story.title,
            name: story.name,
            width,
            theme,
            file,
          });
          // A later story can fail the test. The index teardown then still
          // lists every screenshot that was taken.
          await writeFile(manifest, JSON.stringify(shots, null, 2));
        });
      }

      expect(outside, "requests outside the Storybook folder").toEqual([]);
    });
  }
