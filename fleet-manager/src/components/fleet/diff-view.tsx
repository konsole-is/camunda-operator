import type { DiffLine } from "@/lib/diff";
import { cn } from "@/lib/utils";

/** Unchanged lines that stay visible on each side of a change. */
const CONTEXT = 3;

/**
 * DiffView shows a line diff of an object. Added lines are insertions and
 * removed lines are deletions. A run of more than 2 × CONTEXT + 1 unchanged
 * lines folds into a disclosure that says how many lines it holds. A diff
 * without a change says "No changes".
 */
export function DiffView({
  lines,
  label,
}: {
  lines: DiffLine[];
  label: string;
}) {
  if (!lines.some((line) => line.op !== "same")) {
    return <p className="text-sm text-muted">No changes</p>;
  }
  return (
    <figure
      aria-label={label}
      className="min-w-0 overflow-x-auto rounded-md border border-border bg-panel"
    >
      <div className="min-w-max py-2 font-mono text-xs leading-5">
        {chunks(lines).map((chunk, index) =>
          chunk.folded ? (
            <details key={index} className="text-muted">
              <summary className="cursor-pointer px-3 hover:text-text">
                {`${chunk.lines.length} unchanged lines`}
              </summary>
              {chunk.lines.map((line, i) => (
                <Line key={i} line={line} />
              ))}
            </details>
          ) : (
            chunk.lines.map((line, i) => (
              <Line key={`${index}-${i}`} line={line} />
            ))
          ),
        )}
      </div>
    </figure>
  );
}

function Line({ line }: { line: DiffLine }) {
  const sign = line.op === "added" ? "+" : line.op === "removed" ? "-" : " ";
  const text = <span className="whitespace-pre">{line.text}</span>;
  return (
    <div
      className={cn(
        "flex gap-3 px-3",
        line.op === "added" && "bg-ok/10 text-ok",
        line.op === "removed" && "bg-bad/10 text-bad",
        line.op === "same" && "text-text-2",
      )}
    >
      <span aria-hidden className="w-3 shrink-0 select-none">
        {sign}
      </span>
      {line.op === "added" ? (
        <ins className="whitespace-pre no-underline">{line.text}</ins>
      ) : line.op === "removed" ? (
        <del className="whitespace-pre no-underline">{line.text}</del>
      ) : (
        text
      )}
    </div>
  );
}

function chunks(lines: DiffLine[]): { folded: boolean; lines: DiffLine[] }[] {
  const result: { folded: boolean; lines: DiffLine[] }[] = [];
  let start = 0;
  while (start < lines.length) {
    let end = start;
    const same = lines[start].op === "same";
    while (end < lines.length && (lines[end].op === "same") === same) end++;
    const run = lines.slice(start, end);
    const head = start === 0 ? 0 : CONTEXT;
    const tail = end === lines.length ? 0 : CONTEXT;
    if (same && run.length > head + tail + 1) {
      if (head) result.push({ folded: false, lines: run.slice(0, head) });
      result.push({ folded: true, lines: run.slice(head, run.length - tail) });
      if (tail)
        result.push({ folded: false, lines: run.slice(run.length - tail) });
    } else {
      result.push({ folded: false, lines: run });
    }
    start = end;
  }
  return result;
}
