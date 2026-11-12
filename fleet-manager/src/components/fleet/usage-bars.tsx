import type { PodUsage } from "@/lib/observe/types";
import { cn } from "@/lib/utils";
import { formatBytes, formatCores } from "./units";

/**
 * UsageBars shows the CPU and memory of one pod as two meters. A meter
 * fills against the limit, or the request when there is no limit. Without
 * either it shows the use only. With both, the meter also names the
 * request. Use at 90 % or more of a limit is a warning. Use
 * against a request never is.
 */
export function UsageBars({ usage }: { usage: PodUsage }) {
  return (
    <div className="flex min-w-40 flex-col gap-1.5">
      <Bar
        label="CPU"
        used={usage.cpu.usedCores}
        max={usage.cpu.limitCores ?? usage.cpu.requestCores}
        limit={usage.cpu.limitCores}
        request={usage.cpu.requestCores}
        format={formatCores}
        suffix=" cores"
      />
      <Bar
        label="Memory"
        used={usage.memory.usedBytes}
        max={usage.memory.limitBytes ?? usage.memory.requestBytes}
        limit={usage.memory.limitBytes}
        request={usage.memory.requestBytes}
        format={formatBytes}
      />
    </div>
  );
}

function Bar({
  label,
  used,
  max,
  limit,
  request,
  format,
  suffix = "",
}: {
  label: string;
  used: number;
  max?: number;
  limit?: number;
  request?: number;
  format: (n: number) => string;
  suffix?: string;
}) {
  const share = max ? Math.min(1, used / max) : undefined;
  const warn = limit ? used / limit >= 0.9 : false;
  const text = max
    ? `${format(used)} of ${format(max)}${suffix}`
    : `${format(used)}${suffix}`;
  const requested =
    limit !== undefined && request !== undefined
      ? `request ${format(request)}${suffix}`
      : undefined;
  return (
    <div className="flex items-center gap-2 text-xs">
      <span className="w-14 shrink-0 text-muted">{label}</span>
      <div
        role="meter"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={max || used || 1}
        aria-valuenow={max ? Math.min(used, max) : used}
        aria-valuetext={requested ? `${text}, ${requested}` : text}
        className="relative h-1.5 flex-1 overflow-hidden rounded-full bg-panel-2"
      >
        {share !== undefined && (
          <div
            className={cn("h-full rounded-full", warn ? "bg-warn" : "bg-info")}
            style={{ width: `${share * 100}%` }}
          />
        )}
        {requested && limit && request! < limit && (
          <div
            data-slot="request-mark"
            className="absolute inset-y-0 w-0.5 bg-text"
            style={{ left: `${(request! / limit) * 100}%` }}
          />
        )}
      </div>
      <span className="flex w-28 shrink-0 flex-col text-right text-text-2">
        <span>{text}</span>
        {requested && <span className="text-muted">{requested}</span>}
      </span>
    </div>
  );
}
