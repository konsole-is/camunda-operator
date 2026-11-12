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
import { BACKPRESSURE_RATES, inVersionRange } from "./queries";
import { podUsage } from "./readers";

const BACKPRESSURE_WINDOW_MS = 300_000;
const BACKPRESSURE_STEP_SECONDS = 60;
const BROKER = "zeebe";
// promRange names a series by its labels in key order.
const RATE_SERIES =
  /^kind="(received|dropped)", namespace="([^"]*)", pod="([^"]*)"$/;

type Rates = { received?: number; dropped?: number };

/**
 * fleetPodWarnings returns the pod and metric warnings of every
 * CamundaCluster, by "<namespace>/<name>". missing names metrics-server
 * when metrics.k8s.io gives no usage in a namespace with cluster pods, and
 * prometheus when the backpressure query gives "unavailable" or rejects.
 * A cluster without a version, or with a version outside the range of
 * BACKPRESSURE_RATES, gets no backpressure warning. It rejects when the
 * cache cannot be read.
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

  const brokers = new Map<string, string>();
  for (const cluster of clusters) {
    if (
      !inVersionRange(
        cluster.status?.management?.version,
        BACKPRESSURE_RATES.versions,
      )
    ) {
      continue;
    }
    const namespace = cluster.metadata?.namespace ?? "";
    const name = cluster.metadata?.name ?? "";
    for (const pod of byCluster.get(`${namespace}/${ownerName(name)}`) ?? []) {
      if (
        pod.metadata?.labels?.[COMPONENT_LABEL] === BROKER &&
        pod.metadata.name
      ) {
        brokers.set(
          `${namespace}/${pod.metadata.name}`,
          `${namespace}/${name}`,
        );
      }
    }
  }
  const shares = await backpressureShares(brokers, now);
  if (shares === "unavailable") missing.add("prometheus");

  const entries = clusters.map((cluster): [string, Warning[]] => {
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
    const share =
      shares === "unavailable" ? undefined : shares.get(`${namespace}/${name}`);
    const backpressure =
      share === undefined ? undefined : backpressureWarning(share);
    if (backpressure) warnings.push(backpressure);
    return [`${namespace}/${name}`, warnings];
  });

  return {
    clusters: Object.fromEntries(entries),
    missing: (["metrics-server", "prometheus"] as const).filter((s) =>
      missing.has(s),
    ),
  };
}

// backpressureShares maps brokers, "<namespace>/<pod>" to the
// "<namespace>/<name>" of its cluster, to the backpressure share of each
// cluster. A cluster without a recent share is not in the map.
async function backpressureShares(
  brokers: ReadonlyMap<string, string>,
  now: Date,
): Promise<Map<string, number> | "unavailable"> {
  if (brokers.size === 0) return new Map();
  const series = await promRange(BACKPRESSURE_RATES.query, {
    start: now.getTime() - BACKPRESSURE_WINDOW_MS,
    end: now.getTime(),
    stepSeconds: BACKPRESSURE_STEP_SECONDS,
  }).catch(() => "unavailable" as const);
  if (series === "unavailable") return series;

  const byCluster = new Map<string, Map<number, Rates>>();
  for (const { name, points } of series) {
    const [, kind, namespace, pod] = RATE_SERIES.exec(name) ?? [];
    const cluster = brokers.get(`${namespace}/${pod}`);
    if (!cluster) continue;
    const times = byCluster.get(cluster) ?? new Map<number, Rates>();
    byCluster.set(cluster, times);
    for (const [time, value] of points) {
      const rates = times.get(time) ?? {};
      rates[kind as keyof Rates] = (rates[kind as keyof Rates] ?? 0) + value;
      times.set(time, rates);
    }
  }

  const shares = new Map<string, number>();
  for (const [cluster, times] of byCluster) {
    const newest = [...times]
      .map(([time, rates]) => ({ time, share: shareOf(rates) }))
      .filter(({ share }) => Number.isFinite(share))
      .sort((a, b) => b.time - a.time)[0];
    // promRange leaves out a point without a value, so an older point can be
    // the newest one while the brokers get no request.
    if (
      newest &&
      newest.time >= now.getTime() - BACKPRESSURE_STEP_SECONDS * 1000
    ) {
      shares.set(cluster, newest.share);
    }
  }
  return shares;
}

function shareOf({ received, dropped }: Rates): number {
  return received === undefined || dropped === undefined
    ? Number.NaN
    : dropped / received;
}
