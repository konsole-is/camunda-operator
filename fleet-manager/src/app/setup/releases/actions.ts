"use server";

import { RELEASE_SOURCE_KINDS } from "@/components/setup/releases/source-kinds";
import type { ReleasesData } from "@/components/setup/releases/sources";
import { listEach } from "@/lib/k8s/list-each";
import { deleteRefusalText, releaseDeleteBlock } from "@/lib/usage";
import {
  adminChangeSchema,
  previewClaimMove,
  runChange,
  runClaimMove,
  runPreview,
} from "@/lib/write/run-change";
import type { ApiError, Change, Preview, WriteResult } from "@/lib/write/types";

const releaseSchema = adminChangeSchema(["CamundaRelease"]);

/** previewReleaseChange is the dry run of a create, an update, or a delete of CamundaReleases, under the rules of applyReleaseChange. */
export async function previewReleaseChange(change: Change): Promise<Preview> {
  const errors = await deleteRefusals(change);
  if (errors.length > 0) return { objects: [], errors };
  return runPreview(change, releaseSchema);
}

/**
 * applyReleaseChange writes a create, an update, or a delete of
 * CamundaReleases with the writer of the settings. Before a delete, it
 * reads the lists of the release pages from the cache of the server. It
 * refuses with Forbidden the whole change when releaseDeleteBlock blocks
 * the delete of one of its releases: a release in use, allowed in an
 * environment, or the default of one, with each user named, or any delete
 * while a list cannot be read.
 */
export async function applyReleaseChange(change: Change): Promise<WriteResult> {
  const errors = await deleteRefusals(change);
  if (errors.length > 0) return { ok: false, errors };
  return runChange(change, releaseSchema);
}

// deleteRefusals returns no errors for a change that the schema refuses, so
// that the writer gives its Invalid errors.
async function deleteRefusals(input: Change): Promise<ApiError[]> {
  const parsed = releaseSchema.safeParse(input);
  const deletes = parsed.success ? (parsed.data.delete ?? []) : [];
  if (deletes.length === 0) return [];
  const { lists, readErrors } = await listEach(RELEASE_SOURCE_KINDS);
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which releaseDeleteBlock does not read.
  const sources = lists as unknown as ReleasesData;
  const unread = readErrors.map((e) => e.kind);
  return deletes.flatMap((ref): ApiError[] => {
    const block = releaseDeleteBlock(ref.name, sources, unread);
    return block
      ? [{ code: "Forbidden", message: deleteRefusalText(block), ref }]
      : [];
  });
}

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
