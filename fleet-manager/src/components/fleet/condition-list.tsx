import { age } from "./age";
import type { ConditionRow } from "./conditions-table";
import { StatusDot } from "./status-dot";

/**
 * ConditionList lists conditions in the order given, for a narrow column:
 * the type, a dot of the state with the reason, and under them the message
 * and the time since the last change, as far as the condition has them.
 * label names the list. now is the time that the ages count to.
 */
export function ConditionList({
  conditions,
  label,
  now,
}: {
  conditions: ConditionRow[];
  label: string;
  now: string;
}) {
  if (conditions.length === 0) {
    return <p className="text-sm text-muted">No conditions</p>;
  }
  return (
    <ul aria-label={label} className="flex flex-col divide-y divide-border">
      {conditions.map((c) => (
        <li
          key={c.type}
          className="flex flex-col gap-0.5 py-2.5 first:pt-0 last:pb-0"
        >
          <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-0.5">
            <span className="font-mono text-xs text-text">{c.type}</span>
            <span className="flex items-center gap-1.5 text-xs text-text-2">
              <StatusDot label={c.type} condition={c} />
              {c.reason}
            </span>
          </div>
          {c.message && (
            <p className="text-xs break-words text-muted">{c.message}</p>
          )}
          {c.lastTransitionTime && (
            <p className="text-xs text-muted">
              Changed {age(c.lastTransitionTime, now)} ago
            </p>
          )}
        </li>
      ))}
    </ul>
  );
}
