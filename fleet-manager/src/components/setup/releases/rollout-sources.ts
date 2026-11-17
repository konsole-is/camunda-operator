"use client";

import type { ReadError } from "@/lib/k8s/unread";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { LogicalBackupElasticsearch } from "@/generated/crds/logicalbackupelasticsearch";
import type { LogicalBackupRDBMS } from "@/generated/crds/logicalbackuprdbms";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { useWatchesFresh } from "@/lib/k8s/hooks";
import { ROLLOUT_SOURCE_KINDS } from "./source-kinds";
import { useLive } from "./sources";

/**
 * RolloutLists holds the lists, in all namespaces, that a move of clusters
 * to a release checks and that the other release pages do not read.
 */
export type RolloutLists = {
  clusterPresets: CamundaClusterPreset[];
  sections: PluginSection[];
  registrations: PluginRegistration[];
  requests: PluginRequest[];
  elasticsearchBackups: LogicalBackupElasticsearch[];
  rdbmsBackups: LogicalBackupRDBMS[];
};

/**
 * useRolloutSources follows each list of initial live, with errors and
 * stale as useReleaseSources gives them.
 */
export function useRolloutSources(
  initial: RolloutLists,
): RolloutLists & { errors: ReadError[]; stale: string[] } {
  const results = {
    clusterPresets: useLive("CamundaClusterPreset", initial.clusterPresets),
    sections: useLive("PluginSection", initial.sections),
    registrations: useLive("PluginRegistration", initial.registrations),
    requests: useLive("PluginRequest", initial.requests),
    elasticsearchBackups: useLive(
      "LogicalBackupElasticsearch",
      initial.elasticsearchBackups,
    ),
    rdbmsBackups: useLive("LogicalBackupRDBMS", initial.rdbmsBackups),
  };
  const keys = Object.keys(results) as (keyof RolloutLists)[];
  const kinds = keys.map((key) => ROLLOUT_SOURCE_KINDS[key]);
  const fresh = useWatchesFresh(kinds.map((kind) => ({ kind, opts: {} })));
  const errors = keys.flatMap((key) => {
    const error = results[key].error;
    return error ? [{ kind: ROLLOUT_SOURCE_KINDS[key], message: error }] : [];
  });
  return {
    clusterPresets: results.clusterPresets.items,
    sections: results.sections.items,
    registrations: results.registrations.items,
    requests: results.requests.items,
    elasticsearchBackups: results.elasticsearchBackups.items,
    rdbmsBackups: results.rdbmsBackups.items,
    errors,
    stale: kinds.filter((_, i) => !fresh[i]),
  };
}
