import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import type { PresetImpact, RollMark } from "./impact";

const WARN: ReadonlySet<RollMark> = new Set([
  "Pods roll",
  "Connectors restart",
  "Pods may roll",
]);

/**
 * ImpactPreview shows each resource that inherits the preset with its roll
 * mark, then the notes of the change. While a list that the marks need is
 * not read, it names those lists and shows no resource.
 */
export function ImpactPreview({ impact }: { impact: PresetImpact }): ReactNode {
  return (
    <div className="flex min-w-0 flex-col gap-3">
      {impact.unread.length > 0 ? (
        <p role="alert" className="text-sm text-bad">
          The impact is not known while the fleet manager cannot read{" "}
          {impact.unread.join(", ")}. Save is possible when it can read them.
        </p>
      ) : impact.rows.length === 0 ? (
        <p className="text-sm text-muted">No resource inherits this preset.</p>
      ) : (
        <ul aria-label="Inheriting resources" className="flex flex-col gap-1.5">
          {impact.rows.map((row) => (
            <li
              key={`${row.kind}/${row.namespace}/${row.name}`}
              className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm"
            >
              <span className="font-mono text-xs break-all text-text">
                {row.namespace}/{row.name}
              </span>
              <span
                className={cn(
                  "rounded-full border px-2 py-0.5 text-xs font-medium",
                  WARN.has(row.mark)
                    ? "border-warn/40 bg-warn/10 text-text"
                    : "border-border bg-panel-2 text-muted",
                )}
              >
                {row.mark}
              </span>
            </li>
          ))}
        </ul>
      )}
      {impact.notes.length > 0 && (
        <ul
          aria-label="Notes"
          className="flex flex-col gap-1 border-l-2 border-warn/50 pl-3 text-sm text-text-2"
        >
          {impact.notes.map((note) => (
            <li key={note}>{note}</li>
          ))}
        </ul>
      )}
    </div>
  );
}
