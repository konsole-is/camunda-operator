"use client";

import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { LogicalBackupElasticsearch } from "@/generated/crds/logicalbackupelasticsearch";
import type { LogicalBackupRDBMS } from "@/generated/crds/logicalbackuprdbms";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { useAnswered } from "@/lib/k8s/answered";
import { useWatchesFresh } from "@/lib/k8s/hooks";
import { useLive } from "@/lib/k8s/live";
import { currentReadErrors, type ReadError } from "@/lib/k8s/unread";
import { ROLLOUT_SOURCE_KINDS } from "./source-kinds";

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
 * RolloutSnapshot is the first snapshot of the lists of the Rollout tab. readErrors holds the error of
 * each kind that the server could not read.
 */
export type RolloutSnapshot = RolloutLists & { readErrors?: ReadError[] };

/**
 * useRolloutSources follows each list of initial live, with errors and
 * stale as useReleaseSources gives them.
 */
export function useRolloutSources(
  initial: RolloutSnapshot,
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
  const answered = useAnswered(kinds);
  const live = keys.flatMap((key) => {
    const error = results[key].error;
    return error ? [{ kind: ROLLOUT_SOURCE_KINDS[key], message: error }] : [];
  });
  const errors = currentReadErrors(live, initial.readErrors, answered);
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
