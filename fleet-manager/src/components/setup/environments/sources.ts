"use client";

import type { V1Namespace } from "@kubernetes/client-node";
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
import { useAnswered } from "@/lib/k8s/answered";
import { useWatchesFresh } from "@/lib/k8s/hooks";
import { useLive } from "@/lib/k8s/live";
import { currentReadErrors, type ReadError } from "@/lib/k8s/unread";
import { SOURCE_KINDS } from "./source-kinds";

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

/**
 * EnvironmentSnapshot is the first snapshot of the Environments pages.
 * readErrors holds the error of each kind that the server could not read.
 */
export type EnvironmentSnapshot = EnvironmentSources & {
  readErrors?: ReadError[];
};

/**
 * useEnvironmentSources follows each list of initial live. errors holds
 * the error of each failed watch, and the error of initial.readErrors of
 * each other kind until the watch of that kind answers. The lists then
 * keep their last items, which can be out of date. stale holds the kind of
 * each list whose watch is not fresh (freshKey).
 */
export function useEnvironmentSources(
  initial: EnvironmentSnapshot,
): EnvironmentSources & { errors: ReadError[]; stale: string[] } {
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
  const kinds = Object.values(SOURCE_KINDS);
  const fresh = useWatchesFresh(kinds.map((kind) => ({ kind, opts: {} })));
  const answered = useAnswered(kinds);
  const live = (Object.keys(results) as (keyof EnvironmentSources)[]).flatMap(
    (key) => {
      const error = results[key].error;
      return error ? [{ kind: SOURCE_KINDS[key], message: error }] : [];
    },
  );
  const errors = currentReadErrors(live, initial.readErrors, answered);
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
    stale: kinds.filter((_, i) => !fresh[i]),
  };
}
