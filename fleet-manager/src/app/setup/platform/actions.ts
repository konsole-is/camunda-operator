"use server";

import { PLATFORM_SOURCE_KINDS } from "@/components/setup/platform/source-kinds";
import type { PlatformSources } from "@/components/setup/platform/sources";
import { PLATFORM_USER_KINDS, platformDeleteBlock } from "@/lib/usage";
import { type DeleteRule, deleteRefusals } from "@/lib/write/delete-refusals";
import {
  adminChangeSchema,
  runChange,
  runPreview,
} from "@/lib/write/run-change";
import type { Change, Preview, WriteResult } from "@/lib/write/types";

const platformConfigSchema = adminChangeSchema(["CamundaPlatformConfig"]);

/** previewPlatformConfigChange is the dry run of an update or a delete of CamundaPlatformConfigs, under the rules of applyPlatformConfigChange. */
export async function previewPlatformConfigChange(
  change: Change,
): Promise<Preview> {
  const errors = await deleteRefusals(
    change,
    platformConfigSchema,
    platformRule,
  );
  if (errors.length > 0) return { objects: [], errors };
  return runPreview(change, platformConfigSchema);
}

/**
 * applyPlatformConfigChange writes an update or a delete of
 * CamundaPlatformConfigs with the writer of the settings. It refuses with
 * Forbidden the whole change when platformDeleteBlock blocks the delete of
 * one of its configs: a config that a resource, a claim, or an environment
 * uses, with each user named, or a config whose users cannot be read.
 */
export async function applyPlatformConfigChange(
  change: Change,
): Promise<WriteResult> {
  const errors = await deleteRefusals(
    change,
    platformConfigSchema,
    platformRule,
  );
  if (errors.length > 0) return { ok: false, errors };
  return runChange(change, platformConfigSchema);
}

const platformRule: DeleteRule<typeof PLATFORM_SOURCE_KINDS> = {
  kinds: PLATFORM_SOURCE_KINDS,
  needs: () => PLATFORM_USER_KINDS,
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which platformDeleteBlock does not read.
  blockOf: (lists, unread) => (ref) =>
    platformDeleteBlock(ref.name, lists as unknown as PlatformSources, unread),
};
