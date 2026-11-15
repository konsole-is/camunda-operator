import type { ReactNode } from "react";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import {
  CHIP_TONE,
  CONDITION_TONE,
  DOT_TONE,
  HEALTHY_REASON,
} from "@/components/fleet/status";
import { cn } from "@/lib/utils";

/**
 * ConditionBadge shows one condition of a row by its type: "<type>: <status>",
 * with the reason after a dot when the status is not True or the reason is
 * not Healthy. Without a condition it shows "<type>: No status". The message
 * is the hover text.
 */
export function ConditionBadge({
  type,
  condition,
}: {
  type: string;
  condition?: ConditionRow;
}): ReactNode {
  const tone = condition ? CONDITION_TONE[condition.status] : "off";
  const reason =
    condition?.reason &&
    (condition.status !== "True" || condition.reason !== HEALTHY_REASON)
      ? condition.reason
      : undefined;
  const text = condition
    ? `${type}: ${condition.status}${reason ? ` · ${reason}` : ""}`
    : `${type}: No status`;
  return (
    <span
      title={condition?.message || undefined}
      className={cn(
        "inline-flex max-w-full items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium",
        CHIP_TONE[tone],
      )}
    >
      <span
        aria-hidden
        className={cn("size-1.5 shrink-0 rounded-full", DOT_TONE[tone])}
      />
      <span className="truncate">{text}</span>
    </span>
  );
}
