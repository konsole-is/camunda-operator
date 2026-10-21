import { PlaneListLive } from "@/components/planes/plane-list";
import { list } from "@/lib/k8s/read";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function PlanesPage() {
  // A failed read gives no objects of that kind. The watch of the kind
  // then shows its error on the page.
  const [clusters, claims, namespaces] = await Promise.all([
    list("CamundaManagementCluster").catch(() => []),
    list("CamundaManagementClusterClaim").catch(() => []),
    list("Namespace").catch(() => []),
  ]);
  return <PlaneListLive initial={{ clusters, claims, namespaces }} />;
}
