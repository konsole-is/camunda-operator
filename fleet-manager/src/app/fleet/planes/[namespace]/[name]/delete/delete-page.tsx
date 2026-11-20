"use client";

import { DeletePlane } from "@/components/planes/delete-plane";
import type { DeletePlaneData } from "@/components/planes/delete-plane-model";
import type { WriteMode } from "@/lib/write/types";
import {
  applyPlaneClaimChange,
  applyPlaneClaimDelete,
  previewPlaneClaimDelete,
} from "../actions";

const actions = {
  preview: previewPlaneClaimDelete,
  remove: applyPlaneClaimDelete,
  applyPolicy: applyPlaneClaimChange,
};

/**
 * DeletePage is the client half of the delete page of a management plane.
 * It gives DeletePlane the server actions of the plane route. writeMode is
 * undefined when the server could not read it.
 */
export function DeletePage({
  initial,
  writeMode,
}: {
  initial: DeletePlaneData;
  writeMode: WriteMode | undefined;
}) {
  return (
    <DeletePlane initial={initial} writeMode={writeMode} actions={actions} />
  );
}
