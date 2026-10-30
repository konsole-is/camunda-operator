import { appendFile, mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { type Page, test } from "@playwright/test";
import {
  type JourneyShot,
  renderJourneyIndex,
  shotFile,
} from "./journey-index";

/** JOURNEYS_DIR is the directory of the ui-journeys artifact. */
export const JOURNEYS_DIR = path.resolve("ui-journeys");

/** PAGES_DIR is the directory of the ui-pages artifact of the page scenarios. */
export const PAGES_DIR = path.resolve("ui-pages");

const SHOTS_FILE = "shots.jsonl";

const counts = new Map<string, number>();

/**
 * shot saves a full-page PNG of the page into options.dir and records it for
 * the index that writeShotIndex writes there. Call it from a test. The
 * images are not compared with a baseline.
 */
export async function shot(
  page: Page,
  name: string,
  options: { dir: string },
): Promise<void> {
  const info = test.info();
  const seq = (counts.get(info.testId) ?? 0) + 1;
  counts.set(info.testId, seq);
  const journey = info.titlePath.slice(1).join(" › ");
  const file = shotFile(journey, seq, name);

  await mkdir(options.dir, { recursive: true });
  await page.screenshot({ path: path.join(options.dir, file), fullPage: true });
  const entry: JourneyShot = { journey, name, file };
  await appendFile(
    path.join(options.dir, SHOTS_FILE),
    `${JSON.stringify(entry)}\n`,
  );
}

/**
 * writeShotIndex writes dir/index.html with every shot that shot recorded in
 * dir, under title. It writes the page also when no test took a shot.
 */
export async function writeShotIndex(
  dir: string,
  title: string,
): Promise<void> {
  await mkdir(dir, { recursive: true });
  const lines = await readFile(path.join(dir, SHOTS_FILE), "utf8").catch(
    (err: NodeJS.ErrnoException) => {
      if (err.code === "ENOENT") return "";
      throw err;
    },
  );
  const shots = lines
    .split("\n")
    .filter((line) => line !== "")
    .map((line) => JSON.parse(line) as JourneyShot);
  await writeFile(
    path.join(dir, "index.html"),
    renderJourneyIndex(shots, title),
  );
}
