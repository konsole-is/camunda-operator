"use server";

import {
  adminChangeSchema,
  runChange,
  runPreview,
} from "@/lib/write/run-change";
import type { Change, Preview, WriteResult } from "@/lib/write/types";

const presetSchema = adminChangeSchema([
  "CamundaClusterPreset",
  "ElasticsearchClusterPreset",
  "DatabaseServerPreset",
]);

/** previewPresetChange is the dry run of a change of presets of the three kinds. */
export async function previewPresetChange(change: Change): Promise<Preview> {
  return runPreview(change, presetSchema);
}

/** applyPresetChange writes a change of presets of the three kinds with the writer of the settings. */
export async function applyPresetChange(change: Change): Promise<WriteResult> {
  return runChange(change, presetSchema);
}
