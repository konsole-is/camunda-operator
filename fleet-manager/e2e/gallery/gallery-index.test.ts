import { describe, expect, it } from "vitest";
import { type Shot, renderIndex } from "./gallery-index";

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

  it("escapes names", () => {
    const html = renderIndex([shot("a/b", "<A>", 'B "x"', 390)]);
    expect(html).toContain("&lt;A&gt;");
    expect(html).toContain("B &quot;x&quot;");
    expect(html).not.toContain("<A>");
  });

  it("says so when there are no shots", () => {
    expect(renderIndex([])).toContain("No stories");
  });
});
