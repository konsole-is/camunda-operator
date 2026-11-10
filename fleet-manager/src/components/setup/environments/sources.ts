"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import type { ReadError } from "@/components/planes/planes-table";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { CamundaManagementCluster } from "@/generated/crds/camundamanagementcluster";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { DatabaseServerPreset } from "@/generated/crds/databaseserverpreset";
import type { ElasticsearchClusterPreset } from "@/generated/crds/elasticsearchclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { useResources } from "@/lib/k8s/hooks";
import type { KindName, Resource } from "@/lib/k8s/kinds";

/** EnvironmentSources holds every list that the Environments page reads, in all namespaces. */
export type EnvironmentSources = {
  environments: Environment[];
  namespaces: V1Namespace[];
  clusterClaims: CamundaClusterClaim[];
  planeClaims: CamundaManagementClusterClaim[];
  clusters: CamundaCluster[];
  planes: CamundaManagementCluster[];
  sections: PluginSection[];
  registrations: PluginRegistration[];
  requests: PluginRequest[];
  clusterPresets: CamundaClusterPreset[];
  elasticsearchPresets: ElasticsearchClusterPreset[];
  databasePresets: DatabaseServerPreset[];
  releases: CamundaRelease[];
  platformConfigs: CamundaPlatformConfig[];
};

const SOURCE_KINDS = {
  environments: "Environment",
  namespaces: "Namespace",
  clusterClaims: "CamundaClusterClaim",
  planeClaims: "CamundaManagementClusterClaim",
  clusters: "CamundaCluster",
  planes: "CamundaManagementCluster",
  sections: "PluginSection",
  registrations: "PluginRegistration",
  requests: "PluginRequest",
  clusterPresets: "CamundaClusterPreset",
  elasticsearchPresets: "ElasticsearchClusterPreset",
  databasePresets: "DatabaseServerPreset",
  releases: "CamundaRelease",
  platformConfigs: "CamundaPlatformConfig",
} as const satisfies Record<keyof EnvironmentSources, KindName>;

/**
 * useEnvironmentSources follows each list of initial live. errors holds the
 * error of each failed watch. The lists then keep their last items, which
 * can be out of date.
 */
export function useEnvironmentSources(
  initial: EnvironmentSources,
): EnvironmentSources & { errors: ReadError[] } {
  const environments = useLive("Environment", initial.environments);
  const namespaces = useLive("Namespace", initial.namespaces);
  const clusterClaims = useLive("CamundaClusterClaim", initial.clusterClaims);
  const planeClaims = useLive(
    "CamundaManagementClusterClaim",
    initial.planeClaims,
  );
  const clusters = useLive("CamundaCluster", initial.clusters);
  const planes = useLive("CamundaManagementCluster", initial.planes);
  const sections = useLive("PluginSection", initial.sections);
  const registrations = useLive("PluginRegistration", initial.registrations);
  const requests = useLive("PluginRequest", initial.requests);
  const clusterPresets = useLive(
    "CamundaClusterPreset",
    initial.clusterPresets,
  );
  const elasticsearchPresets = useLive(
    "ElasticsearchClusterPreset",
    initial.elasticsearchPresets,
  );
  const databasePresets = useLive(
    "DatabaseServerPreset",
    initial.databasePresets,
  );
  const releases = useLive("CamundaRelease", initial.releases);
  const platformConfigs = useLive(
    "CamundaPlatformConfig",
    initial.platformConfigs,
  );
  const results = {
    environments,
    namespaces,
    clusterClaims,
    planeClaims,
    clusters,
    planes,
    sections,
    registrations,
    requests,
    clusterPresets,
    elasticsearchPresets,
    databasePresets,
    releases,
    platformConfigs,
  };
  const errors = (Object.keys(results) as (keyof EnvironmentSources)[]).flatMap(
    (key) => {
      const error = results[key].error;
      return error ? [{ kind: SOURCE_KINDS[key], message: error }] : [];
    },
  );
  return {
    environments: environments.items,
    namespaces: namespaces.items,
    clusterClaims: clusterClaims.items,
    planeClaims: planeClaims.items,
    clusters: clusters.items,
    planes: planes.items,
    sections: sections.items,
    registrations: registrations.items,
    requests: requests.items,
    clusterPresets: clusterPresets.items,
    elasticsearchPresets: elasticsearchPresets.items,
    databasePresets: databasePresets.items,
    releases: releases.items,
    platformConfigs: platformConfigs.items,
    errors,
  };
}

// A Resource is plain JSON, so it differs from its model type only in its
// timestamps, which the Environments page does not read.
function useLive<K extends KindName, T>(
  kind: K,
  initial: T[],
): { items: T[]; error?: string } {
  const { items, error } = useResources(
    kind,
    {},
    initial as unknown as Resource<K>[],
  );
  return { items: items as unknown as T[], ...(error && { error }) };
}
