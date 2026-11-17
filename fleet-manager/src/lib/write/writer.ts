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
  collectionPath,
  isKindName,
  objectPath,
  type KindInfo,
  type KindName,
} from "@/lib/k8s/kinds";
import { APP_FIELD_MANAGER } from "@/lib/merge";
import { equalJSON } from "@/lib/policy/json";
import type { Settings } from "@/lib/settings/keys";
import { apiErrorsOf, existsError, staleError } from "./errors";
import { gitWriter } from "./git";
import { clientSideApplyPatch } from "./merge-patch";
import {
  foreignFields,
  offered,
  takenFields,
  withoutFields,
} from "./ownership";
import { refKey, refOf } from "./refs";
import type {
  ApiError,
  Change,
  ForeignField,
  KubeObject,
  ObjectOwnership,
  ObjectRef,
  Preview,
  PreviewObject,
  RefKey,
  WriteMode,
  WriteResult,
  Writer,
} from "./types";

export { refKey, refOf } from "./refs";

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
 * unless the change sets force or takes over a field. It refuses a kind outside KindMap and
 * WriteOnlyKindMap with Invalid, before any request.
 *
 * apply writes nothing when the preview of any object fails. Then it
 * applies and deletes in the order of the change, and a failure keeps the
 * objects written before it. A delete of an object that is gone succeeds.
 * An apply with a precondition fails with Stale when the object was
 * deleted or changed since the page read it, also when the change would
 * conflict. A delete with a precondition or a UID
 * fails with Stale when the object changed or was created again since.
 * After a delete between the preview and the write, a kind that the app
 * may not delete, such as a Namespace, can exist again, with the error of
 * that delete beside Stale.
 *
 * A create of an object that exists fails with Stale in the dry run. An
 * object in a namespace that the same change makes is not dry-run, and the
 * preview shows it as rendered. When such an object fails after apply made
 * its namespace, the namespace stays, and the result names it in
 * createdNamespace.
 *
 * A preview of a Secret shows "(hidden)" for each value, and "(changed)"
 * for a value that the change sets to a new one. It holds no annotation.
 *
 * The preview names in ownership each updated object with a precondition
 * that has a field of spec, metadata.labels, or metadata.annotations of
 * another field manager that the change changes or clears. For an object that change.takeOver or change.remove lists, both
 * writes refuse it after the read, with no other request for it: with
 * Conflict for each listed field that a controller of the operator owns,
 * and with Invalid when the lists are not the offered fields of ownership
 * at the precondition. Then a dry run with force must take no field
 * that takeOver does not list, or the object fails with Conflict. apply
 * applies such an object with force only when takeOver lists a field, and
 * then removes the fields of remove with a JSON patch at the
 * resourceVersion that the apply returned. The preview shows the object
 * after the removal. When the removal fails, apply stops, the apply stays,
 * and each error says which fields the fleet manager did not remove. A
 * newer write between the apply and the removal gives Stale.
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

  // The API server reports a field manager conflict before a stale
  // resourceVersion, also for a write that lands after the read.
  const staleFirst = async (
    errors: ApiError[],
    ref: ObjectRef,
    resourceVersion: string | undefined,
  ): Promise<ApiError[]> => {
    if (
      resourceVersion === undefined ||
      !errors.some((e) => e.code === "Conflict")
    )
      return errors;
    const now = await read(ref).catch(() => null);
    if (now === null || now?.metadata.resourceVersion === resourceVersion)
      return errors;
    return [staleError(ref)];
  };

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
    preconditions: DeletePreconditions,
    dryRun: boolean,
  ) =>
    // The API server reads the options of a delete from its body when it
    // has one, and then ignores the query, so dryRun goes in the body.
    send<unknown>("DELETE", pathOf(ref), {
      apiVersion: "v1",
      kind: "DeleteOptions",
      ...(dryRun ? { dryRun: ["All"] } : {}),
      ...(preconditionsSet(preconditions) ? { preconditions } : {}),
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

  const replaceObject = (
    ref: ObjectRef,
    from: KubeObject,
    to: KubeObject,
    resourceVersion: string | undefined,
    dryRun: boolean,
  ) =>
    send<KubeObject>(
      "PATCH",
      `${pathOf(ref)}?fieldManager=${encodeURIComponent(APP_FIELD_MANAGER)}${dryRun ? "&dryRun=All" : ""}`,
      replacePatch(from, to, resourceVersion),
      // A JSON merge patch reads a null inside a value as a removal.
      "application/json-patch+json",
    );

  // uids holds the uid of each object of the apply list that exists. plans
  // holds, for each object with a take-over list, whether apply forces it
  // and the fields that apply then removes.
  const dryRun = async (
    change: Change,
    options: {
      managers?: Record<RefKey, string>;
      mergePatches?: RefKey[];
    } = {},
  ): Promise<
    Preview & {
      uids: Map<RefKey, string>;
      fresh: Set<KubeObject>;
      plans: Map<RefKey, TakeOverPlan>;
    }
  > => {
    const uids = new Map<RefKey, string>();
    const plans = new Map<RefKey, TakeOverPlan>();
    const refusals = refsOf(change).flatMap(refusalOf);
    if (refusals.length > 0) {
      return { objects: [], errors: refusals, uids, fresh: new Set(), plans };
    }

    const objects: PreviewObject[] = [];
    const errors: ApiError[] = [];
    const ownership: ObjectOwnership[] = [];
    // A Namespace that cannot be read gets that error from its own dry run
    // below.
    const fresh = await inNewNamespaces(change.apply, read).catch(
      () => new Set<KubeObject>(),
    );
    for (const object of change.apply) {
      const ref = refOf(object);
      const key = refKey(ref);
      const resourceVersion = change.preconditions?.[key];
      try {
        const before = await read(ref);
        // The API server creates a missing object even when the applied
        // object carries a resourceVersion.
        if (before === undefined && resourceVersion !== undefined) {
          errors.push(staleError(ref));
          continue;
        }
        if (before !== undefined && change.create?.includes(key)) {
          errors.push(existsError(ref));
          continue;
        }
        // The API server reports a field manager conflict before a stale
        // resourceVersion, and the page must read the object again first.
        if (
          resourceVersion !== undefined &&
          before !== undefined &&
          before.metadata.resourceVersion !== resourceVersion
        ) {
          errors.push(staleError(ref));
          continue;
        }
        if (typeof before?.metadata.uid === "string") {
          uids.set(key, before.metadata.uid);
        }
        const foreign =
          before !== undefined && resourceVersion !== undefined
            ? foreignFields(before, object)
            : undefined;
        if (foreign && foreign.changed.length + foreign.cleared.length > 0)
          ownership.push({ ref, ...foreign });
        const lists = listsOf(change, key);
        if (lists !== undefined) {
          const refused = takeOverRefusals(
            ref,
            before,
            resourceVersion,
            foreign,
            lists,
          );
          if (refused.length > 0) {
            errors.push(...refused);
            continue;
          }
          const force = lists.takeOver.length > 0;
          const after = await applyObject(object, resourceVersion, force, true);
          if (force) {
            const unlisted = unlistedTakes(
              ref,
              before!,
              after,
              foreign!.changed.filter((f) => lists.takeOver.includes(f.path)),
            );
            if (unlisted.length > 0) {
              errors.push(...unlisted);
              continue;
            }
          }
          const cleared = foreign!.cleared.filter(offered);
          plans.set(key, { force, cleared });
          const shown =
            cleared.length === 0
              ? after
              : await replaceObject(
                  ref,
                  before!,
                  withoutFields(after, cleared),
                  resourceVersion,
                  true,
                );
          objects.push(previewObject(ref, before, shown));
          continue;
        }
        // The API server refuses a dry run in a namespace that does not
        // exist yet.
        if (fresh.has(object)) {
          objects.push(previewObject(ref, undefined, object));
          continue;
        }
        const patch = options.mergePatches?.includes(key)
          ? clientSideApplyPatch(before, applyBody(object, resourceVersion))
          : undefined;
        const after =
          patch === undefined
            ? await applyObject(
                object,
                resourceVersion,
                change.force ?? false,
                true,
                options.managers?.[key],
              )
            : await send<KubeObject>(
                "PATCH",
                `${pathOf(ref)}?fieldManager=${encodeURIComponent(APP_FIELD_MANAGER)}&dryRun=All`,
                patch,
                "application/merge-patch+json",
              );
        objects.push(previewObject(ref, before, after));
      } catch (error) {
        errors.push(
          ...(await staleFirst(
            apiErrorsOf(error, ref, resourceVersion !== undefined),
            ref,
            resourceVersion,
          )),
        );
      }
    }
    for (const ref of change.delete ?? []) {
      const preconditions = deletePreconditions(change, ref);
      try {
        const before = await read(ref);
        if (before === undefined) continue;
        await deleteObject(ref, preconditions, true);
        objects.push(previewObject(ref, before, undefined));
      } catch (error) {
        if (isNotFound(error)) continue;
        errors.push(
          ...apiErrorsOf(error, ref, preconditionsSet(preconditions)),
        );
      }
    }
    return {
      objects,
      errors,
      ...(ownership.length > 0 ? { ownership } : {}),
      uids,
      fresh,
      plans,
    };
  };

  const preview = async (change: Change): Promise<Preview> => {
    const { objects, errors, ownership } = await dryRun(change, {
      managers: change.previewManagers,
      mergePatches: change.previewMergePatches,
    });
    return { objects, errors, ...(ownership ? { ownership } : {}) };
  };

  const apply = async (change: Change): Promise<WriteResult> => {
    const { errors, uids, fresh, plans } = await dryRun(change);
    if (errors.length > 0) return { ok: false, errors };

    const newNamespaces = new Set([...fresh].map((o) => o.metadata.namespace));
    const created = new Set<string>();
    for (const object of change.apply) {
      const ref = refOf(object);
      const resourceVersion = change.preconditions?.[refKey(ref)];
      const plan = plans.get(refKey(ref));
      let written: KubeObject;
      try {
        written = await applyObject(
          object,
          resourceVersion,
          plan?.force ?? change.force ?? false,
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
        if (object.kind === "Namespace" && newNamespaces.has(ref.name)) {
          created.add(ref.name);
        }
      } catch (error) {
        const failed = await staleFirst(
          apiErrorsOf(error, ref, resourceVersion !== undefined),
          ref,
          resourceVersion,
        );
        if (ref.namespace !== undefined && created.has(ref.namespace)) {
          return {
            ok: false,
            createdNamespace: ref.namespace,
            errors: failed.map((e) => inCreatedNamespace(e, ref)),
          };
        }
        return { ok: false, errors: failed };
      }
      if (plan === undefined || plan.cleared.length === 0) continue;
      try {
        await replaceObject(
          ref,
          written,
          withoutFields(written, plan.cleared),
          written.metadata.resourceVersion,
          false,
        );
      } catch (error) {
        const paths = plan.cleared.map((f) => f.path).join(", ");
        return {
          ok: false,
          errors: apiErrorsOf(error, ref, true).map((e) => ({
            ...e,
            message: `The fleet manager saved the ${ref.kind} ${ref.name}, but did not remove ${paths}. ${e.message}`,
          })),
        };
      }
    }
    for (const ref of change.delete ?? []) {
      const preconditions = deletePreconditions(change, ref);
      try {
        await deleteObject(ref, preconditions, false);
      } catch (error) {
        if (isNotFound(error)) {
          continue;
        }
        return {
          ok: false,
          errors: apiErrorsOf(error, ref, preconditionsSet(preconditions)),
        };
      }
    }
    return { ok: true, mode: "direct" };
  };

  return { mode: "direct", preview, apply };
}

type DeletePreconditions = { resourceVersion?: string; uid?: string };

// TakeOverPlan is what apply does for an object with a take-over list:
// force the apply, and remove cleared after it.
type TakeOverPlan = { force: boolean; cleared: ForeignField[] };

type TakeOverLists = { takeOver: string[]; remove: string[] };

function listsOf(change: Change, key: RefKey): TakeOverLists | undefined {
  const takeOver = change.takeOver?.[key];
  const remove = change.remove?.[key];
  if (takeOver === undefined && remove === undefined) return undefined;
  return { takeOver: takeOver ?? [], remove: remove ?? [] };
}

// takeOverRefusals checks the lists of an object against what the server
// reads itself: the page never decides which fields the save takes.
function takeOverRefusals(
  ref: ObjectRef,
  before: KubeObject | undefined,
  resourceVersion: string | undefined,
  foreign: { changed: ForeignField[]; cleared: ForeignField[] } | undefined,
  lists: TakeOverLists,
): ApiError[] {
  if (before === undefined || resourceVersion === undefined || !foreign)
    return [
      {
        code: "Invalid",
        message: "A take-over names an object that the change updates.",
        ref,
      },
    ];
  const fields = [...foreign.changed, ...foreign.cleared];
  const operator: ApiError[] = [];
  for (const path of [...lists.takeOver, ...lists.remove]) {
    const manager = fields
      .filter((f) => f.path === path)
      .flatMap((f) => f.managers)
      .find((m) => m.kind === "operator")?.name;
    if (manager !== undefined)
      operator.push({
        code: "Conflict",
        message: `${path} belongs to ${manager}, a controller of the operator. The fleet manager does not take it over.`,
        path,
        manager,
        ref,
      });
  }
  if (operator.length > 0) return operator;
  for (const path of new Set([...lists.takeOver, ...lists.remove])) {
    if (fields.filter((f) => f.path === path).length > 1)
      return [
        {
          code: "Invalid",
          message: `${path} names more than one field of the ${ref.kind} ${ref.name}. The fleet manager does not take over a field that it cannot name exactly.`,
          path,
          ref,
        },
      ];
  }
  const offeredPaths = (list: ForeignField[]) =>
    list.filter(offered).map((f) => f.path);
  if (
    !samePaths(lists.takeOver, offeredPaths(foreign.changed)) ||
    !samePaths(lists.remove, offeredPaths(foreign.cleared))
  )
    return [
      {
        code: "Invalid",
        message:
          "The fields of other tools are not the ones that the review showed. Review the change again.",
        ref,
      },
    ];
  return [];
}

// unlistedTakes gives a Conflict for each field that the forced dry run
// took from a manager while listed does not hold it, as the walk of
// foreignFields can miss a field that the API server counts.
function unlistedTakes(
  ref: ObjectRef,
  before: KubeObject,
  after: KubeObject,
  listed: ForeignField[],
): ApiError[] {
  // Two fields can have one path string, so compare their steps.
  const keys = new Set(listed.map((f) => JSON.stringify(f.steps)));
  return takenFields(before, after)
    .filter((f) => !keys.has(JSON.stringify(f.steps)))
    .flatMap((f) =>
      f.managers.map((m): ApiError => ({
        code: "Conflict",
        message: `The save would take ${f.path} from ${m.name}, and the review did not list it.`,
        path: f.path,
        manager: m.name,
        ref,
      })),
    );
}

function samePaths(a: string[], b: string[]): boolean {
  return (
    a.length === b.length &&
    new Set(a).size === a.length &&
    b.every((path) => a.includes(path))
  );
}

type JsonPatchOp =
  | { op: "add" | "replace"; path: string; value: unknown }
  | { op: "remove"; path: string };

// replacePatch is the JSON patch from from to to of each top-level field
// but apiVersion, kind, metadata, and status, and of metadata.labels and
// metadata.annotations, with resourceVersion as its precondition.
function replacePatch(
  from: KubeObject,
  to: KubeObject,
  resourceVersion: string | undefined,
): JsonPatchOp[] {
  const ops: JsonPatchOp[] =
    resourceVersion === undefined
      ? []
      : [
          {
            op: "replace",
            path: "/metadata/resourceVersion",
            value: resourceVersion,
          },
        ];
  const a = applyBody(from) as Record<string, unknown>;
  const b = applyBody(to) as Record<string, unknown>;
  const differ = (
    at: string,
    x: Record<string, unknown>,
    y: Record<string, unknown>,
    keys: string[],
  ) => {
    for (const key of keys) {
      const path = `${at}/${key.replaceAll("~", "~0").replaceAll("/", "~1")}`;
      if (!Object.hasOwn(y, key)) {
        if (Object.hasOwn(x, key)) ops.push({ op: "remove", path });
      } else if (!Object.hasOwn(x, key) || !equalJSON(x[key], y[key]))
        // A field of a list item that the patch sends with its old value
        // keeps its field managers. The fleet manager does not become a
        // manager of a field that the patch sends with its old value.
        ops.push({ op: "add", path, value: y[key] });
    }
  };
  const top = new Set([...Object.keys(a), ...Object.keys(b)]);
  for (const key of ["apiVersion", "kind", "metadata"]) top.delete(key);
  differ("", a, b, [...top]);
  differ(
    "/metadata",
    from.metadata as Record<string, unknown>,
    to.metadata as Record<string, unknown>,
    ["labels", "annotations"],
  );
  return ops;
}

function deletePreconditions(
  change: Change,
  ref: ObjectRef,
): DeletePreconditions {
  const key = refKey(ref);
  return {
    resourceVersion: change.preconditions?.[key],
    uid: change.uids?.[key],
  };
}

function preconditionsSet(preconditions: DeletePreconditions): boolean {
  return (
    preconditions.resourceVersion !== undefined ||
    preconditions.uid !== undefined
  );
}

// The app never deletes a namespace on its own, so the namespace stays.
function inCreatedNamespace(error: ApiError, ref: ObjectRef): ApiError {
  return {
    ...error,
    message: `The fleet manager created the namespace ${ref.namespace}, but not the ${ref.kind} ${ref.name}. You can save again into the namespace ${ref.namespace}. ${error.message}`,
  };
}

/**
 * getWriter returns the Git writer when the setting git.repository is set,
 * else the direct writer of kubeConfig(). It throws when the settings
 * cannot be read.
 */
export async function getWriter(): Promise<Writer> {
  const settings = await readSettings();
  const kc = kubeConfig();
  if (modeOf(settings) === "direct") return directWriter(kc);
  return gitWriter({
    target: {
      url: settings["git.repository"],
      branch: settings["git.branch"],
      path: settings["git.path"],
      credentialsSecretRef: settings["git.credentialsSecretRef"],
    },
    direct: directWriter(kc),
    read: (ref) => readObject(kc, ref),
  });
}

/** writeMode returns the mode of the writer that getWriter returns. It throws when the settings cannot be read. */
export async function writeMode(): Promise<WriteMode> {
  return modeOf(await readSettings());
}

async function readSettings(): Promise<Settings> {
  // The settings store writes through directWriter of this module, so a
  // static import of the store would be a cycle.
  const { getSettings } = await import("@/lib/settings/store");
  return getSettings();
}

function modeOf(settings: Settings): WriteMode {
  return settings["git.repository"] ? "git" : "direct";
}

/**
 * readLive is readObject with kubeConfig(): a read from the API server, not
 * from the watch cache.
 */
export function readLive(ref: ObjectRef): Promise<KubeObject | undefined> {
  return readObject(kubeConfig(), ref);
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

/**
 * firstExisting returns the first of refs whose object exists on the API
 * server of kc, kubeConfig() by default, or undefined. Each ref must pass
 * refusalOf. It throws when a read fails.
 */
export async function firstExisting(
  refs: readonly ObjectRef[],
  kc: KubeConfig = kubeConfig(),
): Promise<ObjectRef | undefined> {
  for (const ref of refs) {
    if ((await readObject(kc, ref)) !== undefined) return ref;
  }
  return undefined;
}

/**
 * listObjects returns the objects of kind in namespace from the API server
 * of kc, kubeConfig() by default, not from the cache. kind must be a
 * namespaced kind of KindMap. It throws when the read fails.
 */
export async function listObjects(
  kind: KindName,
  namespace: string,
  kc: KubeConfig = kubeConfig(),
): Promise<KubeObject[]> {
  const list = await requestJSON<{ items: KubeObject[] }>(kc, {
    method: "GET",
    path: `${collectionPath(KindMap[kind])}?fieldSelector=${encodeURIComponent(`metadata.namespace=${namespace}`)}`,
    timeoutMs: WRITE_TIMEOUT_MS,
  });
  return list.items;
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

/**
 * inNewNamespaces returns the objects of apply that sit in a namespace that
 * a Namespace of apply makes: one that read does not find.
 */
export async function inNewNamespaces(
  apply: readonly KubeObject[],
  read: (ref: ObjectRef) => Promise<KubeObject | undefined>,
): Promise<Set<KubeObject>> {
  const made = new Set<string>();
  for (const object of apply) {
    if (object.kind !== "Namespace") continue;
    if ((await read(refOf(object))) === undefined) {
      made.add(object.metadata.name);
    }
  }
  return new Set(
    apply.filter(
      (o) =>
        o.metadata.namespace !== undefined && made.has(o.metadata.namespace),
    ),
  );
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
  // objectPath throws for "." and "..". This check gives an Invalid error
  // in place of the throw.
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
