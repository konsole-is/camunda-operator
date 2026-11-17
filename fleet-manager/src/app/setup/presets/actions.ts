"use server";

import type { V1Namespace } from "@kubernetes/client-node";
import { PRESET_SOURCE_KINDS } from "@/components/setup/presets/source-kinds";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import {
  PRESET_DELETE_KINDS,
  type PresetKind,
  presetDeleteChecks,
} from "@/lib/usage";
import { type DeleteRule, deleteRefusals } from "@/lib/write/delete-refusals";
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

/** previewPresetChange is the dry run of a change of presets of the three kinds, under the rules of applyPresetChange. */
export async function previewPresetChange(change: Change): Promise<Preview> {
  const errors = await deleteRefusals(change, presetSchema, presetRule);
  if (errors.length > 0) return { objects: [], errors };
  return runPreview(change, presetSchema);
}

/**
 * applyPresetChange writes a change of presets of the three kinds with the
 * writer of the settings. It refuses with Forbidden the whole change when
 * presetDeleteChecks blocks the delete of one of its presets: a preset in
 * use or allowed in an environment, with each user named, or a preset whose
 * users cannot be read.
 */
export async function applyPresetChange(change: Change): Promise<WriteResult> {
  const errors = await deleteRefusals(change, presetSchema, presetRule);
  if (errors.length > 0) return { ok: false, errors };
  return runChange(change, presetSchema);
}

const presetRule: DeleteRule<typeof PRESET_SOURCE_KINDS> = {
  kinds: PRESET_SOURCE_KINDS,
  needs: (ref) => PRESET_DELETE_KINDS[ref.kind as PresetKind],
  blockOf: (lists, unread) => {
    // A Resource is plain JSON, so it differs from its model type only in its
    // timestamps, which presetDeleteChecks does not read.
    const checkOf = presetDeleteChecks(
      {
        ...lists,
        environments: lists.environments as unknown as Environment[],
        namespaces: lists.namespaces as unknown as V1Namespace[],
        registrations: lists.registrations as unknown as PluginRegistration[],
      },
      unread,
    );
    return (ref) => checkOf(ref.kind as PresetKind, ref.name).block;
  },
};
