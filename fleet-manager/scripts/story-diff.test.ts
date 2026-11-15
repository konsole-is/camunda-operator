import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { describe, expect, it } from "vitest";
import {
  type Comparison,
  type Limits,
  REPORT_LIMITS,
  type Snapshot,
  type SnapshotStory,
  commentLimits,
  compareSnapshots,
  missingSnapshot,
  readSnapshot,
  renderChanges,
  unifiedDiff,
} from "./story-diff.mjs";

const script = path.join(import.meta.dirname, "story-diff.mts");

function tempDir(prefix: string): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), prefix));
}

function write(root: string, file: string, text: string): void {
  fs.mkdirSync(path.dirname(path.join(root, file)), { recursive: true });
  fs.writeFileSync(path.join(root, file), text);
}

function story(id: string, markup: string): SnapshotStory {
  return { id, title: `ui/${id}`, name: "Default", markup };
}

function snapshot(stories: SnapshotStory[], stylesheet = ".a { }"): Snapshot {
  return {
    sha: "0123456789abcdef0123456789abcdef01234567",
    stories,
    stylesheet,
  };
}

/** Writes a snapshot folder as the stories check writes it. */
function snapshotDir(snap: Snapshot): string {
  const dir = tempDir("snapshot-");
  write(
    dir,
    "stories.json",
    JSON.stringify(
      snap.stories.map(({ id, title, name }) => ({ id, title, name })),
    ),
  );
  for (const s of snap.stories) write(dir, `stories/${s.id}.html`, s.markup);
  write(dir, "stylesheet.css", snap.stylesheet);
  write(dir, "sha", `${snap.sha}\n`);
  return dir;
}

const links = {
  main: "https://konsole-is.github.io/camunda-operator/storybook/",
  preview: "https://konsole-is.github.io/camunda-operator/pr-preview/pr-8/",
};

const limits: Limits = commentLimits("https://example.com/run");

describe("unifiedDiff", () => {
  it("is empty when the texts are equal", () => {
    expect(unifiedDiff("a\nb", "a\nb")).toBe("");
  });

  it("shows a changed line with the lines around it", () => {
    const before = ["1", "2", "3", "4", "5", "6", "7", "8"].join("\n");
    const after = ["1", "2", "3", "4", "five", "6", "7", "8"].join("\n");

    expect(unifiedDiff(before, after, 2)).toBe(
      ["@@ -3,5 +3,5 @@", " 3", " 4", "-5", "+five", " 6", " 7"].join("\n"),
    );
  });

  it("puts changes that are far apart into separate hunks", () => {
    const before = Array.from({ length: 20 }, (_, i) => `${i + 1}`);
    const after = [...before];
    after[1] = "two";
    after[17] = "eighteen";

    const diff = unifiedDiff(before.join("\n"), after.join("\n"), 1);

    expect(diff.match(/^@@ /gm)).toHaveLength(2);
    expect(diff).toContain("@@ -1,3 +1,3 @@");
    expect(diff).toContain("@@ -17,3 +17,3 @@");
  });

  it("counts added and removed lines in the hunk line", () => {
    expect(unifiedDiff("a\nc", "a\nb\nc", 0)).toBe("@@ -1,0 +2,1 @@\n+b");
    expect(unifiedDiff("a\nb\nc", "a\nc", 0)).toBe("@@ -2,1 +1,0 @@\n-b");
  });

  it("treats an empty text as no lines", () => {
    expect(unifiedDiff("", "a")).toBe("@@ -0,0 +1,1 @@\n+a");
    expect(unifiedDiff("a", "")).toBe("@@ -1,1 +0,0 @@\n-a");
  });

  it("keeps the common lines of a moved block", () => {
    const diff = unifiedDiff("x\na\nb\nc", "a\nb\nc\nx", 0);

    expect(diff.split("\n").filter((l) => /^[-+]/.test(l))).toEqual([
      "-x",
      "+x",
    ]);
  });
});

