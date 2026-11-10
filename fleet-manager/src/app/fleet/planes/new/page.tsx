import { PlaneCreateFlow } from "@/components/planes/create/plane-create-flow";
import { list } from "@/lib/k8s/read";
import { ACCESS_SECTION, SECTION_LABEL } from "@/lib/labels";
import { loadBundledData } from "@/startup/bundled-data";

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
    releases,
    managementClusters,
    claims,
    sections,
    registrations,
    accessRequests,
    authConfigs,
    databaseServerPresets,
    databaseConfigs,
    images,
  ] = await Promise.all([
    list("Environment").catch(() => []),
    list("Namespace").catch(() => []),
    list("CamundaCluster").catch(() => []),
    list("CamundaPlatformConfig").catch(() => []),
    list("CamundaRelease").catch(() => []),
    list("CamundaManagementCluster").catch(() => []),
    list("CamundaManagementClusterClaim").catch(() => []),
    list("PluginSection").catch(() => []),
    list("PluginRegistration").catch(() => []),
    list("PluginRequest", {
      labelSelector: `${SECTION_LABEL}=${ACCESS_SECTION}`,
    }).catch(() => []),
    list("ManagementAuthConfig").catch(() => []),
    list("DatabaseServerPreset").catch(() => []),
    list("DatabaseConfig").catch(() => []),
    // The server checked both files at start. A table that cannot be read
    // now prefills no version.
    loadBundledData().then(
      (bundled) => bundled.imagesTable,
      () => ({ schemaVersion: 1 as const, minors: {} }),
    ),
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
          releases,
          sections,
          registrations,
          accessRequests,
          authConfigs,
          databaseServerPresets,
          databaseConfigs,
          planes: { clusters: managementClusters, claims, namespaces },
          images,
        }}
      />
    </div>
  );
}
