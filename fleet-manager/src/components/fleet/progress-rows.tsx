import type { ReactNode } from "react";
import { type ConditionLike } from "./status";
import { StatusChip } from "./status-chip";
import { StatusDot } from "./status-dot";

/** ProgressRow is one row of the progress after a create: a request or the cluster. */
export type ProgressRow = {
  id: string;
  label: string;
  detail?: string;
  summary?: string;
  checks: { label: string; condition?: ConditionLike }[];
};

/**
 * ProgressRows shows the progress after a create: on top the Ready
 * condition of the claim with its message, then one row for each request
 * and component, with the state of each of its checks and its summary
 * headline. done, when given, shows under the rows, for example a link to
 * the new cluster.
 */
export function ProgressRows({
  ready,
  rows,
  done,
}: {
  ready?: ConditionLike;
  rows: ProgressRow[];
  done?: ReactNode;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-4">
      {ready && (
        <div className="flex flex-col gap-1">
          <StatusChip condition={ready} />
          {ready.message && (
            <p className="text-sm text-text-2">{ready.message}</p>
          )}
        </div>
      )}
      <ol className="flex flex-col divide-y divide-divider rounded-md border border-border">
        {rows.map((row) => (
          <li
            key={row.id}
            className="flex flex-col gap-1 px-3 py-2 sm:flex-row sm:items-center sm:gap-4"
          >
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium text-text">{row.label}</p>
              {row.detail && <p className="text-xs text-muted">{row.detail}</p>}
            </div>
            <div className="flex flex-wrap items-center gap-3">
              {row.checks.map((check) => (
                <span
                  key={check.label}
                  className="inline-flex items-center gap-1.5 text-xs text-text-2"
                >
                  <StatusDot label={check.label} condition={check.condition} />
                  <span aria-hidden>{check.label}</span>
                </span>
              ))}
            </div>
            {row.summary && (
              <p className="text-xs text-muted sm:w-56">{row.summary}</p>
            )}
          </li>
        ))}
      </ol>
      {done}
    </div>
  );
}
