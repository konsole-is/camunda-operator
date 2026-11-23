"use client";

import { type ReactNode, useState } from "react";
import {
  useDeleteReads,
  useDeleteStage,
} from "@/components/fleet/claim-delete-page";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import {
  type ClaimDeleteActions,
  useClaimDelete,
} from "@/components/fleet/use-claim-delete";
import { useResource, useResources } from "@/lib/k8s/hooks";
import type { WriteMode } from "@/lib/write/types";
import {
  clusterDeletionChoice,
  type DeleteClusterData,
  deleteClusterModel,
  initialPolicy,
} from "./delete-cluster-model";
import { DeleteClusterView } from "./delete-cluster-view";

/**
 * DeleteCluster is the delete page of the cluster of initial.claim. It
 * follows each object and list of initial live, so a list that the server
 * could not read comes back with its watch. It opens on the pick of
 * initialPolicy, and writes only through useClaimDelete on the confirm of
 * the review. Once the delete is sent, the page lists each object that
 * always goes until it is gone. writeMode is undefined when the server
 * could not read it. It needs a QueryClientProvider and a
 * GitPushesProvider above it.
 */
export function DeleteCluster({
  initial,
  writeMode,
  actions,
}: {
  initial: DeleteClusterData;
  writeMode: WriteMode | undefined;
  actions: ClaimDeleteActions;
}): ReactNode {
  const namespace = initial.claim.metadata?.namespace ?? "";
  const name = initial.claim.metadata?.name ?? "";
  const { toast, show, dismiss } = useWriteToast();
  const all = {};
  const [picked, setPicked] = useState(() =>
    initialPolicy(
      clusterDeletionChoice(
        initial,
        initial.errors.map((e) => e.kind),
        writeMode === "direct",
      ),
    ),
  );
  const registrations = useResources(
    "PluginRegistration",
    all,
    initial.registrations,
  );
  const run = useClaimDelete({
    initial: initial.claim,
    policy: picked,
    registrations: registrations.items,
    mode: writeMode,
    intent: `Delete the cluster ${name}`,
    pushedMessage: `Pushed the delete of ${name}.`,
    actions,
    show,
  });
  const inNamespace = { namespace };
  const cluster = useResource(
    "CamundaCluster",
    { namespace, name },
    initial.cluster,
  );
  const namespaceObject = useResource(
    "Namespace",
    { name: namespace },
    initial.namespace,
  );
  const requests = useResources("PluginRequest", inNamespace, initial.requests);
  const optimizes = useResources(
    "CamundaOptimize",
    inNamespace,
    initial.optimizes,
  );
  const schedules = useResources(
    "BackupSchedule",
    inNamespace,
    initial.schedules,
  );
  const environments = useResources("Environment", all, initial.environments);
  const sections = useResources("PluginSection", all, initial.sections);
  const presets = useResources("CamundaClusterPreset", all, initial.presets);
  const releases = useResources("CamundaRelease", all, initial.releases);
  const backupsES = useResources(
    "LogicalBackupElasticsearch",
    inNamespace,
    initial.backups.elasticsearch,
  );
  const backupsRDBMS = useResources(
    "LogicalBackupRDBMS",
    inNamespace,
    initial.backups.rdbms,
  );
  const watched = [
    { kind: "CamundaCluster", opts: { namespace, name }, ...cluster },
    { kind: "Namespace", opts: { name: namespace }, ...namespaceObject },
    { kind: "PluginRequest", opts: inNamespace, ...requests },
    { kind: "CamundaOptimize", opts: inNamespace, ...optimizes },
    { kind: "BackupSchedule", opts: inNamespace, ...schedules },
    { kind: "Environment", opts: all, ...environments },
    { kind: "PluginSection", opts: all, ...sections },
    { kind: "PluginRegistration", opts: all, ...registrations },
    { kind: "CamundaClusterPreset", opts: all, ...presets },
    { kind: "CamundaRelease", opts: all, ...releases },
    { kind: "LogicalBackupElasticsearch", opts: inNamespace, ...backupsES },
    { kind: "LogicalBackupRDBMS", opts: inNamespace, ...backupsRDBMS },
  ] as const;
  const { errors, unread } = useDeleteReads(watched, initial.errors, run.reads);

  const model = deleteClusterModel(
    {
      claim: run.claim ?? initial.claim,
      cluster: cluster.item,
      namespace: namespaceObject.item,
      environments: environments.items,
      sections: sections.items,
      registrations: registrations.items,
      requests: requests.items,
      optimizes: optimizes.items,
      schedules: schedules.items,
      backups: { elasticsearch: backupsES.items, rdbms: backupsRDBMS.items },
      presets: presets.items,
      releases: releases.items,
      errors,
    },
    picked,
    unread,
    writeMode === "direct",
  );
  const stage = useDeleteStage(
    run,
    model.targets,
    {
      CamundaCluster: cluster.item ? [cluster.item] : [],
      CamundaOptimize: optimizes.items,
      BackupSchedule: schedules.items,
      PluginRequest: requests.items,
      CamundaClusterClaim: run.claim ? [run.claim] : [],
    },
    unread,
  );

  return (
    <>
      <DeleteClusterView
        model={model}
        mode={writeMode}
        stage={stage}
        errors={errors}
        remove={run.remove}
        policyStep={run.policyStep}
        keepRefused={run.keepRefused}
        policyOwned={run.policyOwned}
        onPick={setPicked}
        preview={run.preview}
        apply={run.apply}
        show={show}
        onWritten={run.onWritten}
        onStopWaiting={run.stopWaiting}
      />
      {toast && (
        <div className="sticky bottom-4 z-30 mt-6 flex justify-end">
          <Toast result={toast} onDismiss={dismiss} />
        </div>
      )}
    </>
  );
}
