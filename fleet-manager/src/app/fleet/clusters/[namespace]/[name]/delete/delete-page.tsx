"use client";

import { DeleteCluster } from "@/components/cluster/delete-cluster";
import type { DeleteClusterData } from "@/components/cluster/delete-cluster-model";
import type { WriteMode } from "@/lib/write/types";
import {
  applyClaimChange,
  applyClaimDelete,
  previewClaimDelete,
} from "../actions";

const actions = {
  preview: previewClaimDelete,
  remove: applyClaimDelete,
  applyPolicy: applyClaimChange,
};

/**
 * DeletePage is the client half of the delete page of a cluster. It gives
 * DeleteCluster the server actions of the cluster route. writeMode is
 * undefined when the server could not read it.
 */
export function DeletePage({
  initial,
  writeMode,
}: {
  initial: DeleteClusterData;
  writeMode: WriteMode | undefined;
}) {
  return (
    <DeleteCluster initial={initial} writeMode={writeMode} actions={actions} />
  );
}
