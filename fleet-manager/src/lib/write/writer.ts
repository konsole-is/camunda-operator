import "server-only";
import type { KubeConfig } from "@kubernetes/client-node";
import { objectDiff } from "@/lib/diff";
import {
  KubernetesRequestError,
  kubeConfig,
  requestJSON,
} from "@/lib/k8s/client";
import {
  KindMap,
  apiVersionOf,
  isKindName,
  objectPath,
  type KindInfo,
} from "@/lib/k8s/kinds";
import { APP_FIELD_MANAGER } from "@/lib/merge";
import { apiErrorsOf, staleError } from "./errors";
import type {
  ApiError,
  Change,
  KubeObject,
  ObjectRef,
  Preview,
  PreviewObject,
  RefKey,
  WriteResult,
  Writer,
} from "./types";

/** WRITE_TIMEOUT_MS bounds each request of a write. */
export const WRITE_TIMEOUT_MS = 15_000;

/**
 * WriteOnlyKindMap is every kind that the direct writer writes beside the
 * kinds of KindMap. The cache does not list or watch these kinds.
 */
export const WriteOnlyKindMap = {
  ConfigMap: {
    group: "",
    version: "v1",
    plural: "configmaps",
    scope: "Namespaced",
  },
} as const satisfies Record<string, KindInfo>;

const notAName = new Set<string | undefined>(["", ".", ".."]);

const hiddenValue = "(hidden)";
const changedValue = "(changed)";

/** refKey returns the key of an object in Change.preconditions. */
export function refKey(ref: {
  kind: string;
  namespace?: string;
  name: string;
}): RefKey {
  return `${ref.kind}/${ref.namespace ?? ""}/${ref.name}`;
}

/** refOf returns the reference of an object. */
export function refOf(object: KubeObject): ObjectRef {
  const { namespace, name } = object.metadata;
  return {
    apiVersion: object.apiVersion,
    kind: object.kind,
    ...(namespace === undefined ? {} : { namespace }),
    name,
  };
}

/**
 * applyBody is the object to apply: no managedFields, uid,
 * creationTimestamp, generation, or status, and resourceVersion only from
 * the precondition.
 */
export function applyBody(
  object: KubeObject,
  resourceVersion?: string,
): KubeObject {
  const metadata = omit(object.metadata, [
    "managedFields",
    "uid",
    "creationTimestamp",
    "generation",
    "resourceVersion",
  ]);
  return {
    ...omit(object, ["status"]),
    metadata:
      resourceVersion === undefined
        ? metadata
        : { ...metadata, resourceVersion },
  } as KubeObject;
}

/**
 * diffView is the object as a preview compares it: no managedFields,
 * resourceVersion, generation, uid, or creationTimestamp.
 */
export function diffView(
  object: KubeObject | undefined,
): KubeObject | undefined {
  if (object === undefined) return undefined;
  const metadata = omit(object.metadata, [
    "managedFields",
    "resourceVersion",
    "generation",
    "uid",
    "creationTimestamp",
  ]);
  return { ...object, metadata } as KubeObject;
}

/**
 * directWriter writes to the API server of kc, kubeConfig() by default,
 * with server-side apply and the field manager fleet-manager, with
 * force=false unless the change sets force. It writes only the kinds of
 * KindMap and WriteOnlyKindMap, and refuses any other object with Invalid
 * before it sends a request.
 *
 * apply runs the preview of the whole change first and writes nothing
 * when any object fails it. Then it applies the objects and deletes the
 * refs in the order of the change. When a write fails after that, the
 * objects written before it stay written. A delete of an object that is
 * already gone succeeds. An apply with a precondition fails when the
 * object was deleted since the page read it.
 *
 * A preview of a Secret shows "(hidden)" for each value, and "(changed)"
 * for a value that the change sets to a new one. It holds no annotation.
 */
