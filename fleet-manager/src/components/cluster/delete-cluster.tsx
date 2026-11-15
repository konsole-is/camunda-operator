"use client";

import { type ReactNode, useState } from "react";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import {
  type ClaimDeleteActions,
  useClaimDelete,
} from "@/components/fleet/use-claim-delete";
import { useResource, useResources } from "@/lib/k8s/hooks";
import { CLAIM_LABEL } from "@/lib/labels";
import type { WriteMode } from "@/lib/write/types";
import {
  clusterDeletionChoice,
  type DeleteClusterData,
  deleteClusterModel,
  deleteProgress,
  type DeleteTarget,
  initialPolicy,
} from "./delete-cluster-model";
import { type DeleteStage, DeleteClusterView } from "./delete-cluster-view";

/**
 * DeleteCluster is the delete page of the cluster of initial.claim. It
 * follows the claim, the cluster, the requests of the claim, and the
 * CamundaOptimize and BackupSchedule objects of the namespace live; the
 * rest of initial stays as read. It opens on the pick of initialPolicy,
 * and writes only through useClaimDelete on the confirm of the review.
 * Once the delete is sent, the page lists each object that always goes
 * until it is gone. writeMode is undefined when the server could not read
 * it. It needs a QueryClientProvider and a GitPushesProvider above it.
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
  const [picked, setPicked] = useState(() =>
    initialPolicy(clusterDeletionChoice(initial)),
  );
  const run = useClaimDelete({
    initial: initial.claim,
    policy: picked,
    mode: writeMode,
    intent: `Delete the cluster ${name}`,
    pushedMessage: `Pushed the delete of ${name}.`,
    actions,
    show,
  });
  const cluster = useResource(
    "CamundaCluster",
    { namespace, name },
    initial.cluster,
  ).item;
  const requests = useResources(
    "PluginRequest",
    { namespace, labelSelector: `${CLAIM_LABEL}=${name}` },
    initial.requests,
  ).items;
  const optimizes = useResources(
    "CamundaOptimize",
    { namespace },
    initial.optimizes,
  ).items;
  const schedules = useResources(
    "BackupSchedule",
    { namespace },
    initial.schedules,
  ).items;

  const model = deleteClusterModel(
    {
      ...initial,
      claim: run.claim ?? initial.claim,
      cluster,
      requests,
      optimizes,
      schedules,
    },
    picked,
  );
  // The targets stay as they were when the delete started, so that an
  // object that is gone shows as Deleted instead of leaving the list.
  const [targets, setTargets] = useState<DeleteTarget[]>();
  if (run.stage.kind === "deleting" && targets === undefined)
    setTargets(model.targets);

  let stage: DeleteStage;
  if (run.stage.kind === "deleting") {
    stage = {
      kind: "deleting",
      progress: deleteProgress(targets ?? model.targets, {
        CamundaCluster: cluster ? [cluster] : [],
        CamundaOptimize: optimizes,
        BackupSchedule: schedules,
        PluginRequest: requests,
        CamundaClusterClaim: run.claim ? [run.claim] : [],
      }),
      gitWaiting: run.stage.gitWaiting,
      ...(run.claim && { claim: run.claim }),
    };
  } else stage = run.stage;

  return (
    <>
      <DeleteClusterView
        model={model}
        mode={writeMode}
        stage={stage}
        remove={run.remove}
        policyStep={run.policyStep}
        onPick={setPicked}
        preview={actions.preview}
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
