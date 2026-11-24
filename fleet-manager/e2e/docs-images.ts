// The check that the guide pages, the docShot calls of the page scenarios,
// and the committed images agree. Playwright loads this module too, so it
// has no import.meta.
import fs from "node:fs";
import path from "node:path";

/** DOCS_PROJECT is the Playwright project in which docShot saves images. */
export const DOCS_PROJECT = "docs";

/** DOCS_IMAGES_DIR is the folder, relative to fleet-manager/, that a docs run writes. */
export const DOCS_IMAGES_DIR = "docs-images";

/** GUIDE_IMAGES_DIR is the folder of the guide images, relative to the repository root. */
export const GUIDE_IMAGES_DIR = "docs/fleet-manager/images";

/**
 * DOCS_NOW is the time of the page and of the app server in the docs
 * project, a little after the time of the scenario objects
 * (2026-01-01T00:00:00Z). An object that the API server stamps during the
 * run is newer than DOCS_NOW, so its age reads 0s.
 */
export const DOCS_NOW = "2026-01-03T10:00:00Z";

export const MAX_IMAGE_BYTES = 250 * 1024;
export const MAX_FOLDER_BYTES = 6 * 1024 * 1024;
export const IMAGE_WIDTH = 1440;
const MAX_ALT_WORDS = 20;

/** GuideImage is one `![alt](images/<name>.png)` of a guide page. */
export type GuideImage = { page: string; name: string; alt: string };

/** DocShot is one docShot call of a spec file, with the scenario of the file. */
export type DocShot = { file: string; scenario: string; name: string };

/** ImageFile is one PNG of GUIDE_IMAGES_DIR, without its extension. */
export type ImageFile = { name: string; bytes: number; width: number };

const IMAGE = /!\[([^\]]*)\]\(images\/([^)\s]+)\.png\)/g;
const CALL = /docShot\(\s*\w+\s*,\s*("([^"]*)"|[^,)]+)/g;
const SCENARIO = /scenarioName:\s*"([^"]+)"/;
const COMMENT =
  /<!-- Screenshots: make fleet-docs-screenshots, scenarios ([a-z0-9-]+(?:, [a-z0-9-]+)*) -->/;
const NAME = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

/** guideImages returns each image of the page that lives in images/. */
export function guideImages(page: string, markdown: string): GuideImage[] {
  return [...markdown.matchAll(IMAGE)].map((m) => ({
    page,
    name: m[2],
    alt: m[1],
  }));
}

/**
 * screenshotScenarios returns the sorted list of the comment
 * `<!-- Screenshots: make fleet-docs-screenshots, scenarios a, b -->`, or
 * undefined when the page has no such comment.
 */
export function screenshotScenarios(markdown: string): string[] | undefined {
  const m = COMMENT.exec(markdown);
  return m ? m[1].split(", ").sort() : undefined;
}

/**
 * docShots returns each docShot call of a spec file. A call whose name is
 * not a string literal, or a call in a file without test.use({
 * scenarioName }), is a problem and no shot.
 */
export function docShots(
  file: string,
  source: string,
): { shots: DocShot[]; problems: string[] } {
  const scenario = SCENARIO.exec(source)?.[1];
  const shots: DocShot[] = [];
  const problems: string[] = [];
  for (const m of source.matchAll(CALL)) {
    const name = m[2];
    if (name === undefined) {
      problems.push(
        `docShot in ${file} has a name that is not a string literal`,
      );
    } else if (scenario === undefined) {
      problems.push(
        `docShot in ${file} runs in a file that sets no scenarioName`,
      );
    } else {
      shots.push({ file, scenario, name });
    }
  }
  return { shots, problems };
}

/** docShotFiles returns the sorted base names of the spec files in dir that call docShot. */
export function docShotFiles(dir: string): string[] {
  return fs
    .readdirSync(dir)
    .filter((f) => f.endsWith(".spec.ts"))
    .filter((f) =>
      fs.readFileSync(path.join(dir, f), "utf8").includes("docShot("),
    )
    .sort();
}

/** pngWidth returns the width of a PNG from its first 24 bytes. */
export function pngWidth(head: Uint8Array): number {
  return new DataView(head.buffer, head.byteOffset, head.byteLength).getUint32(
    16,
  );
}

