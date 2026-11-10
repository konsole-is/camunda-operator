import "server-only";
import { clusterOptimize } from "@/components/cluster/optimize";
import type { Resource } from "@/lib/k8s/kinds";
import { get, list } from "@/lib/k8s/read";
import { CLUSTER_LABEL } from "@/lib/labels";
import { ownerName } from "@/lib/labels/server";
import { getSettings } from "@/lib/settings";
import { requestPods } from "./kube";
import { promRange } from "./prometheus";
import {
  POD_QUERIES,
  coreMetricDefs,
  queriesFor,
  registrationQueries,
} from "./queries";
import { componentPodSelector } from "./selectors";
import type {
  ChartError,
  MetricDef,
  MetricsAnswer,
  MetricsComponent,
  MetricsRange,
  Series,
  TimeRange,
  Unavailable,
} from "./types";

type ChartDef = MetricDef & { declaredBy?: string };

const ONE_LABEL = /^[A-Za-z_][A-Za-z0-9_]*="([^"]*)"$/;

/** METRICS_RANGES gives the length and the step of each MetricsRange. */
export const METRICS_RANGES: Record<
  MetricsRange,
  { seconds: number; stepSeconds: number }
> = {
  "1h": { seconds: 3600, stepSeconds: 30 },
  "6h": { seconds: 21600, stepSeconds: 120 },
  "24h": { seconds: 86400, stepSeconds: 300 },
};

/** timeRange returns range back from now, in Unix milliseconds. */
export function timeRange(range: MetricsRange, now: number): TimeRange {
  const { seconds, stepSeconds } = METRICS_RANGES[range];
  return { start: now - seconds * 1000, end: now, stepSeconds };
}

/** podVolumes returns the PersistentVolumeClaim names of the volumes of pods, unique and sorted. */
export function podVolumes(pods: readonly Resource<"Pod">[]): string[] {
  const claims = pods.flatMap((pod) =>
    (pod.spec?.volumes ?? []).flatMap((v) =>
      v.persistentVolumeClaim ? [v.persistentVolumeClaim.claimName] : [],
    ),
  );
  return [...new Set(claims)].sort();
}

/**
 * chartsOf returns the answer of defs, where answers[i] is the Prometheus
 * answer of defs[i]. It is "unavailable" when there is a def and no answer
 * is a list of series. Otherwise each def with at least one series gives a
 * chart, in the order of defs. A series named by one label, such as
 * pod="orders-zeebe-0", takes the value of that label as its name.
 */
export function chartsOf(
  defs: readonly ChartDef[],
  answers: readonly (Series[] | Unavailable)[],
  errors: readonly ChartError[],
): MetricsAnswer {
  if (defs.length > 0 && answers.every((a) => a === "unavailable")) {
    return { prometheus: "unavailable" };
  }
  const charts = defs.flatMap((def, i) => {
    const series = answers[i];
    if (series === "unavailable" || !series?.length) return [];
    return [
      {
        label: def.label,
        ...(def.unit && { unit: def.unit }),
        source: def.source,
        ...(def.declaredBy && { declaredBy: def.declaredBy }),
        series: series.map((s) => ({
          ...s,
          name: ONE_LABEL.exec(s.name)?.[1] ?? s.name,
        })),
      },
    ];
  });
  return { prometheus: "answered", charts, errors: [...errors] };
}

/**
 * componentMetrics returns the charts of one component of the CamundaCluster
 * cluster in namespace over range back from now: the queries of queriesFor
 * for the Camunda version of the cluster, for the pods of the component.
 * For "optimize" the pods are those of the CamundaOptimize of the cluster;
 * without one there are no charts. It is undefined when the cluster does
 * not exist, and "notConfigured", without a pod read, when the settings
 * have no Prometheus URL. It rejects when the cache or the settings
 * cannot be read.
 */
export async function componentMetrics(
  namespace: string,
  cluster: string,
  component: MetricsComponent,
  range: MetricsRange,
  now: number,
): Promise<MetricsAnswer | undefined> {
  const live = await get("CamundaCluster", namespace, cluster);
  if (!live) return undefined;
  if (!(await prometheusConfigured())) return { prometheus: "notConfigured" };

  const name = live.metadata?.name ?? cluster;
  let optimizeUid: string | undefined;
  if (component === "optimize") {
    const optimize = clusterOptimize(
      name,
      await list("CamundaOptimize", { namespace }),
    );
    optimizeUid = optimize?.metadata?.uid;
    if (!optimizeUid) return chartsOf([], [], []);
  }
  const pods = await list("Pod", {
    namespace,
    labelSelector: componentPodSelector(
      `${CLUSTER_LABEL}=${ownerName(name)}`,
      component,
      optimizeUid,
    ),
  });
  const defs = coreMetricDefs(
    queriesFor(component, live.status?.management?.version),
    namespace,
    { pods: podNames(pods), volumes: podVolumes(pods) },
  );
  return answer(defs, [], range, now);
}

/**
 * requestMetrics returns the charts of the PluginRequest request in
 * namespace over range back from now: the CPU and memory of the pods of
 * the request, and the metricQueries of its registration, each with
 * declaredBy set to the label of the registration, or its name. A query of
 * the registration that does not render is a ChartError. Without the
 * registration, only the pod charts show. It is undefined when the request
 * does not exist, and "notConfigured", without a pod read, when the
 * settings have no Prometheus URL. It rejects when the cache or the
 * settings cannot be read.
 */
export async function requestMetrics(
  namespace: string,
  request: string,
  range: MetricsRange,
  now: number,
): Promise<MetricsAnswer | undefined> {
  const live = await get("PluginRequest", namespace, request);
  if (!live) return undefined;
  if (!(await prometheusConfigured())) return { prometheus: "notConfigured" };

  const pods = await requestPods(live);
  // The disk query needs the claim names of the pods. A registration that
  // wants a disk chart declares its own query.
  const defs: ChartDef[] = coreMetricDefs(POD_QUERIES, namespace, {
    pods,
    volumes: [],
  });
  const errors: ChartError[] = [];
  const registration = await get(
    "PluginRegistration",
    undefined,
    live.spec.registration,
  );
  if (registration) {
    const declaredBy =
      registration.spec.label || (registration.metadata?.name ?? "");
    for (const q of registrationQueries(registration, live, pods)) {
      if ("error" in q) {
        errors.push({ label: q.label, declaredBy, error: q.error });
      } else {
        defs.push({ ...q, declaredBy });
      }
    }
  }
  return answer(defs, errors, range, now);
}

async function answer(
  defs: readonly ChartDef[],
  errors: readonly ChartError[],
  range: MetricsRange,
  now: number,
): Promise<MetricsAnswer> {
  const window = timeRange(range, now);
  const answers = await Promise.all(
    defs.map((def) => promRange(def.query, window)),
  );
  return chartsOf(defs, answers, errors);
}

async function prometheusConfigured(): Promise<boolean> {
  return Boolean((await getSettings())["prometheus.url"]);
}

function podNames(pods: readonly Resource<"Pod">[]): string[] {
  return pods.flatMap((p) => (p.metadata?.name ? [p.metadata.name] : []));
}
