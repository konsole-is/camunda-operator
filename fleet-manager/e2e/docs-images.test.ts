import { describe, expect, it } from "vitest";
import {
  docShots,
  guideImages,
  imageProblems,
  pngWidth,
  screenshotScenarios,
} from "./docs-images";

const SPEC = `test.use({ scenarioName: "claim-ready" });
test("x", async ({ page }) => {
  await docShot(page, "fleet-view-environment-tab", { ready: row });
});`;
const PAGE = `# The fleet view
<!-- Screenshots: make fleet-docs-screenshots, scenarios claim-ready -->

![The Fleet page on the Production tab.](images/fleet-view-environment-tab.png)
`;
const IMAGE = {
  name: "fleet-view-environment-tab",
  bytes: 90_000,
  width: 1440,
};
const agree = {
  pages: [{ page: "fleet-view.md", markdown: PAGE }],
  specs: [{ file: "claim-ready.spec.ts", source: SPEC }],
  images: [IMAGE],
};

describe("imageProblems", () => {
  it("finds no problem when the guide, the docShot calls, and the folder agree", () => {
    expect(imageProblems(agree)).toEqual([]);
  });

  it("names a page image that no docShot takes", () => {
    expect(imageProblems({ ...agree, specs: [] })).toContain(
      "fleet-view.md names images/fleet-view-environment-tab.png, which no docShot takes",
    );
  });

  it("names a docShot image that no page shows", () => {
    expect(imageProblems({ ...agree, pages: [] })).toContain(
      'docShot "fleet-view-environment-tab" in claim-ready.spec.ts takes an image that no guide page shows',
    );
  });

  it("names a folder file that no docShot takes", () => {
    const old = { name: "old-shot", bytes: 1_000, width: 1440 };
    expect(imageProblems({ ...agree, images: [IMAGE, old] })).toContain(
      "docs/fleet-manager/images/old-shot.png is in the folder, but no docShot takes it",
    );
  });

  it("names a docShot image that the folder lacks", () => {
    expect(imageProblems({ ...agree, images: [] })).toContain(
      'docShot "fleet-view-environment-tab" in claim-ready.spec.ts has no file in docs/fleet-manager/images',
    );
  });

  it("refuses a name that two docShot calls take", () => {
    const specs = [
      { file: "a.spec.ts", source: SPEC },
      { file: "b.spec.ts", source: SPEC },
    ];
    expect(imageProblems({ ...agree, specs })).toContain(
      'docShot "fleet-view-environment-tab" is taken in a.spec.ts and b.spec.ts',
    );
  });

  it("refuses a name that is not lowercase words joined by -", () => {
    const source = SPEC.replace("fleet-view-environment-tab", "Fleet_View");
    const markdown = PAGE.replace("fleet-view-environment-tab", "Fleet_View");
    expect(
      imageProblems({
        pages: [{ page: "fleet-view.md", markdown }],
        specs: [{ file: "claim-ready.spec.ts", source }],
        images: [{ ...IMAGE, name: "Fleet_View" }],
      }),
    ).toContain(
      'docShot "Fleet_View" in claim-ready.spec.ts is not a name of lowercase words joined by -',
    );
  });

  it("refuses an empty alt text and an alt text equal to the name", () => {
    const empty = PAGE.replace("The Fleet page on the Production tab.", "");
    const named = PAGE.replace(
      "The Fleet page on the Production tab.",
      "fleet-view-environment-tab",
    );
    const problems = imageProblems({
      ...agree,
      pages: [
        { page: "a.md", markdown: empty },
        { page: "b.md", markdown: named },
      ],
    });
    expect(problems).toContain(
      "a.md: images/fleet-view-environment-tab.png has no alt text that says what it shows",
    );
    expect(problems).toContain(
      "b.md: images/fleet-view-environment-tab.png has no alt text that says what it shows",
    );
  });

  it("refuses an alt text of more than 20 words", () => {
    const long = Array.from({ length: 21 }, (_, i) => `word${i}`).join(" ");
    const markdown = PAGE.replace(
      "The Fleet page on the Production tab.",
      long,
    );
    expect(
      imageProblems({
        ...agree,
        pages: [{ page: "fleet-view.md", markdown }],
      }),
    ).toContain(
      "fleet-view.md: the alt text of images/fleet-view-environment-tab.png has 21 words, more than 20",
    );
  });

  it("accepts an alt text of exactly 20 words", () => {
    const words = Array.from({ length: 20 }, (_, i) => `word${i}`).join(" ");
    const markdown = PAGE.replace(
      "The Fleet page on the Production tab.",
      words,
    );
    expect(
      imageProblems({
        ...agree,
        pages: [{ page: "fleet-view.md", markdown }],
      }),
    ).toEqual([]);
  });

  it("refuses a scenario comment that differs from the scenarios of the images", () => {
    const markdown = PAGE.replace(
      "scenarios claim-ready",
      "scenarios cluster-ready",
    );
    expect(
      imageProblems({
        ...agree,
        pages: [{ page: "fleet-view.md", markdown }],
      }),
    ).toContain(
      "fleet-view.md names the scenarios cluster-ready, but its images come from claim-ready",
    );
  });

  it("refuses a page with images and no scenario comment", () => {
    const markdown = PAGE.replace(/<!--.*-->\n/, "");
    expect(
      imageProblems({
        ...agree,
        pages: [{ page: "fleet-view.md", markdown }],
      }),
    ).toContain(
      "fleet-view.md names the scenarios (none), but its images come from claim-ready",
    );
  });

  it("refuses a scenario comment on a page without images", () => {
    const other = {
      page: "install.md",
      markdown:
        "# Install\n<!-- Screenshots: make fleet-docs-screenshots, scenarios claim-ready -->\n",
    };
    expect(
      imageProblems({ ...agree, pages: [...agree.pages, other] }),
    ).toContain("install.md names scenarios, but shows no image");
  });

  it("refuses an image over 250 KiB and a folder over 6 MiB", () => {
    const big = { ...IMAGE, bytes: 251 * 1024 };
    expect(imageProblems({ ...agree, images: [big] })).toContain(
      "images/fleet-view-environment-tab.png has 251 KiB, more than 250 KiB",
    );

    const names = Array.from({ length: 26 }, (_, i) => `shot-${i}`);
    const source = `test.use({ scenarioName: "claim-ready" });\n${names
      .map((n) => `await docShot(page, "${n}", { ready: row });`)
      .join("\n")}`;
    const markdown = `# Page\n<!-- Screenshots: make fleet-docs-screenshots, scenarios claim-ready -->\n\n${names
      .map((n) => `![The page with ${n}.](images/${n}.png)`)
      .join("\n\n")}\n`;
    expect(
      imageProblems({
        pages: [{ page: "page.md", markdown }],
        specs: [{ file: "claim-ready.spec.ts", source }],
        images: names.map((name) => ({ name, bytes: 240 * 1024, width: 1440 })),
      }),
    ).toEqual(["docs/fleet-manager/images has 6240 KiB, more than 6144 KiB"]);
  });

  it("refuses an image wider than 1440 px", () => {
    const wide = { ...IMAGE, width: 2880 };
    expect(imageProblems({ ...agree, images: [wide] })).toContain(
      "images/fleet-view-environment-tab.png is 2880 px wide, more than 1440 px",
    );
  });

  it("returns the problems sorted", () => {
    const problems = imageProblems({
      pages: agree.pages,
      specs: [],
      images: [],
    });
    expect(problems).toEqual([...problems].sort());
  });
});

