import type { KindInfo } from "@/lib/k8s/kinds";
import type { ObjectRef } from "@/lib/write/types";

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
