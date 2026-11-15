// Compares the story snapshot of a pull request with the story snapshot of
// main, and writes the part of the preview comment that lists the stories
// that changed, or the full report for the job summary. The stories check
// writes a snapshot. The workflows run this script with plain Node.js and no
// install, so it imports Node.js built-ins only.
//
// Usage:
//   node scripts/story-diff.mts comment <main snapshot> <snapshot> <main url> <preview url> <report url>
//   node scripts/story-diff.mts report <main snapshot> <snapshot> <main url> <preview url>
import fs from "node:fs";
import path from "node:path";

/** SnapshotStory is one story of a snapshot with its rendered markup. */
export type SnapshotStory = {
  id: string;
  title: string;
  name: string;
  markup: string;
};

/**
 * Snapshot is the rendered markup of each story of a Storybook build and the
 * rules of its stylesheet. sha is the commit of the build, or empty when the
 * snapshot does not name it. complete is false when the stories check
 * stopped before it captured the last story, so the snapshot lacks stories
 * that the build has.
 */
export type Snapshot = {
  sha: string;
  complete: boolean;
  stories: SnapshotStory[];
  stylesheet: string;
};

/** StoryChange is a story whose markup differs, with a unified diff of it. */
export type StoryChange = { story: SnapshotStory; diff: string };

/**
 * Comparison is the result of compareSnapshots. sha is the commit of the
 * snapshot of main. total counts the stories of the pull request. stylesheet
 * is a unified diff of the stylesheet rules, empty when they are the same.
 */
export type Comparison = {
  sha: string;
  total: number;
  changed: StoryChange[];
  added: SnapshotStory[];
  removed: SnapshotStory[];
  stylesheet: string;
};

/** Links holds the Storybook of main and the preview of the pull request. */
export type Links = { main: string; preview: string };

/**
 * Limits caps renderChanges. diffLines and diffLength cap each diff. length
 * is counted in UTF-8 bytes. more ends the line that counts what did not fit.
 */
export type Limits = {
  diffLines: number;
  diffLength: number;
  length: number;
  more: string;
};

const STORY_ID = /^[a-z0-9][a-z0-9-]*$/;

// Storybook writes an id from the title and the name of a story, in lower
// case with dashes, so a valid id is also a safe file name.
function storyFile(dir: string, id: string): string {
  if (!STORY_ID.test(id)) {
    throw new Error(`${JSON.stringify(id)} is not the id of a story`);
  }
  return path.join(dir, "stories", `${id}.html`);
}

function readOptional(file: string): string {
  return fs.existsSync(file) ? fs.readFileSync(file, "utf8") : "";
}

/**
 * readSnapshot reads the snapshot in dir: stories.json, stories/<id>.html for
 * each story, and the optional stylesheet.css and sha. It returns undefined
 * when dir has no stories.json. It throws when stories.json does not list
 * stories, or when the markup of a story in it is missing.
 */
export function readSnapshot(dir: string): Snapshot | undefined {
  const index = path.join(dir, "stories.json");
  if (!fs.existsSync(index)) return undefined;
  const parsed: unknown = JSON.parse(fs.readFileSync(index, "utf8"));
  const { complete, stories: entries } = (parsed ?? {}) as Record<
    string,
    unknown
  >;
  if (!Array.isArray(entries)) {
    throw new Error(`${index} is not a list of stories`);
  }
  const stories = entries.map((entry: Record<string, unknown>) => {
    const { id, title, name } = entry ?? {};
    if (
      typeof id !== "string" ||
      typeof title !== "string" ||
      typeof name !== "string"
    ) {
      throw new Error(`${index} has an entry without an id, title, or name`);
    }
    return {
      id,
      title,
      name,
      markup: fs.readFileSync(storyFile(dir, id), "utf8"),
    };
  });
  return {
    sha: readOptional(path.join(dir, "sha")).trim(),
    complete: complete === true,
    stories,
    stylesheet: readOptional(path.join(dir, "stylesheet.css")),
  };
}

/**
 * compareSnapshots returns the stories of head whose markup differs from main,
 * the stories that only head has, the stories that only main has, and a diff
 * of the stylesheet rules. Each list keeps the story order of its snapshot.
 */
export function compareSnapshots(main: Snapshot, head: Snapshot): Comparison {
  const before = new Map(main.stories.map((s) => [s.id, s]));
  const ids = new Set(head.stories.map((s) => s.id));
  const changed: StoryChange[] = [];
  const added: SnapshotStory[] = [];
  for (const story of head.stories) {
    const old = before.get(story.id);
    if (!old) added.push(story);
    else if (old.markup !== story.markup) {
      changed.push({ story, diff: unifiedDiff(old.markup, story.markup) });
    }
  }
  return {
    sha: main.sha,
    total: head.stories.length,
    changed,
    added,
    removed: main.stories.filter((s) => !ids.has(s.id)),
    stylesheet: unifiedDiff(main.stylesheet, head.stylesheet),
  };
}

type Op = { kind: " " | "-" | "+"; line: string };

