import "server-only";
import type { PlaneClaimTabData } from "@/components/planes/claim/data";
import type { PlanePageData } from "@/components/planes/plane-view";
import type { ReadError } from "@/components/planes/planes-table";
import type { KindName } from "@/lib/k8s/kinds";
import { get, list } from "@/lib/k8s/read";
import { CLAIM_LABEL, MANAGEMENT_CLUSTER_LABEL } from "@/lib/labels";
import { ownerName } from "@/lib/labels/owner-name";

/**
 * loadPlanePage reads the first snapshot of the plane page. It returns
 * undefined only when it read both the CamundaManagementCluster and the
 * claim of that name and neither exists. It returns the claim while no
 * CamundaManagementCluster is known, or when the CamundaManagementCluster
 * carries CLAIM_LABEL with the value name. When it returns a page, a failed
 * read of a kind gives no objects of that kind and an entry in readErrors.
 * pods are the pods of the plane in its namespace, which carry
 * MANAGEMENT_CLUSTER_LABEL with ownerName of the plane name.
 */
export async function loadPlanePage(
  namespace: string,
  name: string,
): Promise<PlanePageData | undefined> {
  const claimTab = loadClaimTab(namespace, name);
  const readErrors: ReadError[] = [];
  const failed =
    <T>(kind: KindName, empty: T) =>
    (err: unknown): T => {
      readErrors.push({
        kind,
        message: err instanceof Error ? err.message : String(err),
      });
      return empty;
    };
  const [
    plane,
    claim,
    namespaces,
    environments,
    optimizes,
    managementAuthConfigs,
    pods,
    platformConfigs,
    registrations,
  ] = await Promise.all([
    get("CamundaManagementCluster", namespace, name).catch(
      failed("CamundaManagementCluster", undefined),
    ),
    get("CamundaManagementClusterClaim", namespace, name).catch(
      failed("CamundaManagementClusterClaim", undefined),
    ),
    list("Namespace", {}).catch(failed("Namespace", [])),
    list("Environment", {}).catch(failed("Environment", [])),
    list("CamundaOptimize", {}).catch(failed("CamundaOptimize", [])),
    list("ManagementAuthConfig", {}).catch(failed("ManagementAuthConfig", [])),
    list("Pod", {
      namespace,
      labelSelector: `${MANAGEMENT_CLUSTER_LABEL}=${ownerName(name)}`,
    }).catch(failed("Pod", [])),
    list("CamundaPlatformConfig", {}).catch(
      failed("CamundaPlatformConfig", []),
    ),
    list("PluginRegistration", {}).catch(failed("PluginRegistration", [])),
  ]);
  const objectUnread = readErrors.some(
    (e) =>
      e.kind === "CamundaManagementCluster" ||
      e.kind === "CamundaManagementClusterClaim",
  );
  if (!plane && !claim && !objectUnread) return undefined;
  const joined =
    claim && (!plane || plane.metadata?.labels?.[CLAIM_LABEL] === name);
  return {
    namespace,
    name,
    now: new Date().toISOString(),
    ...(plane && { plane }),
    ...(joined && { claim }),
    namespaces,
    environments,
    optimizes,
    managementAuthConfigs,
    pods,
    platformConfigs,
    registrations,
    readErrors,
    claimTab: await claimTab,
  };
}

// loadClaimTab reads the lists of the Claim tab. Its read errors stay
// apart from those of the page, because the tab follows these kinds.
async function loadClaimTab(
  namespace: string,
  name: string,
): Promise<PlaneClaimTabData> {
  const readErrors: ReadError[] = [];
  const failed = (kind: KindName) => (err: unknown) => {
    readErrors.push({
      kind,
      message: err instanceof Error ? err.message : String(err),
    });
    return [];
  };
  const [clusters, platformConfigs, sections, registrations, requests] =
    await Promise.all([
      list("CamundaCluster", {}).catch(failed("CamundaCluster")),
      list("CamundaPlatformConfig", {}).catch(failed("CamundaPlatformConfig")),
      list("PluginSection", {}).catch(failed("PluginSection")),
      list("PluginRegistration", {}).catch(failed("PluginRegistration")),
      list("PluginRequest", {
        namespace,
        labelSelector: `${CLAIM_LABEL}=${name}`,
      }).catch(failed("PluginRequest")),
    ]);
  return {
    clusters,
    platformConfigs,
    sections,
    registrations,
    requests,
    readErrors,
  };
}