export function directWriter(kc: KubeConfig = kubeConfig()): Writer {
  const send = <T>(
    method: "GET" | "PATCH" | "DELETE",
    path: string,
    body?: unknown,
    contentType?: string,
  ) =>
    requestJSON<T>(kc, {
      method,
      path,
      body,
      contentType,
      timeoutMs: WRITE_TIMEOUT_MS,
    });

  const read = async (path: string): Promise<KubeObject | undefined> => {
    try {
      return await send<KubeObject>("GET", path);
    } catch (error) {
      if (error instanceof KubernetesRequestError && error.statusCode === 404) {
        return undefined;
      }
      throw error;
    }
  };

  const applyObject = (
    object: KubeObject,
    resourceVersion: string | undefined,
    force: boolean,
    dryRun: boolean,
  ) =>
    send<KubeObject>(
      "PATCH",
      `${pathOf(refOf(object))}?fieldManager=${APP_FIELD_MANAGER}&force=${force}${dryRun ? "&dryRun=All" : ""}`,
      applyBody(object, resourceVersion),
      "application/apply-patch+yaml",
    );

  const deleteObject = (
    ref: ObjectRef,
    preconditions: { resourceVersion?: string; uid?: string },
    dryRun: boolean,
  ) =>
    send<unknown>("DELETE", `${pathOf(ref)}${dryRun ? "?dryRun=All" : ""}`, {
      apiVersion: "v1",
      kind: "DeleteOptions",
      ...(preconditions.resourceVersion === undefined &&
      preconditions.uid === undefined
        ? {}
        : { preconditions }),
    });

  // The app may not delete every kind that it writes, such as a Namespace.
  // Then the object stays, and the page shows the error of the delete.
  const undoCreate = async (ref: ObjectRef, uid: string) => {
    try {
      await deleteObject(ref, { uid }, false);
      return [staleError(ref)];
    } catch (error) {
      if (error instanceof KubernetesRequestError && error.statusCode === 404) {
        return [staleError(ref)];
      }
      return [staleError(ref), ...apiErrorsOf(error, ref, false)];
    }
  };

  // uids holds the uid of each object of the apply list that exists.
  const dryRun = async (
    change: Change,
  ): Promise<Preview & { uids: Map<RefKey, string> }> => {
    const uids = new Map<RefKey, string>();
    const refusals = refsOf(change).flatMap(refusalOf);
    if (refusals.length > 0) return { objects: [], errors: refusals, uids };

    const objects: PreviewObject[] = [];
    const errors: ApiError[] = [];
    for (const object of change.apply) {
      const ref = refOf(object);
      const resourceVersion = change.preconditions?.[refKey(ref)];
      try {
        const before = await read(pathOf(ref));
        // The API server creates a missing object even when the applied
        // object carries a resourceVersion.
        if (before === undefined && resourceVersion !== undefined) {
          errors.push(staleError(ref));
          continue;
        }
        if (typeof before?.metadata.uid === "string") {
          uids.set(refKey(ref), before.metadata.uid);
        }
        const after = await applyObject(
          object,
          resourceVersion,
          change.force ?? false,
          true,
        );
        objects.push(previewObject(ref, before, after));
      } catch (error) {
        errors.push(...apiErrorsOf(error, ref, resourceVersion !== undefined));
      }
    }
    for (const ref of change.delete ?? []) {
      const resourceVersion = change.preconditions?.[refKey(ref)];
      try {
        const before = await read(pathOf(ref));
        if (before === undefined) continue;
        await deleteObject(ref, { resourceVersion }, true);
        objects.push(previewObject(ref, before, undefined));
      } catch (error) {
        errors.push(...apiErrorsOf(error, ref, resourceVersion !== undefined));
      }
    }
    return { objects, errors, uids };
  };

  const preview = async (change: Change): Promise<Preview> => {
    const { objects, errors } = await dryRun(change);
    return { objects, errors };
  };

  const apply = async (change: Change): Promise<WriteResult> => {
    const { errors, uids } = await dryRun(change);
    if (errors.length > 0) return { ok: false, errors };

    for (const object of change.apply) {
      const ref = refOf(object);
      const resourceVersion = change.preconditions?.[refKey(ref)];
      try {
        const written = await applyObject(
          object,
          resourceVersion,
          change.force ?? false,
          false,
        );
        const uid = written.metadata.uid;
        // A new uid means that the object was deleted after the dry run
        // and the apply created it again.
        if (
          resourceVersion !== undefined &&
          typeof uid === "string" &&
          uid !== uids.get(refKey(ref))
        ) {
          return { ok: false, errors: await undoCreate(ref, uid) };
        }
      } catch (error) {
        return {
          ok: false,
          errors: apiErrorsOf(error, ref, resourceVersion !== undefined),
        };
      }
    }
    for (const ref of change.delete ?? []) {
      const resourceVersion = change.preconditions?.[refKey(ref)];
      try {
        await deleteObject(ref, { resourceVersion }, false);
      } catch (error) {
        if (
          error instanceof KubernetesRequestError &&
          error.statusCode === 404
        ) {
          continue;
        }
        return {
          ok: false,
          errors: apiErrorsOf(error, ref, resourceVersion !== undefined),
        };
      }
    }
    return { ok: true, mode: "direct" };
  };

  return { mode: "direct", preview, apply };
}

