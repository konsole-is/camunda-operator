"use server";

import type { V1Namespace } from "@kubernetes/client-node";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import {
  deleteRefusalText,
  type PresetKind,
  presetDeleteChecks,
} from "@/lib/usage";
import {
  adminChangeSchema,
  runChange,
  runPreview,
} from "@/lib/write/run-change";
import type { ApiError, Change, Preview, WriteResult } from "@/lib/write/types";
import { loadPresets } from "./load";

const presetSchema = adminChangeSchema([
  "CamundaClusterPreset",
  "ElasticsearchClusterPreset",
  "DatabaseServerPreset",
]);

/** previewPresetChange is the dry run of a change of presets of the three kinds, under the rules of applyPresetChange. */
export async function previewPresetChange(change: Change): Promise<Preview> {
  const errors = await deleteRefusals(change);
  if (errors.length > 0) return { objects: [], errors };
  return runPreview(change, presetSchema);
}

/**
 * applyPresetChange writes a change of presets of the three kinds with the
 * writer of the settings. Before a delete, it reads the lists of the preset
 * pages from the cache of the server. It refuses with Forbidden the whole
 * change when presetDeleteChecks blocks the delete of one of its presets:
 * a preset in use or allowed in an environment, with each user named, or a
 * preset whose users cannot be read.
 */
export async function applyPresetChange(change: Change): Promise<WriteResult> {
  const errors = await deleteRefusals(change);
  if (errors.length > 0) return { ok: false, errors };
  return runChange(change, presetSchema);
}

// deleteRefusals returns no errors for a change that the schema refuses, so
// that the writer gives its Invalid errors.
async function deleteRefusals(input: Change): Promise<ApiError[]> {
  const parsed = presetSchema.safeParse(input);
  const deletes = parsed.success ? (parsed.data.delete ?? []) : [];
  if (deletes.length === 0) return [];
  const sources = await loadPresets();
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which presetDeleteChecks does not read.
  const checkOf = presetDeleteChecks(
    {
      ...sources,
      environments: sources.environments as unknown as Environment[],
      namespaces: sources.namespaces as unknown as V1Namespace[],
      registrations: sources.registrations as unknown as PluginRegistration[],
    },
    sources.readErrors.map((e) => e.kind),
  );
  return deletes.flatMap((ref): ApiError[] => {
    const { block } = checkOf(ref.kind as PresetKind, ref.name);
    return block
      ? [{ code: "Forbidden", message: deleteRefusalText(block), ref }]
      : [];
  });
}
