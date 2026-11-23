"use client";

import { type ReactNode, useState } from "react";
import { initialPolicy } from "@/components/cluster/delete-cluster-model";
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
  type DeletePlaneData,
  deletePlaneModel,
  planeDeletionChoice,
} from "./delete-plane-model";
import { DeletePlaneView } from "./delete-plane-view";

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
    initialPolicy(
      planeDeletionChoice(
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
  const { errors, unread } = useDeleteReads(watched, initial.errors, run.reads);

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
    writeMode === "direct",
  );
  const stage = useDeleteStage(
    run,
    model.targets,
    {
      CamundaManagementCluster: plane.item ? [plane.item] : [],
      PluginRequest: requests.items,
      CamundaManagementClusterClaim: run.claim ? [run.claim] : [],
    },
    unread,
  );

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
