import "server-only";
import type { KubeConfig } from "@kubernetes/client-node";
import { objectDiff } from "@/lib/diff";
import {
  isNotFound,
  kubeConfig,
  requestJSON,
  unlessNotFound,
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
import { gitWriter } from "./git";
import { clientSideApplyPatch } from "./merge-patch";
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
 * directWriter writes to the API server of kc, kubeConfig() by default, by
 * server-side apply with the field manager fleet-manager and force=false
 * unless the change sets force. It refuses a kind outside KindMap and
 * WriteOnlyKindMap with Invalid, before any request.
 *
 * apply writes nothing when the preview of any object fails. Then it
 * applies and deletes in the order of the change, and a failure keeps the
 * objects written before it. A delete of an object that is gone succeeds.
 * An apply with a precondition fails with Stale when the object was
 * deleted since the page read it. After a delete between the preview and
 * the write, a kind that the app may not delete, such as a Namespace, can
 * exist again, with the error of that delete beside Stale.
 *
 * A preview of a Secret shows "(hidden)" for each value, and "(changed)"
 * for a value that the change sets to a new one. It holds no annotation.
 * A preview sends an object of previewMergePatches as the merge patch of a
 * client-side apply (see Change).
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

  const read = (ref: ObjectRef) => readObject(kc, ref);

  const applyObject = (
    object: KubeObject,
    resourceVersion: string | undefined,
    force: boolean,
    dryRun: boolean,
    manager = APP_FIELD_MANAGER,
  ) =>
    send<KubeObject>(
      "PATCH",
      `${pathOf(refOf(object))}?fieldManager=${encodeURIComponent(manager)}&force=${force}${dryRun ? "&dryRun=All" : ""}`,
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

  const undoCreate = async (ref: ObjectRef, uid: string) => {
    try {
      await deleteObject(ref, { uid }, false);
      return [staleError(ref)];
    } catch (error) {
      if (isNotFound(error)) {
        return [staleError(ref)];
      }
      return [staleError(ref), ...apiErrorsOf(error, ref, false)];
    }
  };

  // uids holds the uid of each object of the apply list that exists.
  const dryRun = async (
    change: Change,
    options: {
      managers?: Record<RefKey, string>;
      mergePatches?: RefKey[];
    } = {},
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
        const before = await read(ref);
        // The API server creates a missing object even when the applied
        // object carries a resourceVersion.
        if (before === undefined && resourceVersion !== undefined) {
          errors.push(staleError(ref));
          continue;
        }
        if (typeof before?.metadata.uid === "string") {
          uids.set(refKey(ref), before.metadata.uid);
        }
        const patch = options.mergePatches?.includes(refKey(ref))
          ? clientSideApplyPatch(before, applyBody(object, resourceVersion))
          : undefined;
        const after =
          patch === undefined
            ? await applyObject(
                object,
                resourceVersion,
                change.force ?? false,
                true,
                options.managers?.[refKey(ref)],
              )
            : await send<KubeObject>(
                "PATCH",
                `${pathOf(ref)}?fieldManager=${encodeURIComponent(APP_FIELD_MANAGER)}&dryRun=All`,
                patch,
                "application/merge-patch+json",
              );
        objects.push(previewObject(ref, before, after));
      } catch (error) {
        errors.push(...apiErrorsOf(error, ref, resourceVersion !== undefined));
      }
    }
    for (const ref of change.delete ?? []) {
      const resourceVersion = change.preconditions?.[refKey(ref)];
      try {
        const before = await read(ref);
        if (before === undefined) continue;
        await deleteObject(ref, { resourceVersion }, true);
        objects.push(previewObject(ref, before, undefined));
      } catch (error) {
        if (isNotFound(error)) continue;
        errors.push(...apiErrorsOf(error, ref, resourceVersion !== undefined));
      }
    }
    return { objects, errors, uids };
  };

  const preview = async (change: Change): Promise<Preview> => {
    const { objects, errors } = await dryRun(change, {
      managers: change.previewManagers,
      mergePatches: change.previewMergePatches,
    });
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
        if (isNotFound(error)) {
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

/**
 * getWriter returns the Git writer when the setting git.repository is set,
 * else the direct writer of kubeConfig(). It throws when the settings
 * cannot be read.
 */
export async function getWriter(): Promise<Writer> {
  // The settings store writes through directWriter of this module, so a
  // static import of the store would be a cycle.
  const { getSettings } = await import("@/lib/settings/store");
  const settings = await getSettings();
  const url = settings["git.repository"];
  const kc = kubeConfig();
  if (!url) return directWriter(kc);
  return gitWriter({
    target: {
      url,
      branch: settings["git.branch"],
      path: settings["git.path"],
      credentialsSecretRef: settings["git.credentialsSecretRef"],
    },
    direct: directWriter(kc),
    read: (ref) => readObject(kc, ref),
  });
}

/**
 * readObject returns the object of ref from the API server of kc, or
 * undefined when it does not exist. ref must pass refusalOf.
 */
export function readObject(
  kc: KubeConfig,
  ref: ObjectRef,
): Promise<KubeObject | undefined> {
  return unlessNotFound(
    requestJSON<KubeObject>(kc, {
      method: "GET",
      path: pathOf(ref),
      timeoutMs: WRITE_TIMEOUT_MS,
    }),
  );
}

/** kindInfoOf returns the kind of KindMap or WriteOnlyKindMap, or undefined for any other kind. */
export function kindInfoOf(kind: string): KindInfo | undefined {
  if (isKindName(kind)) return KindMap[kind];
  return Object.hasOwn(WriteOnlyKindMap, kind)
    ? WriteOnlyKindMap[kind as keyof typeof WriteOnlyKindMap]
    : undefined;
}

// pathOf takes a ref that refusalOf accepted.
function pathOf(ref: ObjectRef): string {
  return objectPath(kindInfoOf(ref.kind)!, ref.namespace, ref.name);
}

/** refsOf returns the refs of the objects that change applies, then of those it deletes. */
export function refsOf(change: Change): ObjectRef[] {
  return [...change.apply.map(refOf), ...(change.delete ?? [])];
}

/**
 * refusalOf returns one Invalid error when the fleet manager does not write
 * ref: a kind outside KindMap and WriteOnlyKindMap, another apiVersion, a
 * name that is not a valid name, or a namespace that does not match the
 * scope of the kind. It returns none for a ref that it accepts.
 */
export function refusalOf(ref: ObjectRef): ApiError[] {
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

/**
 * previewObject returns the preview of ref from before and after, without
 * server fields. A Secret shows "(hidden)" or "(changed)" for each value.
 */
export function previewObject(
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
