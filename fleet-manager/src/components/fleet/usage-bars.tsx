import type { PodUsage } from "@/lib/observe/types";
import { cn } from "@/lib/utils";
import { formatBytes, formatCores } from "./units";

/**
 * UsageBars shows the CPU and memory of one pod as two meters. A meter
 * fills against the limit, or the request when there is no limit. Without
 * either it shows the use only. Use at 90 % or more of the limit is a
 * warning.
 */
export function UsageBars({ usage }: { usage: PodUsage }) {
  return (
    <div className="flex min-w-40 flex-col gap-1.5">
      <Bar
        label="CPU"
        used={usage.cpu.usedCores}
        max={usage.cpu.limitCores ?? usage.cpu.requestCores}
        format={formatCores}
        suffix=" cores"
      />
      <Bar
        label="Memory"
        used={usage.memory.usedBytes}
        max={usage.memory.limitBytes ?? usage.memory.requestBytes}
        format={formatBytes}
      />
    </div>
  );
}

function Bar({
  label,
  used,
  max,
  format,
  suffix = "",
}: {
  label: string;
  used: number;
  max?: number;
  format: (n: number) => string;
  suffix?: string;
}) {
  const share = max ? Math.min(1, used / max) : undefined;
  const text = max
    ? `${format(used)} of ${format(max)}${suffix}`
    : `${format(used)}${suffix}`;
  return (
    <div className="flex items-center gap-2 text-xs">
      <span className="w-14 shrink-0 text-muted">{label}</span>
      <div
        role="meter"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={max ?? used}
        aria-valuenow={max ? Math.min(used, max) : used}
        aria-valuetext={text}
        className="h-1.5 flex-1 overflow-hidden rounded-full bg-panel-2"
      >
        {share !== undefined && (
          <div
            className={cn(
              "h-full rounded-full",
              share >= 0.9 ? "bg-warn" : "bg-info",
            )}
            style={{ width: `${share * 100}%` }}
          />
        )}
      </div>
      <span className="w-28 shrink-0 text-right text-text-2">{text}</span>
    </div>
  );
}
