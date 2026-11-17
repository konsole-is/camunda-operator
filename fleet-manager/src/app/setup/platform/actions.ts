"use server";

import { PLATFORM_SOURCE_KINDS } from "@/components/setup/platform/source-kinds";
import type { PlatformSources } from "@/components/setup/platform/sources";
import { listEach } from "@/lib/k8s/list-each";
import { deleteRefusalText, platformDeleteBlock } from "@/lib/usage";
import {
  adminChangeSchema,
  runChange,
  runPreview,
} from "@/lib/write/run-change";
import type { ApiError, Change, Preview, WriteResult } from "@/lib/write/types";

const platformConfigSchema = adminChangeSchema(["CamundaPlatformConfig"]);

/** previewPlatformConfigChange is the dry run of an update or a delete of CamundaPlatformConfigs, under the rules of applyPlatformConfigChange. */
export async function previewPlatformConfigChange(
  change: Change,
): Promise<Preview> {
  const errors = await deleteRefusals(change);
  if (errors.length > 0) return { objects: [], errors };
  return runPreview(change, platformConfigSchema);
}

/**
 * applyPlatformConfigChange writes an update or a delete of
 * CamundaPlatformConfigs with the writer of the settings. Before a delete,
 * it reads the lists of the Platform pages from the cache of the server. It
 * refuses with Forbidden the whole change when platformDeleteBlock blocks
 * the delete of one of its configs: a config that a resource, a claim, or
 * an environment uses, with each user named, or a config whose users
 * cannot be read.
 */
export async function applyPlatformConfigChange(
  change: Change,
): Promise<WriteResult> {
  const errors = await deleteRefusals(change);
  if (errors.length > 0) return { ok: false, errors };
  return runChange(change, platformConfigSchema);
}

// deleteRefusals returns no errors for a change that the schema refuses, so
// that the writer gives its Invalid errors.
async function deleteRefusals(input: Change): Promise<ApiError[]> {
  const parsed = platformConfigSchema.safeParse(input);
  const deletes = parsed.success ? (parsed.data.delete ?? []) : [];
  if (deletes.length === 0) return [];
  const { lists, readErrors } = await listEach(PLATFORM_SOURCE_KINDS);
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which platformDeleteBlock does not read.
  const sources = lists as unknown as PlatformSources;
  const unread = readErrors.map((e) => e.kind);
  return deletes.flatMap((ref): ApiError[] => {
    const block = platformDeleteBlock(ref.name, sources, unread);
    return block
      ? [{ code: "Forbidden", message: deleteRefusalText(block), ref }]
      : [];
  });
}
