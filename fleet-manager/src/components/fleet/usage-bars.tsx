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
      {(["cpu", "memory"] as const).map((resource) => {
        const r = reading(usage, resource);
        return (
          <div key={resource} className="flex items-center gap-2 text-xs">
            <span className="w-14 shrink-0 text-muted">{r.label}</span>
            <Meter reading={r} className="flex-1" />
            <span className="flex w-28 shrink-0 flex-col text-right text-text-2">
              <span>{r.text}</span>
              {r.requested && <span className="text-muted">{r.requested}</span>}
            </span>
          </div>
        );
      })}
    </div>
  );
}

/**
 * UsageMeter shows the CPU or the memory of one pod as its text over one
 * meter, by the rules of UsageBars.
 */
export function UsageMeter({
  usage,
  resource,
}: {
  usage: PodUsage;
  resource: "cpu" | "memory";
}) {
  const r = reading(usage, resource);
  return (
    <div className="flex min-w-28 flex-col gap-1 text-xs">
      <span className="whitespace-nowrap text-text-2">{r.text}</span>
      <Meter reading={r} />
      {r.requested && (
        <span className="whitespace-nowrap text-muted">{r.requested}</span>
      )}
    </div>
  );
}

type Reading = {
  label: string;
  used: number;
  max?: number;
  limit?: number;
  request?: number;
  text: string;
  requested?: string;
};

function reading(usage: PodUsage, resource: "cpu" | "memory"): Reading {
  const [label, used, limit, request, format, suffix] =
    resource === "cpu"
      ? ([
          "CPU",
          usage.cpu.usedCores,
          usage.cpu.limitCores,
          usage.cpu.requestCores,
          formatCores,
          " cores",
        ] as const)
      : ([
          "Memory",
          usage.memory.usedBytes,
          usage.memory.limitBytes,
          usage.memory.requestBytes,
          formatBytes,
          "",
        ] as const);
  const max = limit ?? request;
  return {
    label,
    used,
    max,
    limit,
    request,
    text: max
      ? `${format(used)} of ${format(max)}${suffix}`
      : `${format(used)}${suffix}`,
    requested:
      limit !== undefined && request !== undefined
        ? `request ${format(request)}${suffix}`
        : undefined,
  };
}

function Meter({
  reading: r,
  className,
}: {
  reading: Reading;
  className?: string;
}) {
  const share = r.max ? Math.min(1, r.used / r.max) : undefined;
  const warn = r.limit ? r.used / r.limit >= 0.9 : false;
  return (
    <div
      role="meter"
      aria-label={r.label}
      aria-valuemin={0}
      aria-valuemax={r.max || r.used || 1}
      aria-valuenow={r.max ? Math.min(r.used, r.max) : r.used}
      aria-valuetext={r.requested ? `${r.text}, ${r.requested}` : r.text}
      className={cn(
        "relative h-1.5 overflow-hidden rounded-full bg-panel-2",
        className,
      )}
    >
      {share !== undefined && (
        <div
          className={cn("h-full rounded-full", warn ? "bg-warn" : "bg-info")}
          style={{ width: `${share * 100}%` }}
        />
      )}
      {r.requested && r.limit && r.request! < r.limit && (
        <div
          data-slot="request-mark"
          className="absolute inset-y-0 w-0.5 bg-text"
          style={{ left: `${(r.request! / r.limit) * 100}%` }}
        />
      )}
    </div>
  );
}
