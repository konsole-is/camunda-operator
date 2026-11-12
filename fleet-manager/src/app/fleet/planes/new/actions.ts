"use server";

import {
  createChangeSchema,
  createClaim,
  runPreview,
} from "@/lib/write/run-change";
import type { Change, Preview, WriteResult } from "@/lib/write/types";

/** previewPlaneCreate is the dry run of the create change of a plane. */
export async function previewPlaneCreate(change: Change): Promise<Preview> {
  return runPreview(change, createChangeSchema);
}

/** createPlane writes the create change of a plane with the writer of the settings. */
export async function createPlane(change: Change): Promise<WriteResult> {
  return createClaim(change);
}
