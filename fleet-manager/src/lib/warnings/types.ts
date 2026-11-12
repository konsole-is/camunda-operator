export type WarningSeverity = "Warning" | "Error";

export type WarningKind =
  | "oom"
  | "restarts"
  | "memory"
  | "cpu"
  | "backpressure"
  | "backup"
  | "summary"
  | "request";

/**
 * Warning is one chip of the Warnings column. label is the chip text and
 * detail the tooltip. since is the time of the fact, which the tooltip
 * shows as an age.
 */
export type Warning = {
  kind: WarningKind;
  severity: WarningSeverity;
  label: string;
  detail: string;
  since?: string;
};

/** WarningSource is a source that the pod and metric warnings need. */
export type WarningSource = "metrics-server" | "prometheus";

/**
 * PodWarnings are the pod and metric warnings of each cluster by
 * "<namespace>/<name>", and the sources that did not answer. A cluster
 * without such a warning can be missing from clusters.
 */
export type PodWarnings = {
  clusters: Record<string, Warning[]>;
  missing: WarningSource[];
};