// Above this many cells, the table of the longest common subsequence costs
// too much memory, and the middle part shows as removed and then added.
const MAX_TABLE = 16_000_000;

function middleOps(a: string[], b: string[]): Op[] {
  const n = a.length;
  const m = b.length;
  if ((n + 1) * (m + 1) > MAX_TABLE) {
    return [
      ...a.map((line): Op => ({ kind: "-", line })),
      ...b.map((line): Op => ({ kind: "+", line })),
    ];
  }
  const width = m + 1;
  const common = new Uint32Array((n + 1) * width);
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      common[i * width + j] =
        a[i] === b[j]
          ? common[(i + 1) * width + j + 1] + 1
          : Math.max(common[(i + 1) * width + j], common[i * width + j + 1]);
    }
  }
  const ops: Op[] = [];
  let i = 0;
  let j = 0;
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      ops.push({ kind: " ", line: a[i] });
      i++;
      j++;
    } else if (common[(i + 1) * width + j] >= common[i * width + j + 1]) {
      ops.push({ kind: "-", line: a[i++] });
    } else {
      ops.push({ kind: "+", line: b[j++] });
    }
  }
  while (i < n) ops.push({ kind: "-", line: a[i++] });
  while (j < m) ops.push({ kind: "+", line: b[j++] });
  return ops;
}

/**
 * unifiedDiff returns the lines of before and after as a unified diff, with
 * context lines of context around each change and a "@@ -l,n +l,n @@" line
 * before each hunk. It returns an empty string when the two are equal.
 */
export function unifiedDiff(
  before: string,
  after: string,
  context = 3,
): string {
  if (before === after) return "";
  const a = before === "" ? [] : before.split("\n");
  const b = after === "" ? [] : after.split("\n");
  let start = 0;
  while (start < a.length && start < b.length && a[start] === b[start]) {
    start++;
  }
  let endA = a.length;
  let endB = b.length;
  while (endA > start && endB > start && a[endA - 1] === b[endB - 1]) {
    endA--;
    endB--;
  }
  const ops: Op[] = [
    ...a.slice(0, start).map((line): Op => ({ kind: " ", line })),
    ...middleOps(a.slice(start, endA), b.slice(start, endB)),
    ...a.slice(endA).map((line): Op => ({ kind: " ", line })),
  ];
  // The line of a and of b that each op starts at, counted from 0.
  const lineA: number[] = [];
  const lineB: number[] = [];
  let atA = 0;
  let atB = 0;
  for (const op of ops) {
    lineA.push(atA);
    lineB.push(atB);
    if (op.kind !== "+") atA++;
    if (op.kind !== "-") atB++;
  }
  const hunks: [number, number][] = [];
  ops.forEach((op, index) => {
    if (op.kind === " ") return;
    const from = Math.max(0, index - context);
    const to = Math.min(ops.length, index + context + 1);
    const last = hunks.at(-1);
    if (last && from <= last[1]) last[1] = Math.max(last[1], to);
    else hunks.push([from, to]);
  });
  const out: string[] = [];
  for (const [from, to] of hunks) {
    const part = ops.slice(from, to);
    const countA = part.filter((op) => op.kind !== "+").length;
    const countB = part.filter((op) => op.kind !== "-").length;
    // A side with no lines names the line before the hunk.
    const startA = countA === 0 ? lineA[from] : lineA[from] + 1;
    const startB = countB === 0 ? lineB[from] : lineB[from] + 1;
    out.push(`@@ -${startA},${countA} +${startB},${countB} @@`);
    for (const op of part) out.push(op.kind + op.line);
  }
  return out.join("\n");
}

function storyLink(base: string, id: string): string {
  return `${base.endsWith("/") ? base : `${base}/`}?path=/story/${id}`;
}

function label(story: SnapshotStory): string {
  return `**${story.title}: ${story.name}**`;
}