/** getWriter returns the writer of the app: the direct writer of kubeConfig(). */
export async function getWriter(): Promise<Writer> {
  return directWriter();
}

function kindInfoOf(kind: string): KindInfo | undefined {
  if (isKindName(kind)) return KindMap[kind];
  return Object.hasOwn(WriteOnlyKindMap, kind)
    ? WriteOnlyKindMap[kind as keyof typeof WriteOnlyKindMap]
    : undefined;
}

// pathOf takes a ref that refusalOf accepted.
function pathOf(ref: ObjectRef): string {
  return objectPath(kindInfoOf(ref.kind)!, ref.namespace, ref.name);
}

function refsOf(change: Change): ObjectRef[] {
  return [...change.apply.map(refOf), ...(change.delete ?? [])];
}

function refusalOf(ref: ObjectRef): ApiError[] {
  const info = kindInfoOf(ref.kind);
  if (info === undefined || ref.apiVersion !== apiVersionOf(info)) {
    return [
      {
        code: "Invalid",
        message: `The fleet manager does not write ${ref.kind} ${ref.apiVersion}.`,
        ref,
      },
    ];
  }
  // objectPath does not encode "." and "..", and new URL resolves them as
  // path segments, so such a name would address the namespace or the
  // collection.
  if ([ref.name, ref.namespace].some((part) => notAName.has(part))) {
    return [
      {
        code: "Invalid",
        message: `The name or the namespace of the ${ref.kind} is not a valid name.`,
        ref,
      },
    ];
  }
  const namespaced = info.scope === "Namespaced";
  if (namespaced !== (ref.namespace !== undefined)) {
    return [
      {
        code: "Invalid",
        message: namespaced
          ? `A ${ref.kind} needs a namespace.`
          : `A ${ref.kind} has no namespace.`,
        ref,
      },
    ];
  }
  return [];
}

function previewObject(
  ref: ObjectRef,
  before: KubeObject | undefined,
  after: KubeObject | undefined,
): PreviewObject {
  const [shownBefore, shownAfter] =
    ref.kind === "Secret"
      ? hideSecretValues(diffView(before), diffView(after))
      : [diffView(before), diffView(after)];
  return {
    ref,
    ...(shownBefore === undefined ? {} : { before: shownBefore }),
    ...(shownAfter === undefined ? {} : { after: shownAfter }),
    diff: objectDiff(shownBefore, shownAfter),
  };
}

// A preview goes to the browser, and no Secret value may leave the app
// server (contract 6.2). An annotation can hold a copy of a value, as the
// last-applied annotation of kubectl does.
function hideSecretValues(
  before: KubeObject | undefined,
  after: KubeObject | undefined,
): [KubeObject | undefined, KubeObject | undefined] {
  const old = (before?.data ?? {}) as Record<string, unknown>;
  const hide = (
    object: KubeObject | undefined,
    shown: (key: string, value: unknown) => string,
  ): KubeObject | undefined => {
    if (object === undefined) return undefined;
    const rest = omit(object, ["stringData"]);
    const data = Object.fromEntries(
      Object.entries((object.data ?? {}) as Record<string, unknown>).map(
        ([key, value]) => [key, shown(key, value)],
      ),
    );
    return {
      ...(rest as KubeObject),
      metadata: omit(object.metadata, [
        "annotations",
      ]) as KubeObject["metadata"],
      ...(object.data === undefined ? {} : { data }),
    };
  };
  return [
    hide(before, () => hiddenValue),
    hide(after, (key, value) =>
      Object.hasOwn(old, key) && old[key] === value
        ? hiddenValue
        : changedValue,
    ),
  ];
}

function omit<T extends Record<string, unknown>>(
  record: T,
  keys: string[],
): Partial<T> {
  return Object.fromEntries(
    Object.entries(record).filter(([key]) => !keys.includes(key)),
  ) as Partial<T>;
}
