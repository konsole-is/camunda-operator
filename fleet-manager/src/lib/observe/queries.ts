import "server-only";
import { type TemplateData, parseTemplate, render } from "@/lib/gotemplate";
import type { Resource } from "@/lib/k8s/kinds";
import type { CoreComponent, CoreQuery, MetricDef, QueryError } from "./types";

const CADVISOR =
  "https://github.com/google/cadvisor/blob/master/docs/storage/prometheus.md";
const CAMUNDA =
  "https://docs.camunda.io/docs/self-managed/operational-guides/monitoring/metrics/#available-metrics";
const SELECT = 'namespace="{{ .namespace }}", pod=~"{{ .pods }}"';

// The kubelet reports a series for each container and one for the pod
// cgroup with an empty container label. container!="" drops the second, so
// the sum by pod does not count each pod twice.
const USAGE: CoreQuery[] = [
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
];

const ZEEBE: CoreQuery[] = [
  {
    label: "Records processed",
    query: `sum(rate(zeebe_stream_processor_records_total{${SELECT}, action="processed"}[5m]))`,
    unit: "records/s",
    metric: "zeebe_stream_processor_records_total",
    source: CAMUNDA,
  },
  {
    label: "Records exported",
    query: `sum(rate(zeebe_exporter_events_total{${SELECT}, action="exported"}[5m]))`,
    unit: "events/s",
    metric: "zeebe_exporter_events_total",
    source: CAMUNDA,
  },
  {
    label: "Requests dropped by backpressure",
    query: `sum(rate(zeebe_dropped_request_count_total{${SELECT}}[5m]))`,
    unit: "requests/s",
    metric: "zeebe_dropped_request_count_total",
    source: CAMUNDA,
  },
  {
    label: "Partition health",
    query: `zeebe_health{${SELECT}}`,
    metric: "zeebe_health",
    source: CAMUNDA,
  },
];

/**
 * coreQueries holds the fixed queries of each core component. Each query
 * reads one documented metric, which metric and source name. The query is
 * a metric template with {{ .namespace }} and {{ .pods }}.
 */
export const coreQueries: Record<CoreComponent, CoreQuery[]> = {
  zeebe: [...USAGE, ...ZEEBE],
  gateway: USAGE,
  operate: USAGE,
  tasklist: USAGE,
  admin: USAGE,
  connectors: USAGE,
};

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
 * coreMetricDefs returns the queries of coreQueries for one component,
 * rendered for its pods in namespace. A component without pods has no
 * queries.
 */
export function coreMetricDefs(
  component: CoreComponent,
  namespace: string,
  pods: readonly string[],
): MetricDef[] {
  if (pods.length === 0) return [];
  const data = templateData({ namespace, pods: podsPattern(pods) });
  return coreQueries[component].map(({ label, query, unit, source }) => {
    const rendered = render(query, data, "metric");
    if ("error" in rendered) {
      throw new Error(`The core query ${label} does not render: ${rendered.error}`);
    }
    return { label, query: rendered.text, ...(unit && { unit }), source };
  });
}

/**
 * registrationQueries renders the metricQueries of registration for one
 * request, with pods as the pods of the request. The source of each result
 * is the name of the registration. A query that does not render is a
 * QueryError with the reason. A query that uses {{ .pods }} is left out
 * while pods is empty, so it never matches the pods of others.
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
    outputs: request.status?.outputs ?? {},
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
