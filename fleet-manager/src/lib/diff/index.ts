import { stringify } from "yaml";

/** DiffLine is one line of a line diff: unchanged, added, or removed. */
export type DiffLine = { op: "same" | "added" | "removed"; text: string };

// Above this many cells, the compare table takes more memory than a
// preview may use. The changed part then shows as removed, then added.
const MAX_CELLS = 4_000_000;

/**
 * lineDiff compares two texts line by line. Where a line changes, the
 * removed line comes before the added one. A final newline adds no line.
 */
export function lineDiff(before: string, after: string): DiffLine[] {
  const a = splitLines(before);
  const b = splitLines(after);
  let start = 0;
  while (start < a.length && start < b.length && a[start] === b[start]) start++;
  let end = 0;
  while (
    end < a.length - start &&
    end < b.length - start &&
    a[a.length - 1 - end] === b[b.length - 1 - end]
  )
    end++;
  const same = (lines: string[]): DiffLine[] =>
    lines.map((text) => ({ op: "same", text }));
  return [
    ...same(a.slice(0, start)),
    ...changed(a.slice(start, a.length - end), b.slice(start, b.length - end)),
    ...same(a.slice(a.length - end)),
  ];
}

function changed(a: string[], b: string[]): DiffLine[] {
  if ((a.length + 1) * (b.length + 1) > MAX_CELLS) {
    return [
      ...a.map((text): DiffLine => ({ op: "removed", text })),
      ...b.map((text): DiffLine => ({ op: "added", text })),
    ];
  }
  const common = Array.from(
    { length: a.length + 1 },
    () => new Uint32Array(b.length + 1),
  );
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      common[i][j] =
        a[i] === b[j]
          ? common[i + 1][j + 1] + 1
          : Math.max(common[i + 1][j], common[i][j + 1]);
    }
  }
  const lines: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) {
      lines.push({ op: "same", text: a[i] });
      i++;
      j++;
    } else if (common[i + 1][j] >= common[i][j + 1]) {
      lines.push({ op: "removed", text: a[i++] });
    } else {
      lines.push({ op: "added", text: b[j++] });
    }
  }
  while (i < a.length) lines.push({ op: "removed", text: a[i++] });
  while (j < b.length) lines.push({ op: "added", text: b[j++] });
  return lines;
}

/**
 * objectDiff renders both objects as YAML with sorted keys and compares the
 * lines, so a different key order is no change. A before of undefined is a
 * new object: every line is added.
 */
export function objectDiff(before: unknown, after: unknown): DiffLine[] {
  return lineDiff(before === undefined ? "" : toYaml(before), toYaml(after));
}

function toYaml(value: unknown): string {
  return stringify(value, { sortMapEntries: true });
}

function splitLines(text: string): string[] {
  return text === "" ? [] : text.replace(/\n$/, "").split("\n");
}