/**
 * imageProblems returns one sorted line for each disagreement between the
 * guide pages, the docShot calls of the spec files, and the image files. An
 * empty list means that they agree. It reports:
 *
 * - a page image that no docShot takes, and a docShot that no page shows;
 * - a name that two docShot calls take, a name that is not lowercase words
 *   joined by -, and a name that is not a string literal;
 * - a file that no docShot takes, and a docShot without a file;
 * - an empty alt text, an alt text equal to the name, and one over 20 words;
 * - a page whose scenario comment is missing or differs from the scenarios
 *   of its images, and a comment on a page without images;
 * - a file over MAX_IMAGE_BYTES or wider than IMAGE_WIDTH, and a folder over
 *   MAX_FOLDER_BYTES.
 */
export function imageProblems(input: {
  pages: { page: string; markdown: string }[];
  specs: { file: string; source: string }[];
  images: ImageFile[];
}): string[] {
  const problems: string[] = [];
  const byName = new Map<string, DocShot[]>();
  for (const { file, source } of input.specs) {
    const read = docShots(file, source);
    problems.push(...read.problems);
    for (const s of read.shots)
      byName.set(s.name, [...(byName.get(s.name) ?? []), s]);
  }
  for (const [name, shots] of byName) {
    const first = shots[0].file;
    if (shots.length > 1) {
      problems.push(
        `docShot "${name}" is taken in ${shots.map((s) => s.file).join(" and ")}`,
      );
    }
    if (!NAME.test(name)) {
      problems.push(
        `docShot "${name}" in ${first} is not a name of lowercase words joined by -`,
      );
    }
  }

  const shown = new Set<string>();
  for (const { page, markdown } of input.pages) {
    const images = guideImages(page, markdown);
    const scenarios = new Set<string>();
    for (const { name, alt } of images) {
      shown.add(name);
      const shots = byName.get(name);
      if (shots === undefined) {
        problems.push(
          `${page} names images/${name}.png, which no docShot takes`,
        );
      } else {
        for (const s of shots) scenarios.add(s.scenario);
      }
      const words = alt.trim() === "" ? 0 : alt.trim().split(/\s+/).length;
      if (words === 0 || alt.trim() === name) {
        problems.push(
          `${page}: images/${name}.png has no alt text that says what it shows`,
        );
      } else if (words > MAX_ALT_WORDS) {
        problems.push(
          `${page}: the alt text of images/${name}.png has ${words} words, more than ${MAX_ALT_WORDS}`,
        );
      }
    }
    const listed = screenshotScenarios(markdown);
    const from = [...scenarios].sort();
    if (images.length === 0 && listed !== undefined) {
      problems.push(`${page} names scenarios, but shows no image`);
    } else if (from.length > 0 && listed?.join(", ") !== from.join(", ")) {
      problems.push(
        `${page} names the scenarios ${listed?.join(", ") ?? "(none)"}, but its images come from ${from.join(", ")}`,
      );
    }
  }

  const files = new Set(input.images.map((i) => i.name));
  for (const [name, shots] of byName) {
    if (!shown.has(name)) {
      problems.push(
        `docShot "${name}" in ${shots[0].file} takes an image that no guide page shows`,
      );
    }
    if (!files.has(name)) {
      problems.push(
        `docShot "${name}" in ${shots[0].file} has no file in ${GUIDE_IMAGES_DIR}`,
      );
    }
  }
  let folderBytes = 0;
  for (const { name, bytes, width } of input.images) {
    folderBytes += bytes;
    if (!byName.has(name)) {
      problems.push(
        `${GUIDE_IMAGES_DIR}/${name}.png is in the folder, but no docShot takes it`,
      );
    }
    if (bytes > MAX_IMAGE_BYTES) {
      problems.push(
        `images/${name}.png has ${kib(bytes)} KiB, more than ${kib(MAX_IMAGE_BYTES)} KiB`,
      );
    }
    if (width > IMAGE_WIDTH) {
      problems.push(
        `images/${name}.png is ${width} px wide, more than ${IMAGE_WIDTH} px`,
      );
    }
  }
  if (folderBytes > MAX_FOLDER_BYTES) {
    problems.push(
      `${GUIDE_IMAGES_DIR} has ${kib(folderBytes)} KiB, more than ${kib(MAX_FOLDER_BYTES)} KiB`,
    );
  }
  return [...new Set(problems)].sort();
}

const kib = (bytes: number) => Math.ceil(bytes / 1024);
