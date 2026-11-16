"use server";

import {
  claimChangeSchema,
  runChange,
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
