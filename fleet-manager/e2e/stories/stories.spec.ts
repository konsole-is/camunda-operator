import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import {
  type APIRequestContext,
  type Page,
  type Request,
  errors,
  expect,
  test,
} from "@playwright/test";
import { blockingViolations } from "../axe";
import type { Shot } from "./stories-index";
import { storySnapshot } from "./story-snapshot";
import { MIN_TARGET, smallTargets } from "./targets";

const OUT = path.resolve("ui-stories");
const SNAPSHOT = path.resolve("story-snapshot");
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

// LIMIT bounds each wait for a page. A healthy story sends its DONE event
// in seconds, also under heavy load.
const LIMIT = 60_000;

// ANSWER_LIMIT bounds the question to a stalled page, so that the check
// still fails soon after LIMIT when the page answers nothing.
const ANSWER_LIMIT = 5_000;

// within rejects with the message when promise takes longer than limit, for
// a call such as page.evaluate that takes no timeout.
async function within<T>(
  promise: Promise<T>,
  message: string,
  limit = LIMIT,
): Promise<T> {
  let timer: NodeJS.Timeout | undefined;
  const late = new Promise<never>((_, reject) => {
    timer = setTimeout(() => reject(new Error(message)), limit);
  });
  try {
    return await Promise.race([promise, late]);
  } finally {
    clearTimeout(timer);
  }
}

type Entry = { id: string; title: string; name: string; type: string };
type StoryEvent = { name: string; message: string };

