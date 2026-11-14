/**
 * One screenshot of one story at one width, as gallery.spec.ts and
 * stories.spec.ts record it.
 */
export type Shot = {
  id: string;
  component: string;
  name: string;
  width: number;
  /** theme is absent for a shot of the gallery, which has only dark. */
  theme?: "dark" | "light";
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
 * first, with the dark shot of a width before its light shot. The caption of
 * a light shot names the theme. The image paths stay relative, so the page works from the artifact.
 * title is the title and the heading of the page.
 */
export function renderIndex(
  shots: readonly Shot[],
  title = "Fleet manager UI gallery",
): string {
  const components = new Map<string, Map<string, Shot[]>>();
  for (const s of shots) {
    const byStory = components.get(s.component) ?? new Map<string, Shot[]>();
    byStory.set(s.id, [...(byStory.get(s.id) ?? []), s]);
    components.set(s.component, byStory);
  }
  const sections = [...components.entries()].map(([component, byStory]) => {
    const rows = [...byStory.values()].map((group) => {
      const images = [...group]
        .sort(
          (a, b) =>
            b.width - a.width ||
            Number(a.theme === "light") - Number(b.theme === "light"),
        )
        .map((s) => {
          const caption = `${s.width} px${s.theme === "light" ? ", light" : ""}`;
          return `<figure><img src="${escape(s.file)}" alt="${escape(`${s.name} at ${caption}`)}" loading="lazy"><figcaption>${caption}</figcaption></figure>`;
        })
        .join("");
      return `<h3>${escape(group[0].name)}</h3><div class="row">${images}</div>`;
    });
    return `<section><h2>${escape(component)}</h2>${rows.join("")}</section>`;
  });
  const body =
    sections.length > 0
      ? sections.join("")
      : "<p>No stories were captured.</p>";
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>${escape(title)}</title><style>body{margin:0;padding:24px;background:#0a0f1e;color:#e8ecf5;font-family:system-ui,sans-serif}.row{display:flex;flex-wrap:wrap;gap:16px;align-items:flex-start}figure{margin:0}img{display:block;max-width:100%;border:1px solid #1f2a55}figcaption{color:#a3adc2;font-size:12px;padding-top:4px}</style></head><body><h1>${escape(title)}</h1>${body}</body></html>`;
}
