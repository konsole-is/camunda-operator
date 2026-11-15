import "server-only";
import type { DeleteClusterData } from "@/components/cluster/delete-cluster-model";
import { listOrEmpty } from "@/lib/k8s/list-or-empty";
import { get } from "@/lib/k8s/read";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";

/**
 * loadDeleteClusterPage reads the first snapshot of the delete page of a
 * cluster from the cache of the server. It reads the claim, not the
 * cluster, so that the page still opens while the cluster goes: the claim
 * has the name of its cluster. It returns undefined when the claim does
 * not exist. A failed read of another kind gives no objects of that kind.
 */
export async function loadDeleteClusterPage(
  namespace: string,
  name: string,
): Promise<DeleteClusterData | undefined> {
  const claim = await get("CamundaClusterClaim", namespace, name);
  if (!claim) return undefined;

  const inNamespace = { namespace };
  const [
    cluster,
    namespaceObject,
    environments,
    sections,
    registrations,
    requests,
    optimizes,
    schedules,
    backupsES,
    backupsRDBMS,
    presets,
    releases,
  ] = await Promise.all([
    get("CamundaCluster", namespace, name).catch(() => undefined),
    get("Namespace", undefined, namespace).catch(() => undefined),
    listOrEmpty("Environment", {}),
    listOrEmpty("PluginSection", {}),
    listOrEmpty("PluginRegistration", {}),
    listOrEmpty("PluginRequest", {
      namespace,
      labelSelector: `${CLAIM_LABEL}=${name}`,
    }),
    listOrEmpty("CamundaOptimize", inNamespace),
    listOrEmpty("BackupSchedule", inNamespace),
    listOrEmpty("LogicalBackupElasticsearch", inNamespace),
    listOrEmpty("LogicalBackupRDBMS", inNamespace),
    listOrEmpty("CamundaClusterPreset", {}),
    listOrEmpty("CamundaRelease", {}),
  ]);
  const environmentName =
    namespaceObject?.metadata?.labels?.[ENVIRONMENT_LABEL];
  const environment =
    environmentName === undefined
      ? undefined
      : environments.find((e) => e.metadata?.name === environmentName);
  const spec = cluster?.spec;
  const volumes = spec
    ? mergeCluster(
        presets.find((p) => p.metadata?.name === spec.presetRef)?.spec,
        releases.find((r) => r.metadata?.name === spec.releaseRef)?.spec,
        spec,
      ).spec.zeebe?.persistentVolumeClaimRetentionPolicy?.whenDeleted
    : undefined;
  return {
    claim,
    ...(cluster && { cluster }),
    ...(environment && { environment }),
    sections,
    registrations,
    requests,
    optimizes,
    schedules,
    backups: { elasticsearch: backupsES, rdbms: backupsRDBMS },
    volumes: volumes ?? "Delete",
  };
}
