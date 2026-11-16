"use server";

import {
  adminChangeSchema,
  claimBatchChangeSchema,
  runChange,
  runPreview,
} from "@/lib/write/run-change";
import type { Change, Preview, WriteResult } from "@/lib/write/types";

const releaseSchema = adminChangeSchema(["CamundaRelease"]);

/** previewReleaseChange is the dry run of a create, an update, or a delete of CamundaReleases. */
export async function previewReleaseChange(change: Change): Promise<Preview> {
  return runPreview(change, releaseSchema);
}

/** applyReleaseChange writes a create, an update, or a delete of CamundaReleases with the writer of the settings. */
export async function applyReleaseChange(change: Change): Promise<WriteResult> {
  return runChange(change, releaseSchema);
}

/** previewReleaseMove is the dry run of a move of cluster claims to a release. */
export async function previewReleaseMove(change: Change): Promise<Preview> {
  return runPreview(change, claimBatchChangeSchema);
}

/** applyReleaseMove writes a move of cluster claims to a release with the writer of the settings, as one change. */
export async function applyReleaseMove(change: Change): Promise<WriteResult> {
  return runChange(change, claimBatchChangeSchema);
}
