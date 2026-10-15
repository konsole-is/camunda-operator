import { stringify } from "yaml";

/** DiffLine is one line of a line diff: unchanged, added, or removed. */
export type DiffLine = { op: "same" | "added" | "removed"; text: string };

/**
 * lineDiff compares two texts line by line with a longest common
 * subsequence. Where a line changes, the removed line comes before the
 * added one. A final newline adds no line.
 */
export function lineDiff(before: string, after: string): DiffLine[] {
  const a = splitLines(before);
  const b = splitLines(after);
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
