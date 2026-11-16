"use server";

import {
  claimChangeSchema,
  claimDeleteSchema,
  runBackupChange,
  runChange,
  runPreview,
} from "@/lib/write/run-change";
import type { Change, Preview, WriteResult } from "@/lib/write/types";

/** previewClaimChange is the dry run of a change of one claim. */
export async function previewClaimChange(change: Change): Promise<Preview> {
  return runPreview(change, claimChangeSchema);
}

/** applyClaimChange writes a change of one claim with the writer of the settings. */
export async function applyClaimChange(change: Change): Promise<WriteResult> {
  return runChange(change, claimChangeSchema);
}

/** previewClaimDelete is the dry run of the delete of one claim. */
export async function previewClaimDelete(change: Change): Promise<Preview> {
  return runPreview(change, claimDeleteSchema);
}

/** applyClaimDelete deletes one claim with the writer of the settings. */
export async function applyClaimDelete(change: Change): Promise<WriteResult> {
  return runChange(change, claimDeleteSchema);
}

/**
 * applyBackupChange writes one new backup of cluster, or one delete of a
 * backup in its namespace, straight to the cluster.
 */
export async function applyBackupChange(
  cluster: { namespace: string; name: string },
  change: Change,
): Promise<WriteResult> {
  return runBackupChange(cluster, change);
}
