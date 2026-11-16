import "server-only";
import { createHash } from "node:crypto";
import { stringify } from "yaml";
import { REVISION_ANNOTATION } from "@/lib/labels";
import type { KubeObject } from "./types";
import { applyBody } from "./writer";

export { gitBase, gitPath } from "@/lib/git-layout";
export { appliedClientSide, gitOpsManager } from "@/lib/merge";

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