describe("compareSnapshots", () => {
  it("lists the changed, the new, and the removed stories", () => {
    const main = snapshot([
      story("a", "<p>a</p>"),
      story("b", "<p>b</p>"),
      story("gone", "<p>gone</p>"),
    ]);
    const head = snapshot([
      story("a", "<p>a</p>"),
      story("b", "<p>B</p>"),
      story("new", "<p>new</p>"),
    ]);

    const result = compareSnapshots(main, head);

    expect(result.changed.map((c) => c.story.id)).toEqual(["b"]);
    expect(result.changed[0].diff).toContain("-<p>b</p>\n+<p>B</p>");
    expect(result.added.map((s) => s.id)).toEqual(["new"]);
    expect(result.removed.map((s) => s.id)).toEqual(["gone"]);
    expect(result.total).toBe(3);
    expect(result.sha).toBe(main.sha);
  });

  it("finds no change between equal snapshots", () => {
    const snap = snapshot([story("a", "<p>a</p>")]);

    expect(compareSnapshots(snap, snapshot([...snap.stories]))).toMatchObject({
      changed: [],
      added: [],
      removed: [],
      stylesheet: "",
    });
  });

  it("diffs the stylesheet rules", () => {
    const result = compareSnapshots(
      snapshot([], ".a { color: red; }\n.b { }"),
      snapshot([], ".a { color: blue; }\n.b { }"),
    );

    expect(result.stylesheet).toContain(
      "-.a { color: red; }\n+.a { color: blue; }",
    );
  });

  it("keeps the story order of each snapshot", () => {
    const main = snapshot([story("z", "1"), story("y", "1"), story("x", "1")]);
    const head = snapshot([story("y", "2"), story("z", "2"), story("w", "1")]);

    const result = compareSnapshots(main, head);

    expect(result.changed.map((c) => c.story.id)).toEqual(["y", "z"]);
    expect(result.removed.map((s) => s.id)).toEqual(["x"]);
  });
});

function comparison(over: Partial<Comparison>): Comparison {
  return {
    sha: "0123456789abcdef",
    total: 5,
    changed: [],
    added: [],
    removed: [],
    stylesheet: "",
    ...over,
  };
}

describe("renderChanges", () => {
  it("says in one line that nothing changed, and names the commit of main", () => {
    const text = renderChanges(comparison({}), links, limits);

    expect(text.trim().split("\n")).toHaveLength(1);
    expect(text).toContain("`0123456`");
    expect(text).not.toContain("0123456789");
  });

  it("says that the commit of main is unknown when the snapshot does not name it", () => {
    expect(renderChanges(comparison({ sha: "" }), links, limits)).toContain(
      "main at an unknown commit,",
    );
  });

  it("counts the changed stories and links each on main and on the preview", () => {
    const changed = [
      {
        story: story("ui-button--primary", ""),
        diff: "@@ -1,1 +1,1 @@\n-a\n+b",
      },
    ];

    const text = renderChanges(comparison({ changed }), links, limits);

    expect(text).toContain("1 of 5 stories changed against main at `0123456`");
    expect(text).toContain(`(${links.main}?path=/story/ui-button--primary)`);
    expect(text).toContain(`(${links.preview}?path=/story/ui-button--primary)`);
    expect(text).toMatch(
      /<details><summary>[^<]+<\/summary>\n\n```diff\n@@ -1,1 \+1,1 @@\n-a\n\+b\n```\n\n<\/details>/,
    );
  });

  it("lists the new stories with their preview and the removed with main", () => {
    const text = renderChanges(
      comparison({
        added: [story("ui-new--default", "")],
        removed: [story("ui-old--default", "")],
      }),
      links,
      limits,
    );

    expect(text).toContain("0 of 5 stories changed");
    expect(text).toMatch(
      /New stories:\n\n- .*\(https:\/\/[^)]*pr-8\/\?path=\/story\/ui-new--default\)/,
    );
    expect(text).toMatch(
      /Removed stories:\n\n- .*\(https:\/\/[^)]*storybook\/\?path=\/story\/ui-old--default\)/,
    );
  });

  it("says that the stylesheet changed and shows the changed rules", () => {
    const text = renderChanges(
      comparison({ stylesheet: "@@ -1,1 +1,1 @@\n-.a { }\n+.b { }" }),
      links,
      limits,
    );

    expect(text).toContain(
      "The stylesheet changed, so any story may look different.",
    );
    expect(text).toContain("-.a { }\n+.b { }");
  });

  it("cuts a long diff and says that the full report has the rest", () => {
    const diff = [
      "@@ -1,100 +1,100 @@",
      ...Array.from({ length: 100 }, (_, i) => `+line ${i}`),
    ].join("\n");

    const text = renderChanges(
      comparison({ changed: [{ story: story("a", ""), diff }] }),
      links,
      { ...limits, diffLines: 10 },
    );

    expect(text).toContain("+line 8");
    expect(text).not.toContain("+line 9\n");
    expect(text).toContain("The full report has all of it.");
  });

  it("stays within the length and counts the stories that did not fit", () => {
    const changed = Array.from({ length: 50 }, (_, i) => ({
      story: story(`s-${i}`, ""),
      diff: `@@ -1,1 +1,1 @@\n-${"x".repeat(900)}\n+${"y".repeat(900)}`,
    }));

    const text = renderChanges(comparison({ total: 50, changed }), links, {
      ...limits,
      length: 10_000,
    });
    const shown = text.match(/<details>/g)?.length ?? 0;

    expect(text.length).toBeLessThan(10_000 + 200);
    expect(shown).toBeGreaterThan(0);
    expect(text).toContain(
      `And ${50 - shown} more: the [full report](https://example.com/run) lists them.`,
    );
  });

  it("shows every line of each diff in the report", () => {
    const diff = [
      "@@ -1,100 +1,100 @@",
      ...Array.from({ length: 100 }, (_, i) => `+line ${i}`),
    ].join("\n");

    const text = renderChanges(
      comparison({ changed: [{ story: story("a", ""), diff }] }),
      links,
      REPORT_LIMITS,
    );

    expect(text).toContain("+line 99");
    expect(text).not.toContain("full report");
  });

  it("puts the diff in a fence that a backtick run in it cannot close", () => {
    const diff = "@@ -1,1 +1,1 @@\n-```\n+````";

    const text = renderChanges(
      comparison({ changed: [{ story: story("a", ""), diff }] }),
      links,
      limits,
    );

    expect(text).toContain("`````diff\n");
  });
});

