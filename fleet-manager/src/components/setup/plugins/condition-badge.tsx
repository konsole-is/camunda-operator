import type { ReactNode } from "react";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import {
  CHIP_TONE,
  CONDITION_TONE,
  DOT_TONE,
  HEALTHY_REASON,
  type StatusTone,
} from "@/components/fleet/status";
import { cn } from "@/lib/utils";

/**
 * ConditionBadge shows one condition of a row by its type: "<type>: <status>",
 * with the reason after a dot when the status is not True or the reason is
 * not Healthy. Without a condition it shows "<type>: No status", and with
 * unknown "<type>: Not known". The message is the hover text.
 */
export function ConditionBadge({
  type,
  condition,
  unknown = false,
}: {
  type: string;
  condition?: ConditionRow;
  unknown?: boolean;
}): ReactNode {
  if (unknown) return <Badge tone="off" text={`${type}: Not known`} />;
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
    <Badge tone={tone} text={text} title={condition?.message || undefined} />
  );
}

function Badge({
  tone,
  text,
  title,
}: {
  tone: StatusTone;
  text: string;
  title?: string;
}) {
  return (
    <span
      title={title}
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
