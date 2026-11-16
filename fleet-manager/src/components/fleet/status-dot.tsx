import { cn } from "@/lib/utils";
import { type ConditionLike, DOT_TONE, describeReady } from "./status";

/**
 * StatusDot is the dot of one component in a row, for example Zeebe in the
 * fleet table. Its accessible name says the component and its Ready state.
 * off marks a component that the cluster does not use. unknown marks a
 * component whose condition the page could not read: the dot says "Not
 * known" and ignores condition.
 */
export function StatusDot({
  label,
  condition,
  off = false,
  unknown = false,
}: {
  label: string;
  condition?: ConditionLike;
  off?: boolean;
  unknown?: boolean;
}) {
  const ready = describeReady(condition);
  const state = off
    ? "Not used"
    : unknown
      ? "Not known"
      : ready.reason
        ? `${ready.label} (${ready.reason})`
        : ready.label;
  return (
    <span
      role="img"
      aria-label={`${label}: ${state}`}
      title={off || unknown ? undefined : condition?.message}
      className={cn(
        "inline-block size-2 shrink-0 rounded-full",
        DOT_TONE[off ? "off" : unknown ? "muted" : ready.tone],
      )}
    />
  );
}
