import "server-only";
import type { DeletePlaneData } from "@/components/planes/delete-plane-model";
import type { KindName } from "@/lib/k8s/kinds";
import { get, list, type ListOptions } from "@/lib/k8s/read";
import type { ReadError } from "@/lib/k8s/unread";

/**
 * loadDeletePlanePage reads the first snapshot of the delete page of a
 * management plane from the cache of the server. It reads the claim, so
 * that the page still opens while the plane goes: the plane has the name
 * of its claim. It returns undefined when the claim does not exist. A
 * failed read of another kind gives no objects of that kind, and its error
 * in errors.
 */
export async function loadDeletePlanePage(
  namespace: string,
  name: string,
): Promise<DeletePlaneData | undefined> {
  const claim = await get("CamundaManagementClusterClaim", namespace, name);
  if (!claim) return undefined;

  const errors: ReadError[] = [];
  const failed = (kind: KindName) => (error: unknown) => {
    errors.push({
      kind,
      message: error instanceof Error ? error.message : String(error),
    });
    return undefined;
  };
  const listed = <K extends KindName>(kind: K, opts: ListOptions = {}) =>
    list(kind, opts).catch((error: unknown) => failed(kind)(error) ?? []);
  const [
    plane,
    namespaceObject,
    environments,
    sections,
    registrations,
    requests,
    optimizes,
  ] = await Promise.all([
    get("CamundaManagementCluster", namespace, name).catch(
      failed("CamundaManagementCluster"),
    ),
    get("Namespace", undefined, namespace).catch(failed("Namespace")),
    listed("Environment"),
    listed("PluginSection"),
    listed("PluginRegistration"),
    listed("PluginRequest", { namespace }),
    listed("CamundaOptimize"),
  ]);
  return {
    claim,
    ...(plane && { plane }),
    ...(namespaceObject && { namespace: namespaceObject }),
    environments,
    sections,
    registrations,
    requests,
    optimizes,
    errors,
  };
}
