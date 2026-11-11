import type { ClusterSlotContext } from "@/components/cluster/slots";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";

/** NOW is the clock of slotContext. */
export const NOW = "2026-11-10T12:00:00Z";

/**
 * slotContext returns the slot context of a cluster page: the managed
 * cluster orders in team-a, which runs Camunda 8.9.1, its claim at
 * resourceVersion 41 with three brokers, the Environment prod, which allows
 * the preset medium and the releases camunda-8-9-0 and camunda-8-9-1, those
 * three objects, no other objects, and the write mode direct. over replaces
 * fields of the result, and merged follows the cluster and the lists of over.
 */
export function slotContext(
  over: Partial<ClusterSlotContext> = {},
): ClusterSlotContext {
  const base: Omit<ClusterSlotContext, "merged"> = {
    cluster: cluster(),
    claim: claim(),
    environment: environment(),
    environments: [environment()],
    presets: [preset("medium")],
    releases: [release("8.9.0"), release("8.9.1")],
    platformConfigs: [],
    managementAuthConfigs: [],
    sections: [],
    registrations: [],
    requests: [],
    backups: [],
    restores: [],
    backupStorage: { kind: "none", schedules: [] },
    schedules: [],
    fields: [],
    writeMode: "direct",
    now: NOW,
    ...over,
  };
  const spec = base.cluster.spec;
  return {
    merged: mergeCluster(
      base.presets.find((p) => p.metadata?.name === spec.presetRef)?.spec,
      base.releases.find((r) => r.metadata?.name === spec.releaseRef)?.spec,
      spec,
    ),
    ...base,
    ...over,
  };
}

/** cluster returns the CamundaCluster orders that the claim orders made, with spec merged over its default. */
export function cluster(spec: object = {}): Resource<"CamundaCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: {
      namespace: "team-a",
      name: "orders",
      labels: { [CLAIM_LABEL]: "orders" },
    },
    spec: {
      presetRef: "medium",
      releaseRef: "camunda-8-9-1",
      zeebe: { replicas: 3 },
      ...spec,
    },
    status: { management: { version: "8.9.1" } },
  } as unknown as Resource<"CamundaCluster">;
}

/** claim returns the claim orders at resourceVersion, with spec merged over its default. */
export function claim(
  resourceVersion = "41",
  spec: object = {},
): Resource<"CamundaClusterClaim"> {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace: "team-a", name: "orders", resourceVersion },
    spec: {
      presetRef: "medium",
      releaseRef: "camunda-8-9-1",
      cluster: { zeebe: { replicas: 3 } },
      ...spec,
    },
  } as unknown as Resource<"CamundaClusterClaim">;
}

function environment(): Resource<"Environment"> {
  return {
    kind: "Environment",
    metadata: { name: "prod" },
    spec: {
      displayName: "Production",
      presets: { camundaCluster: ["medium"] },
      releases: {
        allowed: ["camunda-8-9-0", "camunda-8-9-1"],
        default: "camunda-8-9-1",
      },
    },
  } as unknown as Resource<"Environment">;
}

function preset(name: string): Resource<"CamundaClusterPreset"> {
  return {
    kind: "CamundaClusterPreset",
    metadata: { name },
    spec: { cluster: {} },
  } as unknown as Resource<"CamundaClusterPreset">;
}

function release(version: string): Resource<"CamundaRelease"> {
  return {
    kind: "CamundaRelease",
    metadata: { name: `camunda-${version.replaceAll(".", "-")}` },
    spec: { version },
  } as unknown as Resource<"CamundaRelease">;
}
