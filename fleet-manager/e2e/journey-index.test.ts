import { describe, expect, it } from "vitest";
import { renderJourneyIndex, shotFile } from "./journey-index";

describe("shotFile", () => {
  it("numbers the shots of a journey and keeps the name readable", () => {
    expect(shotFile("smoke › opens the fleet", 1, "Fleet page")).toBe(
      "smoke-opens-the-fleet--01-fleet-page.png",
    );
  });

  it("trims the separators at both ends of a name", () => {
    expect(shotFile("  Journey!", 12, "--Step--")).toBe("journey--12-step.png");
  });
});

describe("renderJourneyIndex", () => {
  it("lists the journeys and their shots in the order they were taken", () => {
    const html = renderJourneyIndex([
      { journey: "b", name: "first", file: "b--01-first.png" },
      { journey: "a", name: "second", file: "a--01-second.png" },
      { journey: "b", name: "third", file: "b--02-third.png" },
    ]);
    expect(html.indexOf("<h2>b</h2>")).toBeLessThan(html.indexOf("<h2>a</h2>"));
    expect(html.indexOf('src="b--01-first.png"')).toBeLessThan(
      html.indexOf('src="b--02-third.png"'),
    );
    expect(html.match(/<h2>b<\/h2>/g)).toHaveLength(1);
    expect(html).toContain("<figcaption>third</figcaption>");
  });

  it("escapes names", () => {
    const html = renderJourneyIndex([
      { journey: "<j>", name: "<x>", file: "a&b.png" },
    ]);
    expect(html).toContain("<h2>&lt;j&gt;</h2>");
    expect(html).toContain("<figcaption>&lt;x&gt;</figcaption>");
    expect(html).toContain('src="a&amp;b.png"');
    expect(html).not.toContain("<x>");
  });

  it("says so when no journey took a shot", () => {
    expect(renderJourneyIndex([])).toContain("No journey took a screenshot");
  });
});
