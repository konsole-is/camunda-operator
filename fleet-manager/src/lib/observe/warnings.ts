import "server-only";
import type { Resource } from "@/lib/k8s/kinds";
import { list } from "@/lib/k8s/read";
import { CLUSTER_LABEL, COMPONENT_LABEL } from "@/lib/labels";
import { ownerName } from "@/lib/labels/server";
import {
  type PodWarnings,
  type Warning,
  type WarningSource,
  backpressureWarning,
  containerWarnings,
  limitWarnings,
} from "@/lib/warnings";
import { promRange } from "./prometheus";
import { BACKPRESSURE_SHARE, inVersionRange, renderCoreQuery } from "./queries";
import { podUsage } from "./readers";

const BACKPRESSURE_WINDOW_MS = 300_000;
const BACKPRESSURE_STEP_SECONDS = 60;
const BROKER = "zeebe";

/**
 * fleetPodWarnings returns the pod and metric warnings of every
 * CamundaCluster, by "<namespace>/<name>". missing names metrics-server
 * when metrics.k8s.io gives no usage in a namespace with cluster pods, and
 * prometheus when the backpressure query of a cluster gives "unavailable"
 * or rejects. A cluster without a version, or with a version outside the
 * range of BACKPRESSURE_SHARE, gets no backpressure query. It rejects when
 * the cache cannot be read.
 */
export async function fleetPodWarnings(now: Date): Promise<PodWarnings> {
  const [clusters, pods] = await Promise.all([
    list("CamundaCluster"),
    list("Pod", { labelSelector: CLUSTER_LABEL }),
  ]);
  const byCluster = new Map<string, Resource<"Pod">[]>();
  for (const pod of pods) {
    const owner = `${pod.metadata?.namespace}/${pod.metadata?.labels?.[CLUSTER_LABEL]}`;
    byCluster.set(owner, [...(byCluster.get(owner) ?? []), pod]);
  }
  const missing = new Set<WarningSource>();
  const iso = now.toISOString();

  const namespaces = [...new Set(pods.map((p) => p.metadata?.namespace ?? ""))];
  const usage = new Map(
    (
      await Promise.all(
        namespaces.map(async (namespace) => {
          const answer = await podUsage(namespace, CLUSTER_LABEL).catch(
            () => "unavailable" as const,
          );
          if (answer === "unavailable") {
            missing.add("metrics-server");
            return [];
          }
          return answer.map((u) => [`${namespace}/${u.pod}`, u] as const);
        }),
      )
    ).flat(),
  );

  const entries = await Promise.all(
    clusters.map(async (cluster): Promise<[string, Warning[]]> => {
      const namespace = cluster.metadata?.namespace ?? "";
      const name = cluster.metadata?.name ?? "";
      const own = byCluster.get(`${namespace}/${ownerName(name)}`) ?? [];
      const warnings = own.flatMap((pod) => {
        const used = usage.get(`${namespace}/${pod.metadata?.name}`);
        return [
          ...containerWarnings(pod, iso),
          ...(used ? limitWarnings(used) : []),
        ];
      });
      const backpressure = await backpressureOf(cluster, own, now);
      if (backpressure === "unavailable") missing.add("prometheus");
      else if (backpressure) warnings.push(backpressure);
      return [`${namespace}/${name}`, warnings];
    }),
  );

  return {
    clusters: Object.fromEntries(entries),
    missing: (["metrics-server", "prometheus"] as const).filter((s) =>
      missing.has(s),
    ),
  };
}

async function backpressureOf(
  cluster: Resource<"CamundaCluster">,
  pods: readonly Resource<"Pod">[],
  now: Date,
): Promise<Warning | "unavailable" | undefined> {
  const brokers = pods.flatMap((p) =>
    p.metadata?.labels?.[COMPONENT_LABEL] === BROKER && p.metadata.name
      ? [p.metadata.name]
      : [],
  );
  if (
    brokers.length === 0 ||
    !inVersionRange(
      cluster.status?.management?.version,
      BACKPRESSURE_SHARE.versions,
    )
  ) {
    return undefined;
  }
  const series = await promRange(
    renderCoreQuery(
      BACKPRESSURE_SHARE,
      cluster.metadata?.namespace ?? "",
      brokers,
    ),
    {
      start: now.getTime() - BACKPRESSURE_WINDOW_MS,
      end: now.getTime(),
      stepSeconds: BACKPRESSURE_STEP_SECONDS,
    },
  ).catch(() => "unavailable" as const);
  if (series === "unavailable") return series;
  const points = series[0]?.points ?? [];
  const last = points.findLast(([, value]) => Number.isFinite(value));
  return last ? backpressureWarning(last[1]) : undefined;
}
