import { describe, expect, it } from "vitest";
import { type Shot, renderIndex } from "./stories-index";

const shot = (
  id: string,
  component: string,
  name: string,
  width: number,
): Shot => ({
  id,
  component,
  name,
  width,
  theme: "dark",
  file: `${id}-${width}.png`,
});

describe("renderIndex", () => {
  it("shows both widths of each story, grouped by component", () => {
    const html = renderIndex([
      shot("statuschip/ready", "StatusChip", "Ready", 1440),
      shot("logo/default", "Logo", "Default", 1440),
      shot("statuschip/ready", "StatusChip", "Ready", 390),
    ]);
    expect(html).toContain('src="statuschip/ready-1440.png"');
    expect(html).toContain('src="statuschip/ready-390.png"');
    expect(html.indexOf("<h2>StatusChip</h2>")).toBeLessThan(
      html.indexOf("<h2>Logo</h2>"),
    );
    expect(html.match(/<h3>Ready<\/h3>/g)).toHaveLength(1);
  });

  it("puts the light shot of a width after its dark shot and names it", () => {
    const light = (width: number): Shot => ({
      ...shot("logo/default", "Logo", "Default", width),
      theme: "light",
      file: `logo/default-${width}-light.png`,
    });
    const html = renderIndex([
      light(390),
      light(1440),
      shot("logo/default", "Logo", "Default", 390),
      shot("logo/default", "Logo", "Default", 1440),
    ]);
    const order = [...html.matchAll(/src="([^"]+)"/g)].map((m) => m[1]);
    expect(order).toEqual([
      "logo/default-1440.png",
      "logo/default-1440-light.png",
      "logo/default-390.png",
      "logo/default-390-light.png",
    ]);
    expect(html).toContain("<figcaption>1440 px, light</figcaption>");
    expect(html).toContain("<figcaption>1440 px</figcaption>");
  });

  it("escapes names", () => {
    const html = renderIndex([shot("a/b", "<A>", 'B "x"', 390)]);
    expect(html).toContain("&lt;A&gt;");
    expect(html).toContain("B &quot;x&quot;");
    expect(html).not.toContain("<A>");
  });

  it("names the stories in its title and heading", () => {
    const html = renderIndex([]);
    expect(html).toContain("<title>Fleet manager stories</title>");
    expect(html).toContain("<h1>Fleet manager stories</h1>");
  });

  it("says so when there are no shots", () => {
    expect(renderIndex([])).toContain("No stories");
  });
});
