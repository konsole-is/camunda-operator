/** The fields of a Kubernetes condition that the status components read. */
export type ConditionStatus = "True" | "False" | "Unknown";
export type ConditionLike = {
  status: ConditionStatus;
  reason?: string;
  message?: string;
};

export type StatusTone = "ok" | "warn" | "bad" | "off" | "muted";

/**
 * describeReady turns a Ready condition into the words and the tone that the
 * status components show. A missing condition is "No status". The reason is
 * kept for False and Unknown only, because True always says Healthy.
 */
export function describeReady(condition: ConditionLike | undefined): {
  tone: StatusTone;
  label: string;
  reason?: string;
} {
  switch (condition?.status) {
    case "True":
      return { tone: "ok", label: "Ready" };
    case "False":
      return { tone: "bad", label: "Not ready", reason: condition.reason };
    case "Unknown":
      return { tone: "warn", label: "Unknown", reason: condition.reason };
    default:
      return { tone: "off", label: "No status" };
  }
}

/** Text on a tinted surface. --off is under the contrast floor as text. */
export const CHIP_TONE: Record<StatusTone, string> = {
  ok: "bg-ok/15 text-ok",
  warn: "bg-warn/15 text-warn",
  bad: "bg-bad/15 text-bad",
  off: "bg-panel-2 text-muted",
  muted: "bg-panel-2 text-muted",
};

export const DOT_TONE: Record<StatusTone, string> = {
  ok: "bg-ok shadow-[0_0_6px_var(--ok)]",
  warn: "bg-warn shadow-[0_0_6px_var(--warn)]",
  bad: "bg-bad shadow-[0_0_6px_var(--bad)]",
  off: "bg-off",
  muted: "bg-faint",
};