// recordStoryEvents makes each page that loads next keep the events of its
// Storybook channel, because the body classes do not show a failed play
// function. errorsOf then waits for a DONE event and returns one line for
// each failure. storyMissing carries no message, so its line holds the
// console errors of the page, where Storybook logs why the file failed.
// errorsOf throws when no DONE event comes within LIMIT. label names the
// story in that message.
async function recordStoryEvents(page: Page) {
  const consoleErrors: string[] = [];
  const failedRequests: string[] = [];
  const unfinished = new Map<Request, number>();
  page.on("console", (m) => {
    if (m.type() === "error") consoleErrors.push(m.text());
  });
  page.on("request", (r) => unfinished.set(r, Date.now()));
  page.on("requestfinished", (r) => unfinished.delete(r));
  page.on("requestfailed", (r) => {
    unfinished.delete(r);
    failedRequests.push(`${r.url()}: ${r.failure()?.errorText}`);
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
  // stall describes a page that sent no DONE event. A network change on
  // the host, such as a new Docker container, makes Chromium fail the
  // module requests of the page with ERR_NETWORK_CHANGED, and the preview
  // of Storybook then never starts.
  const stall = async (label: string) => {
    const now = Date.now();
    const list = (lines: string[]) =>
      lines.length ? lines.map((l) => `\n  ${l}`).join("") : " none";
    let shown: string;
    try {
      const state = await within(
        page.evaluate(() => ({
          preview: "__STORYBOOK_PREVIEW__" in globalThis,
          resources: performance.getEntriesByType("resource").length,
        })),
        "the page did not answer",
        ANSWER_LIMIT,
      );
      shown = `The Storybook preview started: ${state.preview ? "yes" : "no"}. Resources loaded: ${state.resources}.`;
    } catch {
      shown = `The page did not answer within ${ANSWER_LIMIT / 1000} s.`;
    }
    const open = [...unfinished].map(
      ([r, at]) => `${r.url()} (for ${Math.round((now - at) / 1000)} s)`,
    );
    return [
      `${label} sent no DONE event within ${LIMIT / 1000} s.`,
      shown,
      `Requests that never finished:${list(open)}`,
      `Failed requests:${list(failedRequests)}`,
      `Console errors:${list(consoleErrors)}`,
    ].join("\n");
  };
  return {
    async open(url: string) {
      consoleErrors.length = 0;
      failedRequests.length = 0;
      unfinished.clear();
      await page.goto(url, { timeout: LIMIT });
    },
    async errorsOf(label: string): Promise<string[]> {
      try {
        await page.waitForFunction(
          (done) =>
            (
              globalThis as unknown as { __storyEvents: StoryEvent[] }
            ).__storyEvents.some((e) => done.includes(e.name)),
          DONE,
          { timeout: LIMIT },
        );
      } catch (error) {
        if (!(error instanceof errors.TimeoutError)) throw error;
        throw new Error(await stall(label));
      }
      const events = await within(
        page.evaluate(
          () =>
            (globalThis as unknown as { __storyEvents: StoryEvent[] })
              .__storyEvents,
        ),
        `${label} did not answer within ${LIMIT / 1000} s`,
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
  await page.goto(`./?path=/story/${first.id}`, { timeout: LIMIT });
  // The preview frame sets sb-show-main on its body once the story shows.
  await expect(
    page.frameLocator("#storybook-preview-iframe").locator("body"),
  ).toHaveClass(/sb-show-main/);
  expect(failed).toEqual([]);
});

// Playwright cannot use the find function of the browser. A text fragment
// reveals the text of a closed DisclosureGroup the same way. Chromium looks
// for the text only for a short time after a navigation, and the story can
// render after that time. So the fragment comes in a second navigation,
// after the story shows the text.
test("a text fragment opens the closed group that holds the text", async ({
  page,
}) => {
  const story =
    "iframe.html?viewMode=story&id=setup-releases-envscopes--no-entries";
  await page.goto(story, { timeout: LIMIT });
  await expect(page.getByText("Variables of Gateway")).toBeAttached({
    timeout: 30_000,
  });
  await page.goto(`${story}#:~:text=Variables%20of%20Gateway`, {
    timeout: LIMIT,
  });
  // The row of each env scope reads its name and its count of entries.
  const toggle = (name: string) =>
    page.getByRole("button", { name: `${name} 0`, exact: true });
  await expect(toggle("Gateway")).toHaveAttribute("aria-expanded", "true");
  await expect(toggle("Operate")).toHaveAttribute("aria-expanded", "false");
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
    const label = `The Docs page${title}`;
    const late = `${label} did not finish its checks within ${LIMIT / 1000} s`;
    expect
      .soft(await storybook.errorsOf(label), "the Docs page throws")
      .toEqual([]);
    await expect(
      page.getByRole("heading", { level: 1, name: "Button" }),
    ).toBeVisible();
    expect
      .soft(
        await within(
          page.evaluate(() => document.documentElement.className),
          late,
        ),
      )
      .toBe(theme);
    expect(
      await within(blockingViolations(page), late),
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
      // The markup of a story at 390 px or in the light theme differs only
      // in measured sizes and in the pressed button of the theme switch.
      const snapshot =
        width === WIDTHS[0] && theme === "dark"
          ? storySnapshot(SNAPSHOT)
          : undefined;
      const manifest = path.join(OUT, `manifest-${width}${suffix}.json`);
      await mkdir(OUT, { recursive: true });
      for (const story of stories) {
        await test.step(story.id, async () => {
          await storybook.open(
            `iframe.html?id=${story.id}&viewMode=story&globals=theme:${theme}`,
          );
          const label = `Story ${story.id} at ${width} px in the ${theme} theme`;
          const late = `${label} did not finish its checks within ${LIMIT / 1000} s`;
          expect
            .soft(await storybook.errorsOf(label), `story ${story.id} throws`)
            .toEqual([]);

          expect
            .soft(
              await within(
                page.evaluate(() => document.documentElement.className),
                late,
              ),
              `story ${story.id} is not in the ${theme} theme`,
            )
            .toBe(theme);

          await within(
            page.evaluate(() => document.fonts.ready),
            late,
          );
          const overflow = await within(
            page.evaluate(
              () =>
                document.documentElement.scrollWidth -
                document.documentElement.clientWidth,
            ),
            late,
          );
          expect
            .soft(overflow, `story ${story.id} scrolls sideways at ${width} px`)
            .toBeLessThanOrEqual(0);

          expect
            .soft(
              await within(blockingViolations(page), late),
              `axe violations in story ${story.id} at ${width} px`,
            )
            .toEqual([]);

          await within(
            snapshot?.add(page, {
              id: story.id,
              title: story.title,
              name: story.name,
            }) ?? Promise.resolve(),
            late,
          );

          const file = `${story.id}-${width}${suffix}.png`;
          await page.screenshot({
            path: path.join(OUT, file),
            fullPage: true,
            timeout: LIMIT,
          });
          // smallTargets scrolls, so it runs after the screenshot, in one theme.
          if (theme === "dark")
            expect
              .soft(
                await within(smallTargets(page), late),
                `targets under ${MIN_TARGET} px in story ${story.id} at ${width} px`,
              )
              .toEqual([]);
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

      await snapshot?.finish();
      expect(outside, "requests outside the Storybook folder").toEqual([]);
    });
  }
