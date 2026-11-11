import type { Resource } from "@/lib/k8s/kinds";
import type { PodUsage } from "@/lib/observe/types";
import type { Warning, WarningSeverity } from "./types";

/** LIMIT_WARNING is the share of a limit at which a pod is near it. */
export const LIMIT_WARNING = 0.75;
/** LIMIT_ERROR is the share of a limit at which near the limit is an error. */
export const LIMIT_ERROR = 0.9;
/** RESTARTS_ABOVE is the restart count above which a recent restart warns. */
export const RESTARTS_ABOVE = 3;
/** RECENT_MS is how long ago a termination counts as recent. */
export const RECENT_MS = 3_600_000;
/** BACKPRESSURE_ABOVE is the share of dropped requests above which backpressure warns. */
export const BACKPRESSURE_ABOVE = 0.01;

const OOM_KILLED = "OOMKilled";

/**
 * containerWarnings returns the out of memory and restarts warnings of the
 * containers of pod whose last termination finished less than RECENT_MS
 * before now. It reads status.containerStatuses only, because the app has
 * no kube-state-metrics to count the restarts of the last hour.
 */
export function containerWarnings(
  pod: Resource<"Pod">,
  now: string,
): Warning[] {
  const name = pod.metadata?.name ?? "";
  return (pod.status?.containerStatuses ?? []).flatMap((status) => {
    const terminated = status.lastState?.terminated;
    const finishedAt = terminated?.finishedAt;
    if (!finishedAt || Date.parse(now) - Date.parse(finishedAt) >= RECENT_MS) {
      return [];
    }
    const container = `Container ${status.name} of pod ${name}`;
    const warnings: Warning[] = [];
    if (terminated.reason === OOM_KILLED) {
      warnings.push({
        kind: "oom",
        severity: "Error",
        label: "Out of memory",
        detail: `${container} ran out of memory`,
        since: finishedAt,
      });
    }
    if (status.restartCount > RESTARTS_ABOVE) {
      warnings.push({
        kind: "restarts",
        severity: "Warning",
        label: "Restarts",
        detail: `${container} restarted ${status.restartCount} times`,
        since: finishedAt,
      });
    }
    return warnings;
  });
}

/**
 * limitWarnings returns a warning for the memory and for the CPU of usage
 * when its use is at least LIMIT_WARNING of its limit, and an error from
 * LIMIT_ERROR. A resource without a limit gives none. The detail gives the
 * share as a whole percent, rounded down.
 */
export function limitWarnings(usage: PodUsage): Warning[] {
  const memory = nearLimit(usage.memory.usedBytes, usage.memory.limitBytes);
  const cpu = nearLimit(usage.cpu.usedCores, usage.cpu.limitCores);
  return [
    ...(memory
      ? [
          {
            kind: "memory" as const,
            severity: memory.severity,
            label: "Near memory limit",
            detail: `Pod ${usage.pod} uses ${memory.percent} % of its memory limit`,
          },
        ]
      : []),
    ...(cpu
      ? [
          {
            kind: "cpu" as const,
            severity: cpu.severity,
            label: "Near CPU limit",
            detail: `Pod ${usage.pod} uses ${cpu.percent} % of its CPU limit`,
          },
        ]
      : []),
  ];
}

/**
 * backpressureWarning returns a warning when share, the part of the
 * requests that backpressure dropped over 5 minutes, is above
 * BACKPRESSURE_ABOVE. A share that is not a finite number gives none.
 */
export function backpressureWarning(share: number): Warning | undefined {
  if (!Number.isFinite(share) || share <= BACKPRESSURE_ABOVE) return undefined;
  return {
    kind: "backpressure",
    severity: "Warning",
    label: "Backpressure",
    detail: `${percentOf(share)} % of the requests were dropped by backpressure in the last 5 minutes`,
  };
}

function nearLimit(
  used: number,
  limit: number | undefined,
): { severity: WarningSeverity; percent: number } | undefined {
  if (!limit) return undefined;
  const share = used / limit;
  if (share < LIMIT_WARNING) return undefined;
  return {
    severity: share >= LIMIT_ERROR ? "Error" : "Warning",
    percent: percentOf(share),
  };
}

function percentOf(share: number): number {
  return Math.floor(share * 100);
}
