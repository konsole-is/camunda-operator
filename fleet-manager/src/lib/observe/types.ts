/**
 * Unavailable is a value, not an error: the cluster has no metrics server,
 * or the app has no Prometheus. A page hides the column or the chart.
 */
export type Unavailable = "unavailable";

/** PodUsage is the live CPU and memory of one pod, with its requests and limits. */
export type PodUsage = {
  pod: string;
  cpu: { usedCores: number; requestCores?: number; limitCores?: number };
  memory: { usedBytes: number; requestBytes?: number; limitBytes?: number };
};

/** Series is one Prometheus range series. A point is a Unix time in milliseconds and a value. */
export type Series = { name: string; points: [time: number, value: number][] };

/**
 * LogLine is one log line. A JSON line gives time, level, message, logger,
 * request, and the rest of its fields. Another line has only raw and
 * message.
 */
export type LogLine = {
  raw: string;
  time?: string;
  level?: string;
  message: string;
  logger?: string;
  request?: string;
  fields: Record<string, unknown>;
};

/** TimeRange is the range of a Prometheus query. start and end are Unix times in milliseconds. */
export type TimeRange = { start: number; end: number; stepSeconds: number };

/** CoreComponent is a value of the label camunda.io/component on the pods of a cluster. */
export type CoreComponent =
  | "zeebe"
  | "gateway"
  | "operate"
  | "tasklist"
  | "admin"
  | "connectors";

/**
 * MetricDef is one chart: a rendered PromQL query. source names where the
 * query comes from: the documentation of its metric, or a registration.
 */
export type MetricDef = {
  label: string;
  query: string;
  unit?: string;
  source: string;
};

/** QueryError is a metric query of a registration that does not render, with the reason. */
export type QueryError = { label: string; source: string; error: string };

/**
 * CoreQuery is a fixed query of a core component. query is a template with
 * {{ .namespace }} and {{ .pods }}. metric is the documented metric that it
 * reads, and source is the address of that documentation.
 */
export type CoreQuery = {
  label: string;
  query: string;
  unit?: string;
  metric: string;
  source: string;
};

/**
 * PartitionHealth is one partition of a cluster. leader and followers are
 * pod names. A partition that no answering broker leads has no leader.
 * health is "unknown", because the management port of a broker does not
 * give it.
 */
export type PartitionHealth = {
  partition: number;
  leader?: string;
  followers: string[];
  processorPhase?: string;
  health: "unknown";
};
