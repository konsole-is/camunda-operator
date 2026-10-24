import "server-only";
import { createHash } from "node:crypto";
import { stringify } from "yaml";
import type { KindInfo } from "@/lib/k8s/kinds";
import { REVISION_ANNOTATION } from "@/lib/labels";
import {
  APP_FIELD_MANAGER,
  type ManagedFieldsEntry,
  child,
  fieldManagers,
} from "@/lib/merge";
import type { KubeObject, ObjectRef } from "./types";
import { applyBody } from "./writer";

/**
 * gitBase returns git.path without its leading and trailing "/", and ""
 * for the root of the repository. It returns undefined for a path that
 * could leave the repository or name no folder: a "." or ".." segment, an
 * empty segment, or a "\".
 */
export function gitBase(path: string): string | undefined {
  const base = path.replace(/^\/+|\/+$/g, "");
  if (base === "") return "";
  if (base.includes("\\")) return undefined;
  const segments = base.split("/");
  return segments.some((s) => s === "" || s === "." || s === "..")
    ? undefined
    : base;
}

/**
 * gitPath returns the file of ref in the repository (contract 6.3):
 * <base>/<namespace>/<plural>/<name>.yaml, or <base>/_cluster/<plural>/<name>.yaml
 * for a cluster-scoped kind. base comes from gitBase.
 */
export function gitPath(base: string, info: KindInfo, ref: ObjectRef): string {
  const folder = info.scope === "Cluster" ? "_cluster" : ref.namespace;
  const file = `${folder}/${info.plural}/${ref.name}.yaml`;
  return base === "" ? file : `${base}/${file}`;
}

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
