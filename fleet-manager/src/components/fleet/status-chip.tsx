import { cn } from "@/lib/utils";
import {
  CHIP_TONE,
  type ConditionLike,
  DOT_TONE,
  describeReady,
} from "./status";

/**
 * StatusChip shows the Ready condition of an object: "Ready", or "Not ready"
 * and "Unknown" with the reason. The message is the hover text. loading wins
 * over condition while the page has no status yet.
 */
export function StatusChip({
  condition,
  loading = false,
}: {
  condition?: ConditionLike;
  loading?: boolean;
}) {
  const ready = loading
    ? { tone: "muted" as const, label: "Loading", reason: undefined }
    : describeReady(condition);
  return (
    <span
      aria-busy={loading || undefined}
      title={loading ? undefined : condition?.message}
      className={cn(
        "inline-flex max-w-full items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium",
        CHIP_TONE[ready.tone],
      )}
    >
      <span
        aria-hidden
        className={cn("size-1.5 shrink-0 rounded-full", DOT_TONE[ready.tone])}
      />
      <span className="truncate">
        {ready.reason ? `${ready.label} · ${ready.reason}` : ready.label}
      </span>
    </span>
  );
}
