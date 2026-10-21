/** One screenshot of a journey step, as shot() records it. */
export type JourneyShot = { journey: string; name: string; file: string };

const escape = (text: string) =>
  text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");

const slug = (text: string) =>
  text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");

/**
 * shotFile returns the file name of shot seq of a journey. seq starts at 1,
 * and the name sorts the shots of one journey in the order they were taken
 * up to 99 shots.
 */
export function shotFile(journey: string, seq: number, name: string): string {
  return `${slug(journey)}--${String(seq).padStart(2, "0")}-${slug(name)}.png`;
}

/**
 * renderJourneyIndex returns one HTML page with every shot: a section per
 * journey in first-seen order, and its shots in input order. The image paths
 * stay relative, so the page works from the artifact.
 */
export function renderJourneyIndex(shots: readonly JourneyShot[]): string {
  const journeys = new Map<string, JourneyShot[]>();
  for (const s of shots) {
    journeys.set(s.journey, [...(journeys.get(s.journey) ?? []), s]);
  }
  const sections = [...journeys.entries()].map(([journey, group]) => {
    const figures = group
      .map(
        (s) =>
          `<figure><img src="${escape(s.file)}" alt="${escape(s.name)}" loading="lazy"><figcaption>${escape(s.name)}</figcaption></figure>`,
      )
      .join("");
    return `<section><h2>${escape(journey)}</h2>${figures}</section>`;
  });
  const body =
    sections.length > 0
      ? sections.join("")
      : "<p>No journey took a screenshot.</p>";
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Fleet manager UI journeys</title><style>body{margin:0;padding:24px;background:#0a0f1e;color:#e8ecf5;font-family:system-ui,sans-serif}figure{margin:0 0 24px}img{display:block;max-width:100%;border:1px solid #1f2a55}figcaption{color:#a3adc2;font-size:12px;padding-top:4px}</style></head><body><h1>Fleet manager UI journeys</h1>${body}</body></html>`;
}
