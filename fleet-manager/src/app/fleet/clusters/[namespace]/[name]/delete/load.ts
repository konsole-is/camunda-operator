import "server-only";
import type { DeleteClusterData } from "@/components/cluster/delete-cluster-model";
import type { KindName } from "@/lib/k8s/kinds";
import { get, list, type ListOptions } from "@/lib/k8s/read";
import type { ReadError } from "@/lib/k8s/unread";

/**
 * loadDeleteClusterPage reads the first snapshot of the delete page of a
 * cluster from the cache of the server. It reads the claim, not the
 * cluster, so that the page still opens while the cluster goes: the claim
 * has the name of its cluster. It returns undefined when the claim does
 * not exist. A failed read of another kind gives no objects of that kind,
 * and its error in errors.
 */
export async function loadDeleteClusterPage(
  namespace: string,
  name: string,
): Promise<DeleteClusterData | undefined> {
  const claim = await get("CamundaClusterClaim", namespace, name);
  if (!claim) return undefined;

  const errors: ReadError[] = [];
  const failed = (kind: KindName) => (error: unknown) => {
    errors.push({ kind, message: messageOf(error) });
    return undefined;
  };
  const listed = <K extends KindName>(kind: K, opts: ListOptions = {}) =>
    list(kind, opts).catch((error: unknown) => failed(kind)(error) ?? []);
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
    get("CamundaCluster", namespace, name).catch(failed("CamundaCluster")),
    get("Namespace", undefined, namespace).catch(failed("Namespace")),
    listed("Environment"),
    listed("PluginSection"),
    listed("PluginRegistration"),
    listed("PluginRequest", inNamespace),
    listed("CamundaOptimize", inNamespace),
    listed("BackupSchedule", inNamespace),
    listed("LogicalBackupElasticsearch", inNamespace),
    listed("LogicalBackupRDBMS", inNamespace),
    listed("CamundaClusterPreset"),
    listed("CamundaRelease"),
  ]);
  return {
    claim,
    ...(cluster && { cluster }),
    ...(namespaceObject && { namespace: namespaceObject }),
    environments,
    sections,
    registrations,
    requests,
    optimizes,
    schedules,
    backups: { elasticsearch: backupsES, rdbms: backupsRDBMS },
    presets,
    releases,
    errors,
  };
}

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
