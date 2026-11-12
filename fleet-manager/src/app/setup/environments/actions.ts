"use server";

import {
  adminChangeSchema,
  namespaceLabelChangeSchema,
  runChange,
  runPreview,
} from "@/lib/write/run-change";
import type { Change, Preview, WriteResult } from "@/lib/write/types";

const environmentSchema = adminChangeSchema(["Environment"]);

/** previewEnvironmentChange is the dry run of a create, an update, or a delete of an Environment. */
export async function previewEnvironmentChange(
  change: Change,
): Promise<Preview> {
  return runPreview(change, environmentSchema);
}

/** applyEnvironmentChange writes a create, an update, or a delete of an Environment with the writer of the settings. */
export async function applyEnvironmentChange(
  change: Change,
): Promise<WriteResult> {
  return runChange(change, environmentSchema);
}

/** previewNamespaceLabel is the dry run of the environment label of one namespace. */
export async function previewNamespaceLabel(change: Change): Promise<Preview> {
  return runPreview(change, namespaceLabelChangeSchema);
}

/** applyNamespaceLabel sets the environment label of one namespace on the cluster, also in Git mode. */
export async function applyNamespaceLabel(
  change: Change,
): Promise<WriteResult> {
  return runChange(change, namespaceLabelChangeSchema);
}
