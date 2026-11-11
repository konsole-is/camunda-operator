import "server-only";
import { type TemplateData, parseTemplate, render } from "@/lib/gotemplate";
import type { PluginsPluginRequestConditionType } from "@/generated/conditions";
import type { Resource } from "@/lib/k8s/kinds";
import type {
  CoreQuery,
  MetricDef,
  MetricsComponent,
  QueryError,
  VersionRange,
} from "./types";

const CADVISOR =
  "https://github.com/google/cadvisor/blob/master/docs/storage/prometheus.md";
const KUBELET = "https://kubernetes.io/docs/reference/instrumentation/metrics/";
const CAMUNDA =
  "https://docs.camunda.io/docs/8.9/self-managed/operational-guides/monitoring/metrics/#available-metrics";
const OUTPUTS_READY: PluginsPluginRequestConditionType = "OutputsReady";
const SELECT = 'namespace="{{ .namespace }}", pod=~"{{ .pods }}"';
const SELECT_VOLUMES =
  'namespace="{{ .namespace }}", persistentvolumeclaim=~"{{ .pods }}"';
const VERSION = /^(\d+)\.(\d+)\.(\d+)$/;

/**
 * CAMUNDA_8_9 is the range of the Camunda versions whose metrics page the
 * Camunda queries cite. Widen it for a new minor only after its metrics
 * page names the same metrics.
 */
export const CAMUNDA_8_9: VersionRange = { min: "8.9.0", below: "8.10.0" };

/**
 * POD_QUERIES are the queries of every component: CPU, memory, and the
 * disk used of the volumes of the pods. They read kubelet metrics, so they
 * apply to every Camunda version.
 */
export const POD_QUERIES: CoreQuery[] = [
  // The kubelet reports a series for each container and one for the pod
  // cgroup with an empty container label. container!="" drops the second,
  // so the sum by pod does not count each pod twice.
  {
    label: "CPU",
    query: `sum by (pod) (rate(container_cpu_usage_seconds_total{${SELECT}, container!=""}[5m]))`,
    unit: "cores",
    metric: "container_cpu_usage_seconds_total",
    source: CADVISOR,
  },
  {
    label: "Memory",
    query: `sum by (pod) (container_memory_working_set_bytes{${SELECT}, container!=""})`,
    unit: "bytes",
    metric: "container_memory_working_set_bytes",
    source: CADVISOR,
  },
  // Two scrapes of one volume, from two targets, must not add up.
  {
    label: "Disk used",
    query: `max by (persistentvolumeclaim) (kubelet_volume_stats_used_bytes{${SELECT_VOLUMES}})`,
    unit: "bytes",
    metric: "kubelet_volume_stats_used_bytes",
    source: KUBELET,
    over: "volumes",
  },
];

// The PROCESS elements count the child instances of a call activity too,
// so the label says activated, not started.
const ZEEBE: CoreQuery[] = [
  {
    label: "Process instances activated",
    query: `sum by (pod) (rate(zeebe_element_instance_events_total{${SELECT}, action="activated", type="PROCESS"}[5m]))`,
    unit: "instances/s",
    metric: "zeebe_element_instance_events_total",
    source: CAMUNDA,
    versions: CAMUNDA_8_9,
  },
  {
    label: "Records processed",
    query: `sum by (pod) (rate(zeebe_stream_processor_records_total{${SELECT}, action="processed"}[5m]))`,
    unit: "records/s",
    metric: "zeebe_stream_processor_records_total",
    source: CAMUNDA,
    versions: CAMUNDA_8_9,
  },
  {
    label: "Records exported",
    query: `sum by (pod) (rate(zeebe_exporter_events_total{${SELECT}, action="exported"}[5m]))`,
    unit: "events/s",
    metric: "zeebe_exporter_events_total",
    source: CAMUNDA,
    versions: CAMUNDA_8_9,
  },
  {
    label: "Requests dropped by backpressure",
    query: `sum by (pod) (rate(zeebe_dropped_request_count_total{${SELECT}}[5m]))`,
    unit: "requests/s",
    metric: "zeebe_dropped_request_count_total",
    source: CAMUNDA,
    versions: CAMUNDA_8_9,
  },
  {
    label: "Partition health",
    query: `min by (pod) (zeebe_health{${SELECT}})`,
    metric: "zeebe_health",
    source: CAMUNDA,
    versions: CAMUNDA_8_9,
  },
];

/**
 * PARTITION_HEALTH gives one series for each partition, named by its
 * partition label, with the worst zeebe_health value of the pods: 1
 * healthy, 0 unhealthy, -1 dead.
 */
export const PARTITION_HEALTH: CoreQuery = {
  label: "Partition health",
  query: `min by (partition) (zeebe_health{${SELECT}})`,
  metric: "zeebe_health",
  source: CAMUNDA,
  versions: CAMUNDA_8_9,
};

/**
 * coreQueries holds the fixed queries of each component, in chart order.
 * Each query reads one documented metric, which metric and source name,
 * and gives one series for each pod, or for each volume over volumes.
 * queriesFor picks the queries of a Camunda version.
 */
export const coreQueries: Record<MetricsComponent, CoreQuery[]> = {
  zeebe: [...POD_QUERIES, ...ZEEBE],
  gateway: POD_QUERIES,
  operate: POD_QUERIES,
  tasklist: POD_QUERIES,
  admin: POD_QUERIES,
  connectors: POD_QUERIES,
  optimize: POD_QUERIES,
};