describe("docShots", () => {
  it("reads each literal name with the scenario of the file", () => {
    expect(docShots("claim-ready.spec.ts", SPEC).shots).toEqual([
      {
        file: "claim-ready.spec.ts",
        scenario: "claim-ready",
        name: "fleet-view-environment-tab",
      },
    ]);
  });

  it("refuses a docShot name that is not a string literal", () => {
    expect(
      docShots(
        "a.spec.ts",
        'test.use({ scenarioName: "s" });\nawait docShot(page, name, {});',
      ).problems,
    ).toEqual(["docShot in a.spec.ts has a name that is not a string literal"]);
  });

  it("refuses a template literal as a docShot name", () => {
    expect(
      docShots(
        "a.spec.ts",
        'test.use({ scenarioName: "s" });\nawait docShot(page, `x-${n}`, {});',
      ).problems,
    ).toEqual(["docShot in a.spec.ts has a name that is not a string literal"]);
  });

  it("reads a call split over lines as prettier writes it", () => {
    const source = `test.use({ scenarioName: "claim-ready" });
  await docShot(
    page,
    "cluster-claim-tab",
    { ready: button },
  );`;
    expect(docShots("x.spec.ts", source)).toEqual({
      shots: [
        {
          file: "x.spec.ts",
          scenario: "claim-ready",
          name: "cluster-claim-tab",
        },
      ],
      problems: [],
    });
  });

  it("refuses a docShot in a file without a scenario", () => {
    expect(
      docShots("a.spec.ts", 'await docShot(page, "x", {});').problems,
    ).toEqual([
      "docShot in a.spec.ts runs in a file that sets no scenarioName",
    ]);
  });

  it("ignores the import of docShot", () => {
    const source = `import { docShot } from "./docs-shot";\n${SPEC}`;
    expect(docShots("a.spec.ts", source).shots).toHaveLength(1);
  });
});

describe("guideImages and screenshotScenarios", () => {
  it("reads the alt text and the name of each image of the page", () => {
    expect(guideImages("fleet-view.md", PAGE)).toEqual([
      {
        page: "fleet-view.md",
        name: "fleet-view-environment-tab",
        alt: "The Fleet page on the Production tab.",
      },
    ]);
  });

  it("ignores an image outside images/ and a link that is not an image", () => {
    const markdown =
      "![A diagram.](../assets/flow.png)\n[a link](images/x.png)\n";
    expect(guideImages("p.md", markdown)).toEqual([]);
  });

  it("reads the sorted scenario list of the comment, and undefined without one", () => {
    expect(
      screenshotScenarios(
        "# P\n<!-- Screenshots: make fleet-docs-screenshots, scenarios cluster-ready, claim-ready -->\n",
      ),
    ).toEqual(["claim-ready", "cluster-ready"]);
    expect(screenshotScenarios("# P\n")).toBeUndefined();
  });
});

it("pngWidth reads the width from the IHDR chunk", () => {
  const head = new Uint8Array(24);
  head.set([0x00, 0x00, 0x05, 0xa0], 16); // 1440
  expect(pngWidth(head)).toBe(1440);
});
