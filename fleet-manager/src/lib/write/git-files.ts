import "server-only";
import { createHash } from "node:crypto";
import { stringify } from "yaml";
import { REVISION_ANNOTATION } from "@/lib/labels";
import {
  APP_FIELD_MANAGER,
  type ManagedFieldsEntry,
  child,
  fieldManagers,
} from "@/lib/merge";
import { lastAppliedOf } from "./merge-patch";
import type { KubeObject } from "./types";
import { applyBody } from "./writer";

export { gitBase, gitPath } from "@/lib/git-layout";

/**
 * renderObject returns the YAML file of object for Git and its revision.
 * The file has no status and no server fields of metadata, and carries the
 * revision in the annotation REVISION_ANNOTATION. The revision is the hex
 * SHA-256 of the content: the same object in another key order, or with
 * an old revision annotation, gets the same revision.
 */
export function renderObject(object: KubeObject): {
  text: string;
  revision: string;
} {
  const body = applyBody(object);
  const annotations = {
    ...((body.metadata.annotations ?? {}) as Record<string, string>),
  };
  delete annotations[REVISION_ANNOTATION];
  const metadata = { ...body.metadata };
  delete metadata.annotations;
  if (Object.keys(annotations).length > 0) metadata.annotations = annotations;
  const content = { ...body, metadata };
  const revision = createHash("sha256")
    .update(stringify(content, { sortMapEntries: true }))
    .digest("hex");
  const text = stringify(
    {
      ...content,
      metadata: {
        ...metadata,
        annotations: { ...annotations, [REVISION_ANNOTATION]: revision },
      },
    },
    { sortMapEntries: true },
  );
  return { text, revision };
}

/**
 * gitOpsManager returns the field manager of the GitOps tool on live: the
 * manager that applied REVISION_ANNOTATION with server-side apply. It
 * is never APP_FIELD_MANAGER. It returns undefined when live is undefined,
 * or when no such manager exists, for example when the GitOps tool applies
 * with client-side apply.
 */
export function gitOpsManager(
  live: KubeObject | undefined,
): string | undefined {
  const entries = (live?.metadata.managedFields ?? []) as ManagedFieldsEntry[];
  const applied = entries.filter(
    (e) => e.operation === "Apply" && e.manager !== APP_FIELD_MANAGER,
  );
  return fieldManagers({ metadata: { managedFields: applied } })[
    child(child("metadata", "annotations"), REVISION_ANNOTATION)
  ];
}

/**
 * appliedClientSide reports whether the GitOps tool applied live with
 * client-side apply in a way that a JSON merge patch repeats: live is a
 * custom resource and lastAppliedOf(live) is not undefined. A kind of a
 * group without a dot, such as the core group or apps, or of a *.k8s.io
 * group counts as built in, because kubectl sends a built-in kind a
 * strategic merge patch, which merges some lists by key.
 */
export function appliedClientSide(live: KubeObject | undefined): boolean {
  if (live === undefined) return false;
  const slash = live.apiVersion.indexOf("/");
  const group = slash === -1 ? "" : live.apiVersion.slice(0, slash);
  if (!group.includes(".") || group.endsWith(".k8s.io")) return false;
  return lastAppliedOf(live) !== undefined;
}