/**
 * inVersionRange reports whether the Camunda version is in range. A
 * version that is not MAJOR.MINOR.PATCH, a pre-release included, is in no
 * range. Every version, and no version, is in an undefined range.
 */
export function inVersionRange(
  version: string | undefined,
  range: VersionRange | undefined,
): boolean {
  if (!range) return true;
  const v = parts(version);
  if (!v) return false;
  return (
    compare(v, parts(range.min)!) >= 0 &&
    (range.below === undefined || compare(v, parts(range.below)!) < 0)
  );
}

/** queriesFor returns the coreQueries of component that apply to the Camunda version. */
export function queriesFor(
  component: MetricsComponent,
  version: string | undefined,
): CoreQuery[] {
  return coreQueries[component].filter((q) =>
    inVersionRange(version, q.versions),
  );
}

/**
 * podsPattern returns the PromQL regular expression that matches exactly
 * the given pod names. Callers must not pass an empty list: the empty
 * pattern matches a series without a pod label.
 */
export function podsPattern(names: readonly string[]): string {
  return [...names]
    .sort()
    .map((name) => name.replaceAll(".", "[.]"))
    .join("|");
}

/**
 * coreMetricDefs renders queries for the pods of one component in
 * namespace, and a query over volumes for targets.volumes. A component
 * without pods has no queries, and a query over volumes is left out while
 * targets.volumes is empty.
 */
export function coreMetricDefs(
  queries: readonly CoreQuery[],
  namespace: string,
  targets: { pods: readonly string[]; volumes: readonly string[] },
): MetricDef[] {
  if (targets.pods.length === 0) return [];
  return queries.flatMap((q) => {
    const names = q.over === "volumes" ? targets.volumes : targets.pods;
    if (names.length === 0) return [];
    return [
      {
        label: q.label,
        query: renderCoreQuery(q, namespace, names),
        ...(q.unit && { unit: q.unit }),
        source: q.source,
      },
    ];
  });
}

/**
 * renderCoreQuery returns the PromQL of query in namespace. names are the
 * pod names, or the PersistentVolumeClaim names of a query over volumes.
 * Callers must not pass an empty names list, as for podsPattern.
 */
export function renderCoreQuery(
  query: CoreQuery,
  namespace: string,
  names: readonly string[],
): string {
  const data = templateData({ namespace, pods: podsPattern(names) });
  const rendered = render(query.query, data, "metric");
  if ("error" in rendered) {
    throw new Error(
      `The core query ${query.label} does not render: ${rendered.error}`,
    );
  }
  return rendered.text;
}

/**
 * registrationQueries renders the metricQueries of registration for one
 * request, with pods as the pods of the request. The source of each result
 * is the name of the registration. A query that does not render is a
 * QueryError with the reason. A query that uses {{ .pods }} is left out
 * while pods is empty, so it never matches the pods of others. The outputs
 * count only while OutputsReady is True for the current generation; before
 * that, a query that uses an output and is not left out is a QueryError.
 */
export function registrationQueries(
  registration: Resource<"PluginRegistration">,
  request: Resource<"PluginRequest">,
  pods: readonly string[],
): (MetricDef | QueryError)[] {
  const source = registration.metadata?.name ?? "";
  const parameters = request.spec.parameters;
  const data = templateData({
    namespace: request.metadata?.namespace ?? "",
    claim: request.spec.claimRef.name,
    request: request.metadata?.name ?? "",
    serviceAccount: request.spec.target.serviceAccountName ?? "",
    parameters:
      typeof parameters === "object" &&
      parameters !== null &&
      !Array.isArray(parameters)
        ? (parameters as Record<string, unknown>)
        : {},
    outputs: trustedOutputs(request),
    pods: pods.length ? podsPattern(pods) : undefined,
  });
  const results: (MetricDef | QueryError)[] = [];
  for (const { label, query, unit } of registration.spec.metricQueries ?? []) {
    const parsed = parseTemplate(query, "metric");
    if (!("error" in parsed) && parsed.usesPods && pods.length === 0) continue;
    const rendered = render(query, data, "metric");
    results.push(
      "error" in rendered
        ? { label, source, error: rendered.error }
        : { label, query: rendered.text, ...(unit && { unit }), source },
    );
  }
  return results;
}

function trustedOutputs(
  request: Resource<"PluginRequest">,
): Record<string, string> {
  const ready = request.status?.conditions?.find(
    (c) => c.type === OUTPUTS_READY,
  );
  return ready?.status === "True" &&
    ready.observedGeneration === request.metadata?.generation
    ? (request.status?.outputs ?? {})
    : {};
}

function templateData(data: Partial<TemplateData>): TemplateData {
  return {
    namespace: "",
    claim: "",
    request: "",
    serviceAccount: "",
    parameters: {},
    outputs: {},
    ...data,
  };
}

function parts(version: string | undefined): number[] | undefined {
  const match = VERSION.exec(version ?? "");
  return match ? match.slice(1).map(Number) : undefined;
}

function compare(a: readonly number[], b: readonly number[]): number {
  for (let i = 0; i < a.length; i++) {
    if (a[i] !== b[i]) return a[i] - b[i];
  }
  return 0;
}
