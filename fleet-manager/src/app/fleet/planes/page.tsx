import { PlaneListLive } from "@/components/planes/plane-list";
import { list } from "@/lib/k8s/read";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function PlanesPage() {
  const [clusters, claims, namespaces] = await Promise.all([
    list("CamundaManagementCluster"),
    list("CamundaManagementClusterClaim"),
    list("Namespace"),
  ]);
  return <PlaneListLive initial={{ clusters, claims, namespaces }} />;
}
