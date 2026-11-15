import "server-only";
import type { PlanePageData } from "@/components/planes/plane-view";
import { listOrEmpty } from "@/lib/k8s/list-or-empty";
import { get } from "@/lib/k8s/read";
import { CLAIM_LABEL } from "@/lib/labels";

/**
 * loadPlanePage reads the first snapshot of the plane page. It returns
 * undefined when neither the CamundaManagementCluster nor the claim of that
 * name exists. The claim joins the plane while no CamundaManagementCluster
 * exists, or when the CamundaManagementCluster carries CLAIM_LABEL with the
 * value name. A failed read of a kind gives no objects of that kind.
 */
export async function loadPlanePage(
  namespace: string,
  name: string,
): Promise<PlanePageData | undefined> {
  const [plane, claim, namespaces, environments] = await Promise.all([
    get("CamundaManagementCluster", namespace, name).catch(() => undefined),
    get("CamundaManagementClusterClaim", namespace, name).catch(
      () => undefined,
    ),
    listOrEmpty("Namespace", {}),
    listOrEmpty("Environment", {}),
  ]);
  if (!plane && !claim) return undefined;
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
  };
}
