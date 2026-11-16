"use client";

import { useState } from "react";
import type { ReadError } from "@/components/planes/planes-table";
import { useResources, useWatchesFresh } from "@/lib/k8s/hooks";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import type { Change, Preview, WriteResult } from "@/lib/write/types";
import { PRESET_SOURCE_KINDS } from "./source-kinds";

/** PresetSources holds every list that the preset pages read, in all namespaces. */
export type PresetSources = {
  -readonly [K in keyof typeof PRESET_SOURCE_KINDS]: Resource<
    (typeof PRESET_SOURCE_KINDS)[K]
  >[];
};

/** PresetSnapshot is the first read of the preset pages on the server: the lists, and the error of each list that could not be read. */
export type PresetSnapshot = PresetSources & { readErrors: ReadError[] };

/** PresetActions are the writes of the preset pages: the server actions on the page, fakes in tests and stories. */
export type PresetActions = {
  previewChange: (change: Change) => Promise<Preview>;
  applyChange: (change: Change) => Promise<WriteResult>;
};

/**
 * usePresetSources follows each list of initial live. errors holds each
 * list that is not current: a failed watch, a failed first read until the
 * watch sends a list, or a watch that reconnects. Its items can be stale.
 */
export function usePresetSources(
  initial: PresetSnapshot,
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
  const kinds = keys.map((key) => PRESET_SOURCE_KINDS[key]);
  const fresh = useWatchesFresh(kinds.map((kind) => ({ kind, opts: {} })));
  const answered = useAnswered(kinds, fresh);
  const errors = keys.flatMap((key, i): ReadError[] => {
    const kind = kinds[i];
    const error = lists[key].error;
    if (error) return [{ kind, message: error }];
    if (!answered.has(kind))
      return initial.readErrors.filter((e) => e.kind === kind);
    return fresh[i] ? [] : [{ kind, message: RECONNECTING }];
  });
  const items = Object.fromEntries(
    keys.map((key) => [key, lists[key].items]),
  ) as PresetSources;
  return { ...items, errors };
}

const RECONNECTING = "the list is out of date until the watch reconnects";

// useAnswered gives the kinds whose watch sent a list at least once. Until
// then the read of the server stands, with its error.
function useAnswered(
  kinds: readonly KindName[],
  fresh: readonly boolean[],
): ReadonlySet<string> {
  const [answered, setAnswered] = useState<ReadonlySet<string>>(new Set());
  const added = kinds.filter((kind, i) => fresh[i] && !answered.has(kind));
  if (added.length > 0) setAnswered(new Set([...answered, ...added]));
  return answered;
}

function useLive<K extends KindName>(
  kind: K,
  initial: Resource<K>[],
): { items: Resource<K>[]; error?: string } {
  return useResources(kind, {}, initial);
}
