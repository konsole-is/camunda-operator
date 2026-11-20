"use server";

import {
  claimChangeSchema,
  previewClaimDelete as previewDelete,
  runBackupChange,
  runChange,
  runClaimDelete,
  runPreview,
} from "@/lib/write/run-change";
import type { Change, Preview, WriteResult } from "@/lib/write/types";

/** previewClaimChange is the dry run of a change of one claim. */
export async function previewClaimChange(change: Change): Promise<Preview> {
  return runPreview(change, claimChangeSchema("CamundaClusterClaim"));
}

/** applyClaimChange writes a change of one claim with the writer of the settings. */
export async function applyClaimChange(change: Change): Promise<WriteResult> {
  return runChange(change, claimChangeSchema("CamundaClusterClaim"));
}

/** previewClaimDelete is the dry run of the delete of one cluster claim, as previewClaimDelete of run-change gives it. */
export async function previewClaimDelete(change: Change): Promise<Preview> {
  return previewDelete(change, "CamundaClusterClaim");
}

/** applyClaimDelete deletes one cluster claim with runClaimDelete. */
export async function applyClaimDelete(change: Change): Promise<WriteResult> {
  return runClaimDelete(change, "CamundaClusterClaim");
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
