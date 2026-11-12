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
 * message. pod is set by the stream hooks: the pod whose log has the line.
 */
export type LogLine = {
  raw: string;
  pod?: string;
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
  "zeebe" | "gateway" | "operate" | "tasklist" | "admin" | "connectors";

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
 * VersionRange is a range of Camunda versions MAJOR.MINOR.PATCH: min is in
 * it, below is not. Without below, the range has no upper bound.
 */
export type VersionRange = { min: string; below?: string };

/** MetricsComponent is a core component, or "optimize" for the pods of a CamundaOptimize. */
export type MetricsComponent = CoreComponent | "optimize";

/**
 * CoreQuery is a fixed query of a core component. query is a template with
 * {{ .namespace }} and {{ .pods }}. metric is the documented metric that it
 * reads, and source is the address of that documentation. versions is the
 * range of Camunda versions that the source covers. A query without
 * versions applies to every version. With over "volumes", {{ .pods }} holds
 * the PersistentVolumeClaim names of the pods, not the pod names.
 */
export type CoreQuery = {
  label: string;
  query: string;
  unit?: string;
  metric: string;
  source: string;
  versions?: VersionRange;
  over?: "volumes";
};

/** MetricsRange is a time range of the charts, back from now. */
export type MetricsRange = "1h" | "6h" | "24h";

/**
 * MetricsRef names the pods and the queries of a set of charts: a
 * component of a cluster, or a PluginRequest. The namespace is a separate
 * parameter.
 */
export type MetricsRef =
  { cluster: string; component: MetricsComponent } | { request: string };

/**
 * MetricChartData is one chart that has data. It has at least one series.
 * declaredBy is the label of the registration that declared the query; a
 * fixed query has none.
 */
export type MetricChartData = {
  label: string;
  unit?: string;
  source: string;
  declaredBy?: string;
  series: Series[];
};

/** ChartError is a query of a registration that does not render, with the reason. */
export type ChartError = { label: string; declaredBy: string; error: string };

/**
 * MetricsAnswer is the answer of the charts of a MetricsRef. "notConfigured"
 * means that the app has no Prometheus URL, and "unavailable" that
 * Prometheus answered no query. An answer leaves out the chart of a query
 * without a finite point. hiddenForVersion is set when a Camunda query of
 * the component does not apply to the Camunda version of the cluster, or
 * the cluster reports none. Its version is set only when the cluster
 * reports one.
 */
export type MetricsAnswer =
  | { prometheus: "notConfigured" }
  | { prometheus: "unavailable" }
  | {
      prometheus: "answered";
      charts: MetricChartData[];
      errors: ChartError[];
      hiddenForVersion?: { version?: string };
    };

/**
 * PartitionHealthState is the zeebe_health value of a partition. It is
 * "unknown" when Prometheus gives no value for the partition.
 */
export type PartitionHealthState = "healthy" | "unhealthy" | "dead" | "unknown";

/**
 * PartitionHealth is one partition of a cluster. leader and followers are
 * pod names. A partition that no answering broker leads has no leader.
 */
export type PartitionHealth = {
  partition: number;
  leader?: string;
  followers: string[];
  processorPhase?: string;
  health: PartitionHealthState;
};
