"use server";

import {
  adminChangeSchema,
  runChange,
  runPreview,
} from "@/lib/write/run-change";
import type { Change, Preview, WriteResult } from "@/lib/write/types";

const platformConfigSchema = adminChangeSchema(["CamundaPlatformConfig"]);

/** previewPlatformConfigChange is the dry run of an update or a delete of CamundaPlatformConfigs. */
export async function previewPlatformConfigChange(
  change: Change,
): Promise<Preview> {
  return runPreview(change, platformConfigSchema);
}

/** applyPlatformConfigChange writes an update or a delete of CamundaPlatformConfigs with the writer of the settings. */
export async function applyPlatformConfigChange(
  change: Change,
): Promise<WriteResult> {
  return runChange(change, platformConfigSchema);
}
