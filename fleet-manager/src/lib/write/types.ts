import type { DiffLine } from "@/lib/diff";

/** WriteMode is how the app saves: to the API, or as a commit in Git. */
export type WriteMode = "direct" | "git";

/** KubeObject is a Kubernetes object as plain JSON. */
export type KubeObject = {
  apiVersion: string;
  kind: string;
  metadata: {
    name: string;
    namespace?: string;
    resourceVersion?: string;
    [key: string]: unknown;
  };
  [key: string]: unknown;
};

/** ObjectRef names one object. namespace is unset for a cluster-scoped kind. */
export type ObjectRef = {
  apiVersion: string;
  kind: string;
  namespace?: string;
  name: string;
};

/** RefKey is "<kind>/<namespace>/<name>", with an empty namespace for a cluster-scoped kind. */
export type RefKey = `${string}/${string}/${string}`;

/**
 * Change is one save of a page: the objects to apply and the objects to
 * delete. preconditions holds the resourceVersion that the page read for
 * an object. A write of that object fails with Stale when the object
 * changed since. force sends each apply of the change with the force
 * option of server-side apply. A change with directOnly goes to the API
 * in Git mode too.
 *
 * The preview of the direct writer applies an object of previewManagers as
 * the field manager that it maps to, unless it sends the object as a merge
 * patch. It sends an object of previewMergePatches as the JSON merge patch
 * of a client-side apply. An object whose live copy is missing or has no
 * last applied file that is a JSON object gets the server-side-apply dry
 * run. apply ignores both. For a change without directOnly, the Git writer
 * sets both.
 */
export type Change = {
  intent: string;
  apply: KubeObject[];
  delete?: ObjectRef[];
  preconditions?: Record<RefKey, string>;
  force?: boolean;
  directOnly?: boolean;
  previewManagers?: Record<RefKey, string>;
  previewMergePatches?: RefKey[];
};

/**
 * ApiErrorCode is why a write failed. Stale: the object changed or was
 * deleted since the page read it. Conflict: another field manager owns a
 * field. Invalid: the API server or the app refused the object. Forbidden:
 * the app may not write it. Unavailable: the API server did not answer,
 * does not serve the kind, or has no such namespace.
 */
export type ApiErrorCode =
  "Stale" | "Conflict" | "Invalid" | "Forbidden" | "Unavailable";

/**
 * ApiError is one refusal. path is the field path, without a leading dot,
 * when the answer names one. manager is the field manager of a Conflict.
 */
export type ApiError = {
  code: ApiErrorCode;
  message: string;
  path?: string;
  manager?: string;
  ref?: ObjectRef;
};

/** PreviewObject is one object of a preview. after is unset for a delete, before for a create. */
export type PreviewObject = {
  ref: ObjectRef;
  before?: KubeObject;
  after?: KubeObject;
  diff: DiffLine[];
};

/** Preview is the result of a dry run. An object with an error is not in objects. */
export type Preview = { objects: PreviewObject[]; errors: ApiError[] };

/**
 * WriteResult is the result of a write. In Git mode, revisions maps each
 * pushed object to its revision hash, and paths maps it to its file in the
 * repository. deleted maps each deleted object to the file that the commit
 * removed. A deleted object is not in revisions or paths.
 */
export type WriteResult =
  | { ok: true; mode: "direct" }
  | {
      ok: true;
      mode: "git";
      commit: string;
      revisions: Record<RefKey, string>;
      paths: Record<RefKey, string>;
      deleted: Record<RefKey, string>;
    }
  | { ok: false; errors: ApiError[] };

/** Writer saves a Change. preview writes nothing. */
export interface Writer {
  mode: WriteMode;
  preview(change: Change): Promise<Preview>;
  apply(change: Change): Promise<WriteResult>;
}
