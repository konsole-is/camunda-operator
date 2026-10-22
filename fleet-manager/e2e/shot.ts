import { appendFile, mkdir } from "node:fs/promises";
import path from "node:path";
import { type Page, test } from "@playwright/test";
import { type JourneyShot, shotFile } from "./journey-index";

/** OUT is the directory of the ui-journeys artifact. */
export const OUT = path.resolve("ui-journeys");

const counts = new Map<string, number>();

/**
 * shot saves a full-page PNG of the page into ui-journeys and records it for
 * the index of the artifact. Call it from a test of a journey. The images are
 * not compared with a baseline.
 */
export async function shot(page: Page, name: string): Promise<void> {
  const info = test.info();
  const seq = (counts.get(info.testId) ?? 0) + 1;
  counts.set(info.testId, seq);
  const journey = info.titlePath.slice(1).join(" › ");
  const file = shotFile(journey, seq, name);

  await mkdir(OUT, { recursive: true });
  await page.screenshot({ path: path.join(OUT, file), fullPage: true });
  const entry: JourneyShot = { journey, name, file };
  await appendFile(path.join(OUT, "shots.jsonl"), `${JSON.stringify(entry)}\n`);
}