// A fence longer than each run of backticks in the text, so that the text
// cannot close it.
function fenced(kind: string, text: string): string {
  const longest = Math.max(
    0,
    ...[...text.matchAll(/`+/g)].map((m) => m[0].length),
  );
  const fence = "`".repeat(Math.max(3, longest + 1));
  return `${fence}${kind}\n${text}\n${fence}`;
}

function cut(diff: string, limits: Limits): { text: string; cut: boolean } {
  const lines = diff.split("\n");
  const kept: string[] = [];
  let length = 0;
  for (const line of lines) {
    if (kept.length >= limits.diffLines) break;
    if (length + line.length + 1 > limits.diffLength) break;
    kept.push(line);
    length += line.length + 1;
  }
  return { text: kept.join("\n"), cut: kept.length < lines.length };
}

function details(summary: string, diff: string, limits: Limits): string {
  const { text, cut: wasCut } = cut(diff, limits);
  const tail = wasCut
    ? "\n\nThe diff is longer than this. The full report has all of it."
    : "";
  return `<details><summary>${summary}</summary>\n\n${fenced("diff", text)}${tail}\n\n</details>`;
}

/**
 * renderChanges returns the Markdown that lists the result of a comparison:
 * a count of the changed stories against the commit of main, a note when the
 * stylesheet changed with a diff of its rules, each changed story with a
 * link to it on main and on the preview and a diff of its markup, and the new
 * and the removed stories. When nothing differs, it returns one line that
 * says so. Only the first line, the stylesheet note, the sentences that say
 * what did not fit, and the line break at the end can make the text longer
 * than limits.length bytes. When a story does not fit, the last
 * sentence counts the stories that did not fit.
 */
export function renderChanges(
  comparison: Comparison,
  links: Links,
  limits: Limits,
): string {
  const { sha, total, changed, added, removed, stylesheet } = comparison;
  const at = sha
    ? `main at \`${sha.slice(0, 7)}\``
    : "main at an unknown commit";
  if (!changed.length && !added.length && !removed.length && !stylesheet) {
    return `**Story changes:** none. Each story renders the same as on ${at}, and the stylesheet is the same.\n`;
  }
  let text = `**Story changes:** ${changed.length} of ${total} ${total === 1 ? "story" : "stories"} changed against ${at}.`;
  let full = false;
  let bytes = Buffer.byteLength(text);
  // fits adds piece when it fits in the length, and adds nothing after a
  // piece that did not fit, so the text keeps the order of the comparison.
  const fits = (piece: string): boolean => {
    const size = Buffer.byteLength(piece);
    full ||= bytes + size > limits.length;
    if (!full) {
      text += piece;
      bytes += size;
    }
    return !full;
  };
  if (stylesheet) {
    const note = "\n\nThe stylesheet changed, so any story may look different.";
    text += note;
    bytes += Buffer.byteLength(note);
    if (!fits(`\n${details("Changed rules", stylesheet, limits)}`)) {
      text += ` The changed rules do not fit here: ${limits.more}`;
    }
  }
  let omitted = 0;
  for (const { story, diff } of changed) {
    const piece = `\n\n${label(story)}: [main](${storyLink(links.main, story.id)}), [preview](${storyLink(links.preview, story.id)})\n${details("Markup diff", diff, limits)}`;
    if (!fits(piece)) omitted++;
  }
  const list = (
    heading: string,
    stories: SnapshotStory[],
    name: "main" | "preview",
  ) => {
    stories.forEach((s, i) => {
      const line = `- ${label(s)}: [${name}](${storyLink(links[name], s.id)})`;
      if (!fits(i === 0 ? `\n\n${heading}\n\n${line}` : `\n${line}`)) {
        omitted++;
      }
    });
  };
  list("New stories:", added, "preview");
  list("Removed stories:", removed, "main");
  if (omitted > 0) {
    text += `\n\nAnd ${omitted} more ${omitted === 1 ? "story" : "stories"}: ${limits.more}`;
  }
  return `${text}\n`;
}

/**
 * missingSnapshot returns the line that replaces the list of changes when
 * the snapshot of main or of the pull request is missing or not complete.
 */
export function missingSnapshot(which: "main" | "pull request"): string {
  return which === "main"
    ? "**Story changes:** unknown. No complete story snapshot of main was found, so the stories were not compared. A successful run of the workflow on main makes one.\n"
    : "**Story changes:** unknown. The stories check made no complete snapshot, so the stories were not compared.\n";
}

// GitHub refuses a comment of more than 65536 characters. The preview part of
// the comment comes before this text.
const COMMENT_LENGTH = 60_000;
// GitHub refuses a job summary of more than 1 MiB.
const REPORT_LENGTH = 900_000;

/**
 * commentLimits caps the list of changes for the preview comment. The line
 * that counts what did not fit links reportUrl.
 */
export function commentLimits(reportUrl: string): Limits {
  return {
    diffLines: 40,
    diffLength: 4_000,
    length: COMMENT_LENGTH,
    more: `the [full report](${reportUrl}) lists them.`,
  };
}

/** REPORT_LIMITS shows each diff in full, within the size of a job summary. */
export const REPORT_LIMITS: Limits = {
  diffLines: Number.POSITIVE_INFINITY,
  diffLength: Number.POSITIVE_INFINITY,
  length: REPORT_LENGTH,
  more: "the job summary has no room for them.",
};

function main(args: string[]): void {
  const [command, mainDir, headDir, mainUrl, previewUrl, reportUrl] = args;
  if (command !== "comment" && command !== "report") {
    throw new Error(
      `${JSON.stringify(command)} is not a command: use comment or report`,
    );
  }
  const head = readSnapshot(headDir ?? "");
  const base = readSnapshot(mainDir ?? "");
  let text: string;
  if (!head?.complete) text = missingSnapshot("pull request");
  else if (!base?.complete) text = missingSnapshot("main");
  else {
    text = renderChanges(
      compareSnapshots(base, head),
      { main: mainUrl ?? "", preview: previewUrl ?? "" },
      command === "comment" ? commentLimits(reportUrl ?? "") : REPORT_LIMITS,
    );
  }
  process.stdout.write(text);
}

if (import.meta.main) {
  try {
    main(process.argv.slice(2));
  } catch (error) {
    console.error((error as Error).message);
    process.exit(1);
  }
}
