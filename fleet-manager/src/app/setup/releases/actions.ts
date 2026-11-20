"use server";

import { RELEASE_SOURCE_KINDS } from "@/components/setup/releases/source-kinds";
import type { ReleasesData } from "@/components/setup/releases/sources";
import { RELEASE_DELETE_KINDS, releaseDeleteBlock } from "@/lib/usage";
import { type DeleteRule, deleteRefusals } from "@/lib/write/delete-refusals";
import {
  adminChangeSchema,
  previewClaimMove,
  runChange,
  runClaimMove,
  runPreview,
} from "@/lib/write/run-change";
import type { Change, Preview, WriteResult } from "@/lib/write/types";

const releaseSchema = adminChangeSchema(["CamundaRelease"]);

/** previewReleaseChange is the dry run of a create, an update, or a delete of CamundaReleases, under the rules of applyReleaseChange. */
export async function previewReleaseChange(change: Change): Promise<Preview> {
  const errors = await deleteRefusals(change, releaseSchema, releaseRule);
  if (errors.length > 0) return { objects: [], errors };
  return runPreview(change, releaseSchema);
}

/**
 * applyReleaseChange writes a create, an update, or a delete of
 * CamundaReleases with the writer of the settings. It refuses with Forbidden
 * the whole change when releaseDeleteBlock blocks the delete of one of its
 * releases: a release in use, allowed in an environment, or the default of
 * one, with each user named, or any delete while a list cannot be read.
 */
export async function applyReleaseChange(change: Change): Promise<WriteResult> {
  const errors = await deleteRefusals(change, releaseSchema, releaseRule);
  if (errors.length > 0) return { ok: false, errors };
  return runChange(change, releaseSchema);
}

const releaseRule: DeleteRule<typeof RELEASE_SOURCE_KINDS> = {
  kinds: RELEASE_SOURCE_KINDS,
  needs: () => RELEASE_DELETE_KINDS,
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which releaseDeleteBlock does not read.
  blockOf: (lists, unread) => (ref) =>
    releaseDeleteBlock(ref.name, lists as unknown as ReleasesData, unread),
};

/** previewReleaseMove is the dry run of a move of cluster claims to a release, with the checks of runClaimMove. */
export async function previewReleaseMove(change: Change): Promise<Preview> {
  return previewClaimMove(change);
}

/**
 * applyReleaseMove writes a move of cluster claims to a release with the
 * writer of the settings, as one change. runClaimMove says what it refuses.
 */
export async function applyReleaseMove(change: Change): Promise<WriteResult> {
  return runClaimMove(change);
}
