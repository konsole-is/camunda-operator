import { PlaneCreateFlow } from "@/components/planes/create/plane-create-flow";
import { list } from "@/lib/k8s/read";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function NewPlanePage() {
  // A failed read gives no objects of that kind. The watch of the kind
  // then shows its error above the steps.
  const [
    environments,
    namespaces,
    clusters,
    platformConfigs,
    managementClusters,
    claims,
  ] = await Promise.all([
    list("Environment").catch(() => []),
    list("Namespace").catch(() => []),
    list("CamundaCluster").catch(() => []),
    list("CamundaPlatformConfig").catch(() => []),
    list("CamundaManagementCluster").catch(() => []),
    list("CamundaManagementClusterClaim").catch(() => []),
  ]);
  return (
    <div className="flex flex-col gap-6">
      <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
        New management plane
      </h1>
      <PlaneCreateFlow
        initial={{
          environments,
          namespaces,
          clusters,
          platformConfigs,
          planes: { clusters: managementClusters, claims, namespaces },
        }}
      />
    </div>
  );
}
