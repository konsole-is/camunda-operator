/** One screenshot of one story at one width, as gallery.spec.ts records it. */
export type Shot = {
  id: string;
  component: string;
  name: string;
  width: number;
  file: string;
};

const escape = (text: string) =>
  text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");

/**
 * renderIndex returns one HTML page with every shot: a section per component
 * in first-seen order, a row per story, and its widths side by side, widest
 * first. The image paths stay relative, so the page works from the artifact.
 */
export function renderIndex(shots: readonly Shot[]): string {
  const components = new Map<string, Map<string, Shot[]>>();
  for (const s of shots) {
    const byStory = components.get(s.component) ?? new Map<string, Shot[]>();
    byStory.set(s.id, [...(byStory.get(s.id) ?? []), s]);
    components.set(s.component, byStory);
  }
  const sections = [...components.entries()].map(([component, byStory]) => {
    const rows = [...byStory.values()].map((group) => {
      const images = [...group]
        .sort((a, b) => b.width - a.width)
        .map(
          (s) =>
            `<figure><img src="${escape(s.file)}" alt="${escape(`${s.name} at ${s.width} px`)}" loading="lazy"><figcaption>${s.width} px</figcaption></figure>`,
        )
        .join("");
      return `<h3>${escape(group[0].name)}</h3><div class="row">${images}</div>`;
    });
    return `<section><h2>${escape(component)}</h2>${rows.join("")}</section>`;
  });
  const body =
    sections.length > 0
      ? sections.join("")
      : "<p>No stories were captured.</p>";
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Fleet manager UI gallery</title><style>body{margin:0;padding:24px;background:#0a0f1e;color:#e8ecf5;font-family:system-ui,sans-serif}.row{display:flex;flex-wrap:wrap;gap:16px;align-items:flex-start}figure{margin:0}img{display:block;max-width:100%;border:1px solid #1f2a55}figcaption{color:#a3adc2;font-size:12px;padding-top:4px}</style></head><body><h1>Fleet manager UI gallery</h1>${body}</body></html>`;
}
