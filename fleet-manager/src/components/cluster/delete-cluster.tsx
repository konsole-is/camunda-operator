"use client";

import { type ReactNode, useState } from "react";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import type { ReadError } from "@/components/planes/planes-table";
import { unreadKinds } from "@/components/setup/releases/rows";
import {
  type ClaimDeleteActions,
  useClaimDelete,
} from "@/components/fleet/use-claim-delete";
import { useResource, useResources, useWatchesFresh } from "@/lib/k8s/hooks";
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
  const requestsOf = { namespace, labelSelector: `${CLAIM_LABEL}=${name}` };
  const cluster = useResource(
    "CamundaCluster",
    { namespace, name },
    initial.cluster,
  );
  const requests = useResources("PluginRequest", requestsOf, initial.requests);
  const optimizes = useResources(
    "CamundaOptimize",
    { namespace },
    initial.optimizes,
  );
  const schedules = useResources(
    "BackupSchedule",
    { namespace },
    initial.schedules,
  );
  const watched = [
    { kind: "CamundaCluster", opts: { namespace }, name, ...cluster },
    { kind: "PluginRequest", opts: requestsOf, ...requests },
    { kind: "CamundaOptimize", opts: { namespace }, ...optimizes },
    { kind: "BackupSchedule", opts: { namespace }, ...schedules },
  ] as const;
  const fresh = useWatchesFresh(watched);
  // A current snapshot of a watch replaces the list that the server could
  // not read.
  const replaced = watched
    .filter((w, i) => fresh[i] && !w.error)
    .map((w): string => w.kind);
  const errors: ReadError[] = [
    ...initial.errors.filter((e) => !replaced.includes(e.kind)),
    ...watched.flatMap((w) =>
      w.error ? [{ kind: w.kind, message: w.error }] : [],
    ),
    ...run.reads.errors,
  ];
  const unread = unreadKinds({
    errors,
    stale: [
      ...watched.filter((_, i) => !fresh[i]).map((w) => w.kind),
      ...run.reads.stale,
    ],
  });

  const model = deleteClusterModel(
    {
      ...initial,
      claim: run.claim ?? initial.claim,
      cluster: cluster.item,
      requests: requests.items,
      optimizes: optimizes.items,
      schedules: schedules.items,
    },
    picked,
    unread,
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
      progress: deleteProgress(
        targets ?? model.targets,
        {
          CamundaCluster: cluster.item ? [cluster.item] : [],
          CamundaOptimize: optimizes.items,
          BackupSchedule: schedules.items,
          PluginRequest: requests.items,
          CamundaClusterClaim: run.claim ? [run.claim] : [],
        },
        unread,
      ),
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
        errors={errors}
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
