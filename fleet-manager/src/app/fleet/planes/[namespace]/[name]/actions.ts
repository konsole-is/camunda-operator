"use server";

import {
  claimChangeSchema,
  previewClaimDelete,
  runChange,
  runClaimDelete,
  runPreview,
} from "@/lib/write/run-change";
import type { Change, Preview, WriteResult } from "@/lib/write/types";

/** previewPlaneClaimChange is the dry run of a change of one plane claim. */
export async function previewPlaneClaimChange(
  change: Change,
): Promise<Preview> {
  return runPreview(change, claimChangeSchema);
}

/** applyPlaneClaimChange writes a change of one plane claim with the writer of the settings. */
export async function applyPlaneClaimChange(
  change: Change,
): Promise<WriteResult> {
  return runChange(change, claimChangeSchema);
}

/** previewPlaneClaimDelete is the dry run of the delete of one plane claim, as previewClaimDelete of run-change gives it. */
export async function previewPlaneClaimDelete(
  change: Change,
): Promise<Preview> {
  return previewClaimDelete(change);
}

/** applyPlaneClaimDelete deletes one plane claim with runClaimDelete. */
export async function applyPlaneClaimDelete(
  change: Change,
): Promise<WriteResult> {
  return runClaimDelete(change);
}
