"use server";

import {
  createChangeSchema,
  createClaim,
  runPreview,
} from "@/lib/write/run-change";
import type { Change, Preview, WriteResult } from "@/lib/write/types";

/** previewClusterCreate is the dry run of the create change of a cluster. */
export async function previewClusterCreate(change: Change): Promise<Preview> {
  return runPreview(change, createChangeSchema);
}

/** createCluster writes the create change of a cluster with the writer of the settings. */
export async function createCluster(change: Change): Promise<WriteResult> {
  return createClaim(change);
}
