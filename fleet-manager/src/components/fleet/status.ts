export type ConditionStatus = "True" | "False" | "Unknown";

/** The fields of a Kubernetes condition that the status components read. */
export type ConditionLike = {
  status: ConditionStatus;
  reason?: string;
  message?: string;
};

export type StatusTone = "ok" | "warn" | "bad" | "off" | "muted";

export const CONDITION_TONE: Record<ConditionStatus, StatusTone> = {
  True: "ok",
  False: "bad",
  Unknown: "warn",
};

/** HEALTHY_REASON is ReasonHealthy of api/v1, the reason of a healthy Ready. */
export const HEALTHY_REASON = "Healthy";

/**
 * describeReady turns a Ready condition into the words and the tone that the
 * status components show. A missing condition is "No status". The reason is
 * kept for False and Unknown, and for True with trueReason unless it is
 * HEALTHY_REASON, such as Suspended.
 */
export function describeReady(
  condition: ConditionLike | undefined,
  { trueReason = false }: { trueReason?: boolean } = {},
): {
  tone: StatusTone;
  label: string;
  reason?: string;
} {
  switch (condition?.status) {
    case "True":
      return trueReason &&
        condition.reason &&
        condition.reason !== HEALTHY_REASON
        ? {
            tone: CONDITION_TONE.True,
            label: "Ready",
            reason: condition.reason,
          }
        : { tone: CONDITION_TONE.True, label: "Ready" };
    case "False":
      return {
        tone: CONDITION_TONE.False,
        label: "Not ready",
        reason: condition.reason,
      };
    case "Unknown":
      return {
        tone: CONDITION_TONE.Unknown,
        label: "Unknown",
        reason: condition.reason,
      };
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

/** Text on the panel background. */
export const TEXT_TONE: Record<StatusTone, string> = {
  ok: "text-ok",
  warn: "text-warn",
  bad: "text-bad",
  off: "text-muted",
  muted: "text-muted",
};

export const DOT_TONE: Record<StatusTone, string> = {
  ok: "bg-ok shadow-[0_0_6px_var(--ok)]",
  warn: "bg-warn shadow-[0_0_6px_var(--warn)]",
  bad: "bg-bad shadow-[0_0_6px_var(--bad)]",
  off: "bg-off ring-1 ring-faint",
  muted: "bg-faint",
};