describe("missingSnapshot", () => {
  it("says which snapshot is missing", () => {
    expect(missingSnapshot("main")).toMatch(/snapshot of main/);
    expect(missingSnapshot("pull request")).toMatch(
      /stories check made no snapshot/,
    );
  });
});

describe("readSnapshot", () => {
  it("reads the snapshot that the stories check writes", () => {
    const snap = snapshot([story("a", "<p>a</p>"), story("b", "<p>b</p>")]);

    expect(readSnapshot(snapshotDir(snap))).toEqual(snap);
  });

  it("returns undefined for a folder without stories.json", () => {
    expect(readSnapshot(tempDir("empty-"))).toBeUndefined();
    expect(
      readSnapshot(path.join(tempDir("empty-"), "missing")),
    ).toBeUndefined();
  });

  it("reads a snapshot without a stylesheet and a commit", () => {
    const dir = tempDir("snapshot-");
    write(dir, "stories.json", '[{"id":"a","title":"ui/A","name":"Default"}]');
    write(dir, "stories/a.html", "<p>a</p>");

    expect(readSnapshot(dir)).toEqual({
      sha: "",
      stories: [story("a", "<p>a</p>")].map((s) => ({ ...s, title: "ui/A" })),
      stylesheet: "",
    });
  });

  it("throws when the markup of a listed story is missing", () => {
    const dir = tempDir("snapshot-");
    write(dir, "stories.json", '[{"id":"a","title":"ui/A","name":"Default"}]');

    expect(() => readSnapshot(dir)).toThrow();
  });

  it.each(["../a", "A", "a/b", ""])("refuses the story id %j", (id) => {
    const dir = tempDir("snapshot-");
    write(dir, "stories.json", JSON.stringify([{ id, title: "t", name: "n" }]));

    expect(() => readSnapshot(dir)).toThrow(/not the id of a story/);
  });

  it("throws when stories.json is not a list of stories", () => {
    const dir = tempDir("snapshot-");
    write(dir, "stories.json", '{"id":"a"}');
    expect(() => readSnapshot(dir)).toThrow(/not a list/);

    write(dir, "stories.json", '[{"id":"a"}]');
    expect(() => readSnapshot(dir)).toThrow(/without an id, title, or name/);
  });
});

describe("the command line", () => {
  function run(...args: string[]) {
    return spawnSync(process.execPath, [script, ...args], { encoding: "utf8" });
  }

  const main = snapshot([story("a", "<p>a</p>"), story("b", "<p>b</p>")]);
  const head = snapshot([story("a", "<p>a</p>"), story("b", "<p>B</p>")]);

  it("prints the changes for the comment", () => {
    const result = run(
      "comment",
      snapshotDir(main),
      snapshotDir(head),
      links.main,
      links.preview,
      "https://example.com/run",
    );

    expect(result.status).toBe(0);
    expect(result.stdout).toBe(
      renderChanges(
        compareSnapshots(main, head),
        links,
        commentLimits("https://example.com/run"),
      ),
    );
  });

  it("prints the full report", () => {
    const result = run(
      "report",
      snapshotDir(main),
      snapshotDir(head),
      links.main,
      links.preview,
    );

    expect(result.status).toBe(0);
    expect(result.stdout).toBe(
      renderChanges(compareSnapshots(main, head), links, REPORT_LIMITS),
    );
  });

  it("says so when there is no snapshot of main", () => {
    const result = run(
      "comment",
      path.join(tempDir("baseline-"), "missing"),
      snapshotDir(head),
      links.main,
      links.preview,
      "https://example.com/run",
    );

    expect(result.status).toBe(0);
    expect(result.stdout).toBe(missingSnapshot("main"));
  });

  it("says so when the pull request has no snapshot", () => {
    const result = run(
      "comment",
      snapshotDir(main),
      path.join(tempDir("snapshot-"), "missing"),
      links.main,
      links.preview,
      "https://example.com/run",
    );

    expect(result.status).toBe(0);
    expect(result.stdout).toBe(missingSnapshot("pull request"));
  });

  it("exits 1 and names a command that it does not know", () => {
    const result = run("diff");

    expect(result.status).toBe(1);
    expect(result.stderr).toContain('"diff"');
  });
});
