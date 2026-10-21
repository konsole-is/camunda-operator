import "server-only";
import type { Resource } from "@/lib/k8s/kinds";
import { list } from "@/lib/k8s/read";
import { CLUSTER_LABEL, COMPONENT_LABEL } from "@/lib/labels";
import { ownerName } from "@/lib/labels/server";
import { readSecretValues } from "@/lib/secrets/values";
import type { PartitionHealth, Unavailable } from "./types";

const TIMEOUT_MS = 5_000;

/**
 * BrokerAnswer is the answer of GET /actuator/partitions of one broker:
 * its partitions by partition id. A follower can have a null phase.
 */
export type BrokerAnswer = {
  pod: string;
  partitions: Record<
    string,
    { role?: string | null; streamProcessorPhase?: string | null }
  >;
};

/**
 * mergePartitions returns partitions 1 to count with the leader and the
 * followers that the answers name, and the processor phase of the leader.
 * A partition that no answer leads has no leader. A role other than LEADER
 * or FOLLOWER counts as neither.
 */
export function mergePartitions(
  answers: readonly BrokerAnswer[],
  count: number,
): PartitionHealth[] {
  return Array.from({ length: count }, (_, i) => {
    const partition = i + 1;
    const merged: PartitionHealth = {
      partition,
      followers: [],
      health: "unknown",
    };
    for (const { pod, partitions } of answers) {
      const state = partitions[String(partition)];
      if (state?.role === "FOLLOWER") merged.followers.push(pod);
      if (state?.role === "LEADER") {
        merged.leader = pod;
        if (state.streamProcessorPhase) {
          merged.processorPhase = state.streamProcessorPhase;
        }
      }
    }
    return merged;
  });
}

/**
 * partitionHealth asks GET /actuator/partitions of each broker pod of
 * cluster, at the pod IP with the scheme, port, and path of
 * status.management.endpoint, and with the basic auth of its Secret when
 * the method is basic. It runs on the server only. Health is "unknown",
 * because the management port does not give it; the metric zeebe_health
 * gives it through promRange. A broker that does not answer in 5 seconds
 * is left out. It returns "unavailable" when the cluster has no
 * status.management, the auth Secret or one of its keys is missing, no
 * broker pod has an IP, or no broker answers.
 */
export async function partitionHealth(
  cluster: Resource<"CamundaCluster">,
): Promise<PartitionHealth[] | Unavailable> {
  const management = cluster.status?.management;
  const namespace = cluster.metadata?.namespace;
  if (!management || !namespace) return "unavailable";

  const headers: Record<string, string> = { Accept: "application/json" };
  const ref = management.auth.credentialsSecretRef;
  if (management.auth.method === "basic") {
    if (!ref) return "unavailable";
    const values = await readSecretValues(namespace, ref.name);
    const username = values?.[ref.usernameKey ?? "username"];
    const password = values?.[ref.passwordKey ?? "password"];
    if (username === undefined || password === undefined) {
      return "unavailable";
    }
    headers.Authorization =
      "Basic " + Buffer.from(`${username}:${password}`).toString("base64");
  }

  const pods = await list("Pod", {
    namespace,
    labelSelector: `${CLUSTER_LABEL}=${ownerName(cluster.metadata?.name ?? "")},${COMPONENT_LABEL}=zeebe`,
  });
  const brokers = pods.flatMap((pod) =>
    pod.status?.podIP && pod.metadata?.name
      ? [{ pod: pod.metadata.name, ip: pod.status.podIP }]
      : [],
  );
  if (brokers.length === 0) return "unavailable";

  const settled = await Promise.allSettled(
    brokers.map(({ pod, ip }) =>
      askBroker(brokerURL(management.endpoint, ip), headers, pod),
    ),
  );
  const answers = settled.flatMap((s) =>
    s.status === "fulfilled" && s.value ? [s.value] : [],
  );
  if (answers.length === 0) return "unavailable";
  return mergePartitions(answers, management.partitions);
}

function brokerURL(endpoint: string, ip: string): URL {
  const url = new URL(endpoint);
  url.hostname = ip.includes(":") ? `[${ip}]` : ip;
  url.pathname = url.pathname.replace(/\/+$/, "") + "/actuator/partitions";
  url.search = "";
  url.hash = "";
  return url;
}

async function askBroker(
  url: URL,
  headers: Record<string, string>,
  pod: string,
): Promise<BrokerAnswer | undefined> {
  // A redirect would carry the credentials to another address.
  const response = await fetch(url, {
    headers,
    signal: AbortSignal.timeout(TIMEOUT_MS),
    redirect: "manual",
  });
  if (response.status !== 200) return undefined;
  const body = (await response.json()) as unknown;
  if (!isObject(body) || !Object.values(body).every(isObject)) {
    return undefined;
  }
  return { pod, partitions: body as BrokerAnswer["partitions"] };
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
