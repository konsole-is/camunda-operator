"use client";

import type { ReadError } from "@/components/planes/planes-table";
import { useResources } from "@/lib/k8s/hooks";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import type { Change, Preview, WriteResult } from "@/lib/write/types";
import { PRESET_SOURCE_KINDS } from "./source-kinds";

/** PresetSources holds every list that the preset pages read, in all namespaces. */
export type PresetSources = {
  [K in keyof typeof PRESET_SOURCE_KINDS]: Resource<
    (typeof PRESET_SOURCE_KINDS)[K]
  >[];
};

/** PresetActions are the writes of the preset pages: the server actions on the page, fakes in tests and stories. */
export type PresetActions = {
  previewChange: (change: Change) => Promise<Preview>;
  applyChange: (change: Change) => Promise<WriteResult>;
};

/**
 * usePresetSources follows each list of initial live. errors holds the
 * error of each failed watch. The lists then keep their last items, which
 * can be out of date.
 */
export function usePresetSources(
  initial: PresetSources,
): PresetSources & { errors: ReadError[] } {
  const lists = {
    clusterPresets: useLive("CamundaClusterPreset", initial.clusterPresets),
    elasticsearchPresets: useLive(
      "ElasticsearchClusterPreset",
      initial.elasticsearchPresets,
    ),
    databasePresets: useLive("DatabaseServerPreset", initial.databasePresets),
    environments: useLive("Environment", initial.environments),
    namespaces: useLive("Namespace", initial.namespaces),
    clusters: useLive("CamundaCluster", initial.clusters),
    elasticsearchClusters: useLive(
      "ElasticsearchCluster",
      initial.elasticsearchClusters,
    ),
    databaseServers: useLive("DatabaseServer", initial.databaseServers),
    clusterClaims: useLive("CamundaClusterClaim", initial.clusterClaims),
    planeClaims: useLive("CamundaManagementClusterClaim", initial.planeClaims),
    requests: useLive("PluginRequest", initial.requests),
    sections: useLive("PluginSection", initial.sections),
    registrations: useLive("PluginRegistration", initial.registrations),
  };
  const keys = Object.keys(PRESET_SOURCE_KINDS) as (keyof PresetSources)[];
  const errors = keys.flatMap((key) => {
    const error = lists[key].error;
    return error ? [{ kind: PRESET_SOURCE_KINDS[key], message: error }] : [];
  });
  const items = Object.fromEntries(
    keys.map((key) => [key, lists[key].items]),
  ) as PresetSources;
  return { ...items, errors };
}

function useLive<K extends KindName>(
  kind: K,
  initial: Resource<K>[],
): { items: Resource<K>[]; error?: string } {
  return useResources(kind, {}, initial);
}
