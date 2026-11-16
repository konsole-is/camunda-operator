"use client";

import { type ReactNode, useState } from "react";
import {
  deleteProgress,
  type DeleteTarget,
  initialPolicy,
  mergeTargets,
} from "@/components/cluster/delete-cluster-model";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import {
  type ClaimDeleteActions,
  useClaimDelete,
} from "@/components/fleet/use-claim-delete";
import { unreadKinds } from "@/components/setup/releases/rows";
import { useResource, useResources, useWatchesFresh } from "@/lib/k8s/hooks";
import type { WriteMode } from "@/lib/write/types";
import {
  type DeletePlaneData,
  deletePlaneModel,
  planeDeletionChoice,
} from "./delete-plane-model";
import { type DeletePlaneStage, DeletePlaneView } from "./delete-plane-view";
import type { ReadError } from "./planes-table";

/**
 * DeletePlane is the delete page of the management plane of initial.claim.
 * It follows each object and list of initial live, so a list that the
 * server could not read comes back with its watch. It opens on the pick of
 * initialPolicy, and writes only through useClaimDelete on the confirm of
 * the review. Once the delete is sent, the page lists each object that
 * always goes until it is gone. writeMode is undefined when the server
 * could not read it. It needs a QueryClientProvider and a
 * GitPushesProvider above it.
 */
export function DeletePlane({
  initial,
  writeMode,
  actions,
}: {
  initial: DeletePlaneData;
  writeMode: WriteMode | undefined;
  actions: ClaimDeleteActions;
}): ReactNode {
  const namespace = initial.claim.metadata?.namespace ?? "";
  const name = initial.claim.metadata?.name ?? "";
  const { toast, show, dismiss } = useWriteToast();
  const all = {};
  const [picked, setPicked] = useState(() =>
    initialPolicy(planeDeletionChoice(initial)),
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
    intent: `Delete the management plane ${name}`,
    pushedMessage: `Pushed the delete of ${name}.`,
    actions,
    show,
  });
  const inNamespace = { namespace };
  const plane = useResource(
    "CamundaManagementCluster",
    { namespace, name },
    initial.plane,
  );
  const namespaceObject = useResource(
    "Namespace",
    { name: namespace },
    initial.namespace,
  );
  const requests = useResources("PluginRequest", inNamespace, initial.requests);
  const optimizes = useResources("CamundaOptimize", all, initial.optimizes);
  const environments = useResources("Environment", all, initial.environments);
  const sections = useResources("PluginSection", all, initial.sections);
  const watched = [
    { kind: "CamundaManagementCluster", opts: { namespace, name }, ...plane },
    { kind: "Namespace", opts: { name: namespace }, ...namespaceObject },
    { kind: "PluginRequest", opts: inNamespace, ...requests },
    { kind: "CamundaOptimize", opts: all, ...optimizes },
    { kind: "Environment", opts: all, ...environments },
    { kind: "PluginSection", opts: all, ...sections },
    { kind: "PluginRegistration", opts: all, ...registrations },
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

  const model = deletePlaneModel(
    {
      claim: run.claim ?? initial.claim,
      plane: plane.item,
      namespace: namespaceObject.item,
      environments: environments.items,
      sections: sections.items,
      registrations: registrations.items,
      requests: requests.items,
      optimizes: optimizes.items,
      errors,
    },
    picked,
    unread,
  );
  // A target stays once the delete started, so that an object that is gone
  // shows as Deleted instead of leaving the list.
  const [targets, setTargets] = useState<DeleteTarget[]>();
  if (run.stage.kind === "deleting") {
    const merged = mergeTargets(targets ?? [], model.targets);
    if (JSON.stringify(merged) !== JSON.stringify(targets)) setTargets(merged);
  }

  let stage: DeletePlaneStage;
  if (run.stage.kind === "deleting") {
    stage = {
      kind: "deleting",
      progress: deleteProgress(
        mergeTargets(targets ?? [], model.targets),
        {
          CamundaManagementCluster: plane.item ? [plane.item] : [],
          PluginRequest: requests.items,
          CamundaManagementClusterClaim: run.claim ? [run.claim] : [],
        },
        unread,
      ),
      gitWaiting: run.stage.gitWaiting,
      ...(run.claim && { claim: run.claim }),
      ...(run.stage.keepRefused && { keepRefused: run.stage.keepRefused }),
    };
  } else stage = run.stage;

  return (
    <>
      <DeletePlaneView
        model={model}
        mode={writeMode}
        stage={stage}
        errors={errors}
        remove={run.remove}
        policyStep={run.policyStep}
        keepRefused={run.keepRefused}
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
