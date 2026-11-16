import "server-only";
import { z } from "zod";
import { apiVersionOf, KindMap, type KindName } from "@/lib/k8s/kinds";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import {
  type BackupLike,
  deletionPolicyOf,
  type DeletionPolicy,
  equalJSON,
  type ResolvedPolicy,
  runningBackup,
} from "@/lib/policy";
import { existsError, staleError } from "./errors";
import { refKey, refOf } from "./refs";
import type {
  ApiError,
  Change,
  RefKey,
  KubeObject,
  ObjectRef,
  Preview,
  WriteResult,
} from "./types";
import { firstExisting, getWriter, listObjects, readLive } from "./writer";

/** kubeObjectSchema accepts an object with apiVersion, kind, and metadata.name, and keeps every other field. */
export const kubeObjectSchema: z.ZodType<KubeObject> = z.looseObject({
  apiVersion: z.string().min(1),
  kind: z.string().min(1),
  metadata: z.looseObject({
    name: z.string().min(1),
    namespace: z.string().min(1).optional(),
    resourceVersion: z.string().optional(),
  }),
});

/** objectRefSchema accepts an ObjectRef. */
export const objectRefSchema: z.ZodType<ObjectRef> = z.object({
  apiVersion: z.string().min(1),
  kind: z.string().min(1),
  namespace: z.string().min(1).optional(),
  name: z.string().min(1),
});

const refKeySchema = z.templateLiteral([
  z.string(),
  "/",
  z.string(),
  "/",
  z.string(),
]);

/**
 * changeSchema accepts a Change with an intent and at least one object to
 * apply or delete. Each key of create names an applied object that has no
 * precondition. Each key of uids and of generations names a deleted object.
 */
export const changeSchema: z.ZodType<Change> = z
  .object({
    intent: z.string().min(1),
    apply: z.array(kubeObjectSchema),
    delete: z.array(objectRefSchema).optional(),
    preconditions: z.record(refKeySchema, z.string()).optional(),
    uids: z.record(refKeySchema, z.string().min(1)).optional(),
    generations: z.record(refKeySchema, z.number().int().positive()).optional(),
    create: z.array(refKeySchema).optional(),
    force: z.boolean().optional(),
    directOnly: z.boolean().optional(),
  })
  .refine((c) => c.apply.length + (c.delete?.length ?? 0) > 0, {
    message: "The change has no object to apply or delete.",
  })
  .refine(
    (c) => {
      const applied = c.apply.map((o) => refKey(refOf(o)));
      return (c.create ?? []).every((key) => applied.includes(key));
    },
    {
      message: "create names an object that the change does not apply.",
      path: ["create"],
    },
  )
  .refine(
    (c) => {
      const deleted: string[] = (c.delete ?? []).map(refKey);
      return Object.keys(c.uids ?? {}).every((key) => deleted.includes(key));
    },
    {
      message: "uids names an object that the change does not delete.",
      path: ["uids"],
    },
  )
  .refine(
    (c) => {
      const deleted: string[] = (c.delete ?? []).map(refKey);
      return Object.keys(c.generations ?? {}).every((key) =>
        deleted.includes(key),
      );
    },
    {
      message: "generations names an object that the change does not delete.",
      path: ["generations"],
    },
  )
  .refine(
    (c) =>
      (c.create ?? []).every((key) => c.preconditions?.[key] === undefined),
    {
      message: "An object of create has no precondition.",
      path: ["create"],
    },
  );

const CLAIM_KINDS: readonly string[] = [
  "CamundaClusterClaim",
  "CamundaManagementClusterClaim",
];

/**
 * claimChangeSchema accepts a Change that applies exactly one claim of a
 * cluster or a management plane, with a namespace, and that has no delete,
 * no create, no force, and no directOnly.
 */
export const claimChangeSchema: z.ZodType<Change> = changeSchema.refine(
  (c) =>
    c.apply.length === 1 &&
    CLAIM_KINDS.includes(c.apply[0].kind) &&
    c.apply[0].metadata.namespace !== undefined &&
    c.delete === undefined &&
    c.create === undefined &&
    c.force === undefined &&
    c.directOnly === undefined,
  { message: "The change must apply one claim and nothing else." },
);

/**
 * claimDeleteSchema accepts a Change that deletes exactly one claim of a
 * cluster or a management plane and nothing else: an empty apply, one
 * delete ref with the kind and apiVersion of a claim, a namespace, and a
 * name and namespace other than "." and "..", the UID and the generation
 * of that claim in uids and generations, and no preconditions, create,
 * force, or directOnly.
 */
export const claimDeleteSchema: z.ZodType<Change> = changeSchema.refine(
  (c) => {
    const ref = c.delete?.[0];
    return (
      c.apply.length === 0 &&
      c.delete?.length === 1 &&
      ref !== undefined &&
      CLAIM_KINDS.includes(ref.kind) &&
      // The direct writer reports a delete that answers 404 as done, so a
      // wrong apiVersion would report a delete that never happened.
      ref.apiVersion === apiVersionOf(KindMap[ref.kind as KindName]) &&
      ref.namespace !== undefined &&
      // runClaimDelete and previewClaimDelete read the claim by its path
      // before the writer checks the ref, and pathSegment throws for "."
      // and "..".
      ![ref.name, ref.namespace].some(
        (part) => part === "." || part === "..",
      ) &&
      c.uids?.[refKey(ref)] !== undefined &&
      c.generations?.[refKey(ref)] !== undefined &&
      c.preconditions === undefined &&
      c.create === undefined &&
      c.force === undefined &&
      c.directOnly === undefined
    );
  },
  { message: "The change must delete one claim and nothing else." },
);

/**
 * CLAIM_DELETE_ATTEMPTS is how many deletes runClaimDelete sends before it
 * refuses a claim whose status keeps changing. A spec change makes the
 * claim controller write the status once, and once more for each request
 * that a plugin reports again.
 */
export const CLAIM_DELETE_ATTEMPTS = 5;

/**
 * runClaimDelete is runChange with claimDeleteSchema, for the delete of a
 * claim. A status write of the claim since the review does not refuse it.
 * It refuses with Stale, and deletes nothing, when the claim is gone, has
 * another UID, or has another generation than the change names, that is
 * when its spec changed since the review. It refuses with Invalid a claim
 * that lost its cleanup finalizer after the claim controller recorded its
 * policy, and with Stale while a PluginRequest that the claim controls and
 * that is not being deleted has another deletion policy than the claim.
 * In direct mode it deletes at the resourceVersion of a fresh read, and
 * reads again after a conflict, at most CLAIM_DELETE_ATTEMPTS times. Then
 * it refuses with Stale and says that the claim kept changing. In Git mode
 * the Git writer checks the UID and the generation. A failed read gives
 * Unavailable.
 */
export async function runClaimDelete(input: unknown): Promise<WriteResult> {
  const parsed = claimDeleteSchema.safeParse(input);
  if (!parsed.success) return { ok: false, errors: issuesOf(parsed.error) };
  const change = parsed.data;
  const ref = change.delete![0];
  try {
    const writer = await getWriter();
    for (let attempt = 0; attempt < CLAIM_DELETE_ATTEMPTS; attempt++) {
      const live = await readLive(ref);
      const refusal =
        reviewRefusalOf(change, ref, live) ??
        (await cleanupRefusalOf(ref, live!));
      if (refusal !== undefined) return { ok: false, errors: [refusal] };
      if (writer.mode === "git") return await writer.apply(change);
      const result = await writer.apply({
        ...change,
        preconditions: { [refKey(ref)]: live!.metadata.resourceVersion ?? "" },
      });
      if (result.ok || !result.errors.every((e) => e.code === "Stale"))
        return result;
    }
  } catch (error) {
    return { ok: false, errors: [unavailable(error)] };
  }
  return {
    ok: false,
    errors: [
      {
        code: "Stale",
        message: `The ${ref.kind} ${ref.name} kept changing during the delete. Delete it again.`,
        ref,
      },
    ],
  };
}

/**
 * previewClaimDelete is runPreview with claimDeleteSchema. It gives no
 * objects and the refusal of runClaimDelete when the claim is gone, has
 * another UID or generation, or lost its cleanup finalizer.
 */
export async function previewClaimDelete(input: unknown): Promise<Preview> {
  const parsed = claimDeleteSchema.safeParse(input);
  if (!parsed.success) return { objects: [], errors: issuesOf(parsed.error) };
  const change = parsed.data;
  const ref = change.delete![0];
  try {
    const live = await readLive(ref);
    // A policy write that corrects the requests can follow the preview.
    const refusal =
      reviewRefusalOf(change, ref, live) ??
      (await cleanupRefusalOf(ref, live!, false));
    if (refusal !== undefined) return { objects: [], errors: [refusal] };
    return await (await getWriter()).preview(change);
  } catch (error) {
    return { objects: [], errors: [unavailable(error)] };
  }
}

function reviewRefusalOf(
  change: Change,
  ref: ObjectRef,
  live: KubeObject | undefined,
): ApiError | undefined {
  if (live === undefined)
    return {
      code: "Stale",
      message: `The ${ref.kind} ${ref.name} does not exist.`,
      ref,
    };
  const key = refKey(ref);
  return live.metadata.uid !== change.uids?.[key] ||
    live.metadata.generation !== change.generations?.[key]
    ? staleError(ref)
    : undefined;
}

const CLEANUP_FINALIZER = "fleet.camunda.io/cleanup";

// The claim finalizer writes no policy on a request that is already being
// deleted. A claim without its finalizer gives no policy to its requests
// on delete.
async function cleanupRefusalOf(
  ref: ObjectRef,
  live: KubeObject,
  requests = true,
): Promise<ApiError | undefined> {
  const { uid, finalizers } = live.metadata as {
    uid?: string;
    finalizers?: string[];
  };
  const spec = live.spec as
    { deletionPolicy?: DeletionPolicy | "" } | undefined;
  const status = live.status as { resolvedPolicy?: ResolvedPolicy } | undefined;
  if (
    status?.resolvedPolicy !== undefined &&
    !(finalizers ?? []).includes(CLEANUP_FINALIZER)
  )
    return {
      code: "Invalid",
      message: `The ${ref.kind} ${ref.name} has no finalizer ${CLEANUP_FINALIZER}, so the operator cannot give the deletion policy to its options. Add the finalizer back, then delete the claim again.`,
      ref,
    };
  if (!requests) return undefined;
  const policy = deletionPolicyOf(spec?.deletionPolicy, status?.resolvedPolicy);
  const behind = (await listObjects("PluginRequest", ref.namespace!))
    .filter((request) => {
      const metadata = request.metadata as {
        ownerReferences?: { controller?: boolean; uid?: string }[];
        deletionTimestamp?: string;
      };
      return (
        metadata.deletionTimestamp === undefined &&
        (metadata.ownerReferences ?? []).some(
          (o) => o.controller === true && o.uid === uid,
        ) &&
        (request.spec as { deletionPolicy?: string } | undefined)
          ?.deletionPolicy !== policy
      );
    })
    .map((request) => request.metadata.name);
  if (behind.length === 0) return undefined;
  return {
    code: "Stale",
    message:
      behind.length === 1
        ? `The request ${behind[0]} does not have the deletion policy yet. The operator writes it within a moment. Try again.`
        : `The requests ${behind.join(", ")} do not have the deletion policy yet. The operator writes it within a moment. Try again.`,
    ref,
  };
}

const CLUSTER_CLAIM_API_VERSION = apiVersionOf(KindMap.CamundaClusterClaim);

/**
 * claimBatchChangeSchema accepts a Change that applies one or more
 * CamundaClusterClaims, each with a namespace, the apiVersion of the kind,
 * and a precondition, no claim twice, and that has no delete, no create, no
 * force, and no directOnly.
 */
export const claimBatchChangeSchema: z.ZodType<Change> = changeSchema.refine(
  (c) => {
    const keys = c.apply.map((o) => refKey(refOf(o)));
    return (
      c.apply.every(
        (o, i) =>
          o.kind === "CamundaClusterClaim" &&
          o.apiVersion === CLUSTER_CLAIM_API_VERSION &&
          o.metadata.namespace !== undefined &&
          c.preconditions?.[keys[i]] !== undefined,
      ) &&
      new Set(keys).size === keys.length &&
      c.delete === undefined &&
      c.create === undefined &&
      c.force === undefined &&
      c.directOnly === undefined
    );
  },
  { message: "The change must apply cluster claims and nothing else." },
);

/**
 * runClaimMove is runChange with claimBatchChangeSchema, for a move of
 * cluster claims to a release. It refuses the whole change and writes
 * nothing when a claim does not exist (Stale), changed since the page read
 * it (Stale), differs from the live claim outside spec.releaseRef,
 * spec.allowVersionDowngrade, status, and the metadata that the API server
 * sets (Invalid), or has a cluster with a backup of either kind that has
 * not ended (Stale, naming the backup). Each such error names the claim in
 * ref. When these checks cannot read the claims or the backups, it gives
 * Unavailable.
 */
export async function runClaimMove(input: unknown): Promise<WriteResult> {
  const parsed = claimBatchChangeSchema.safeParse(input);
  if (!parsed.success) return { ok: false, errors: issuesOf(parsed.error) };
  try {
    const refusals = await moveRefusalsOf(parsed.data);
    if (refusals.length > 0) return { ok: false, errors: refusals };
  } catch (error) {
    return { ok: false, errors: [unavailable(error)] };
  }
  return runChange(parsed.data, claimBatchChangeSchema);
}

/**
 * previewClaimMove is runPreview with claimBatchChangeSchema. It gives no
 * objects and the errors of runClaimMove when that refuses the change.
 */
export async function previewClaimMove(input: unknown): Promise<Preview> {
  const parsed = claimBatchChangeSchema.safeParse(input);
  if (!parsed.success) return { objects: [], errors: issuesOf(parsed.error) };
  try {
    const refusals = await moveRefusalsOf(parsed.data);
    if (refusals.length > 0) return { objects: [], errors: refusals };
  } catch (error) {
    return { objects: [], errors: [unavailable(error)] };
  }
  return runPreview(parsed.data, claimBatchChangeSchema);
}

const MOVE_FIELDS = ["releaseRef", "allowVersionDowngrade"];

async function moveRefusalsOf(change: Change): Promise<ApiError[]> {
  const listed = new Map<string, Promise<KubeObject[]>>();
  const list = (kind: KindName, namespace: string) => {
    const key = `${kind}/${namespace}`;
    if (!listed.has(key)) listed.set(key, listObjects(kind, namespace));
    return listed.get(key)!;
  };
  const errors: ApiError[] = [];
  for (const claim of change.apply) {
    const ref = refOf(claim);
    const namespace = ref.namespace!;
    const live = (await list("CamundaClusterClaim", namespace)).find(
      (c) => c.metadata.name === ref.name,
    );
    if (live === undefined) {
      errors.push({
        code: "Stale",
        message: `The ${ref.kind} ${ref.name} does not exist.`,
        ref,
      });
      continue;
    }
    if (live.metadata.resourceVersion !== change.preconditions?.[refKey(ref)]) {
      errors.push(staleError(ref));
      continue;
    }
    if (!movesOnly(claim, live)) {
      errors.push({
        code: "Invalid",
        message:
          "A move can change only spec.releaseRef and spec.allowVersionDowngrade of a claim.",
        ref,
      });
      continue;
    }
    const backups = (
      await Promise.all(
        (BACKUP_KINDS as KindName[]).map((kind) => list(kind, namespace)),
      )
    ).flat() as unknown as BackupLike[];
    const backup = runningBackup(
      { metadata: { namespace, name: ref.name } },
      backups,
    );
    if (backup !== undefined) {
      const phase = backups.find(
        (b) => b.kind === backup.kind && b.metadata?.name === backup.name,
      )?.status?.phase;
      errors.push({
        code: "Stale",
        message: `Backup ${backup.name} of the cluster is ${phase === "Running" ? "running" : "pending"}. Move the cluster after it ends.`,
        ref,
      });
    }
  }
  return errors;
}

function movesOnly(claim: KubeObject, live: KubeObject): boolean {
  const spec = (claim.spec ?? {}) as Record<string, unknown>;
  const liveSpec = (live.spec ?? {}) as Record<string, unknown>;
  const downgrade = spec.allowVersionDowngrade;
  return (
    typeof spec.releaseRef === "string" &&
    spec.releaseRef !== "" &&
    (equalJSON(downgrade, liveSpec.allowVersionDowngrade) ||
      typeof downgrade === "string") &&
    equalJSON(outsideMove(claim), outsideMove(live))
  );
}

function outsideMove(claim: KubeObject): unknown {
  const metadata = { ...claim.metadata };
  for (const key of SERVER_METADATA) delete metadata[key];
  const spec = { ...((claim.spec ?? {}) as Record<string, unknown>) };
  for (const field of MOVE_FIELDS) delete spec[field];
  return { ...claim, status: undefined, metadata, spec };
}

const SERVER_METADATA = [
  "managedFields",
  "resourceVersion",
  "uid",
  "creationTimestamp",
  "generation",
];

const BACKUP_KINDS: readonly string[] = [
  "LogicalBackupElasticsearch",
  "LogicalBackupRDBMS",
];
const BACKUP_API_VERSION = "core.camunda.io/v1";
const BACKUP_METADATA: readonly string[] = [
  "name",
  "namespace",
  "labels",
  "annotations",
];

/**
 * backupChangeSchema accepts a Change with directOnly, no force, and no
 * preconditions that does one of two things for cluster. It creates one
 * backup or restore in the namespace of cluster, listed in create, with
 * only name, namespace, labels, and annotations in its metadata. The spec
 * of a backup of either kind is exactly { clusterRef: { name } }, of a
 * logical restore of either kind exactly { backupRef: { name },
 * targetClusterRef: { name } }, and of a PointInTimeRestore exactly
 * { clusterRef: { name }, timestamp }, where each cluster name is
 * cluster.name and timestamp is RFC 3339 in UTC to the second. Or it
 * deletes one backup of either kind in that namespace. A delete has no
 * precondition, because the status of a running backup changes its
 * resourceVersion. It can carry the UID of the backup in uids.
 */
export function backupChangeSchema(cluster: {
  namespace: string;
  name: string;
}): z.ZodType<Change> {
  return changeSchema.refine(
    (c) => {
      const plain =
        c.directOnly === true &&
        c.force === undefined &&
        c.preconditions === undefined;
      if (!plain) return false;
      if (c.apply.length === 1 && c.delete === undefined) {
        const object = c.apply[0];
        return (
          isClusterWrite(object, cluster) &&
          c.create?.length === 1 &&
          c.create[0] === refKey(refOf(object))
        );
      }
      const ref = c.delete?.[0];
      return (
        c.apply.length === 0 &&
        c.delete?.length === 1 &&
        c.create === undefined &&
        ref !== undefined &&
        ref.apiVersion === BACKUP_API_VERSION &&
        BACKUP_KINDS.includes(ref.kind) &&
        ref.namespace === cluster.namespace
      );
    },
    {
      message:
        "The change must create one backup or restore of the cluster, or delete one backup in its namespace, straight to the cluster.",
    },
  );
}

/**
 * runBackupChange is runChange with backupChangeSchema(cluster). Each
 * refusal below gives Stale and writes nothing. A new backup fails when a
 * backup of either kind in the namespace has its name, and a new restore
 * when a restore of any kind there has its name. A new restore also fails
 * while a restore of the cluster is not Completed or Failed, and a new
 * logical restore when its backup does not exist, is not Completed, is
 * being deleted, or is a backup of another cluster. A delete fails while a logical restore that
 * is not Completed or Failed reads the backup. When these checks cannot
 * read the backups or the restores, it gives Unavailable and writes
 * nothing.
 */
export async function runBackupChange(
  cluster: { namespace: string; name: string },
  input: unknown,
): Promise<WriteResult> {
  const schema = backupChangeSchema(cluster);
  const parsed = schema.safeParse(input);
  if (!parsed.success) return { ok: false, errors: issuesOf(parsed.error) };
  try {
    const refusal = await backupRefusalOf(cluster, parsed.data);
    if (refusal !== undefined) return { ok: false, errors: [refusal] };
  } catch (error) {
    return { ok: false, errors: [unavailable(error)] };
  }
  return runChange(parsed.data, schema);
}

const RESTORE_OF = {
  LogicalBackupElasticsearch: "LogicalRestoreElasticsearch",
  LogicalBackupRDBMS: "LogicalRestoreRDBMS",
} as const;

const BACKUP_OF = {
  LogicalRestoreElasticsearch: "LogicalBackupElasticsearch",
  LogicalRestoreRDBMS: "LogicalBackupRDBMS",
} as const;

const RESTORE_KINDS = [
  "LogicalRestoreElasticsearch",
  "LogicalRestoreRDBMS",
  "PointInTimeRestore",
] as const;

type Stored = KubeObject & {
  spec?: {
    backupRef?: { name?: string };
    clusterRef?: { name?: string };
    targetClusterRef?: { name?: string };
  };
  status?: { phase?: string };
};

async function listStored(
  kind: Parameters<typeof listObjects>[0],
  namespace: string,
): Promise<Stored[]> {
  return (await listObjects(kind, namespace)) as Stored[];
}

function ended(object: Stored): boolean {
  const phase = object.status?.phase;
  return phase === "Completed" || phase === "Failed";
}

async function backupRefusalOf(
  cluster: { namespace: string; name: string },
  change: Change,
): Promise<ApiError | undefined> {
  const { namespace } = cluster;
  const [object] = change.apply;
  if (object !== undefined && !BACKUP_KINDS.includes(object.kind)) {
    return restoreRefusalOf(cluster, object as Stored);
  }
  if (object !== undefined) {
    const taken = await firstExisting(
      BACKUP_KINDS.map((kind) => ({
        apiVersion: BACKUP_API_VERSION,
        kind,
        namespace,
        name: object.metadata.name,
      })),
    );
    return taken === undefined ? undefined : existsError(taken);
  }
  const ref = change.delete![0];
  const restores = await listStored(
    RESTORE_OF[ref.kind as keyof typeof RESTORE_OF],
    namespace,
  );
  const reader = restores.find(
    (r) => r.spec?.backupRef?.name === ref.name && !ended(r),
  );
  return reader === undefined
    ? undefined
    : {
        code: "Stale",
        message: `Restore ${reader.metadata.name} reads the backup ${ref.name}. Delete the backup after the restore ends.`,
        ref,
      };
}

async function restoreRefusalOf(
  cluster: { namespace: string; name: string },
  restore: Stored,
): Promise<ApiError | undefined> {
  const { namespace } = cluster;
  const ref = refOf(restore);
  const restores = (
    await Promise.all(RESTORE_KINDS.map((kind) => listStored(kind, namespace)))
  ).flat();
  const taken = restores.find((r) => r.metadata.name === restore.metadata.name);
  if (taken !== undefined) return existsError(refOf(taken));
  const running = restores.find((r) => {
    const target =
      r.kind === "PointInTimeRestore"
        ? r.spec?.clusterRef?.name
        : r.spec?.targetClusterRef?.name;
    return target === cluster.name && !ended(r);
  });
  if (running !== undefined) {
    return {
      code: "Stale",
      message: `Restore ${running.metadata.name} of this cluster has not ended. Start a new restore after it ends.`,
      ref,
    };
  }
  if (!Object.hasOwn(BACKUP_OF, restore.kind)) return undefined;
  const backupKind = BACKUP_OF[restore.kind as keyof typeof BACKUP_OF];
  const name = restore.spec?.backupRef?.name;
  const backups = await listStored(backupKind, namespace);
  const backup = backups.find((b) => b.metadata.name === name);
  const of = backup?.spec?.clusterRef?.name;
  const phase = backup?.status?.phase || "Pending";
  let message: string | undefined;
  if (backup === undefined) message = `Backup ${name} does not exist.`;
  else if (of !== cluster.name) {
    message = `Backup ${name} is a backup of the cluster ${of}. A restore writes only into the cluster of its backup.`;
  } else if (phase !== "Completed") {
    message = `Backup ${name} is ${phase}, and a restore reads only a Completed backup.`;
  } else if (backup.metadata.deletionTimestamp) {
    message = `Backup ${name} is being deleted, so a restore cannot read it.`;
  }
  return message === undefined ? undefined : { code: "Stale", message, ref };
}

function isClusterWrite(
  object: KubeObject,
  cluster: { namespace: string; name: string },
): boolean {
  const { apiVersion, kind, metadata, spec, ...rest } = object;
  return (
    Object.keys(rest).length === 0 &&
    apiVersion === BACKUP_API_VERSION &&
    metadata.namespace === cluster.namespace &&
    Object.keys(metadata).every((key) => BACKUP_METADATA.includes(key)) &&
    specFits(kind, spec, cluster.name)
  );
}

// SECOND_UTC is the form of an RFC 3339 time in UTC to the second, the form
// in which the page writes the timestamp of a PointInTimeRestore.
const SECOND_UTC = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/;

function specFits(kind: string, spec: unknown, cluster: string): boolean {
  const ref = { name: cluster };
  if (BACKUP_KINDS.includes(kind)) return equalJSON(spec, { clusterRef: ref });
  const fields = spec as
    { backupRef?: { name?: unknown }; timestamp?: unknown } | undefined;
  if (Object.hasOwn(BACKUP_OF, kind)) {
    const name = fields?.backupRef?.name;
    return (
      typeof name === "string" &&
      name !== "" &&
      equalJSON(spec, { backupRef: { name }, targetClusterRef: ref })
    );
  }
  if (kind !== "PointInTimeRestore") return false;
  const timestamp = fields?.timestamp;
  return (
    typeof timestamp === "string" &&
    SECOND_UTC.test(timestamp) &&
    !Number.isNaN(Date.parse(timestamp)) &&
    new Date(timestamp).toISOString().startsWith(timestamp.slice(0, 19)) &&
    equalJSON(spec, { clusterRef: ref, timestamp })
  );
}

/**
 * createChangeSchema accepts a Change that applies one claim of either
 * kind with a namespace, optionally after the Namespace of that namespace,
 * whose create lists the key of each applied object, and that has no
 * delete, no preconditions, no force, and no directOnly.
 */
export const createChangeSchema: z.ZodType<Change> = changeSchema.refine(
  (c) => {
    const claim = c.apply.at(-1);
    const namespace = claim?.metadata.namespace;
    if (
      claim === undefined ||
      !CLAIM_KINDS.includes(claim.kind) ||
      namespace === undefined
    ) {
      return false;
    }
    const shapeOK =
      c.apply.length === 1 ||
      (c.apply.length === 2 &&
        c.apply[0].kind === "Namespace" &&
        c.apply[0].metadata.name === namespace);
    const keys = c.apply.map((o) => refKey(refOf(o)));
    return (
      shapeOK &&
      c.create?.length === keys.length &&
      keys.every((key) => c.create!.includes(key)) &&
      c.delete === undefined &&
      c.preconditions === undefined &&
      c.force === undefined &&
      c.directOnly === undefined
    );
  },
  {
    message:
      "The change must create one claim, and at most its new namespace first.",
  },
);

/** createClaim writes a create change: runChange with createChangeSchema. */
export async function createClaim(input: unknown): Promise<WriteResult> {
  return runChange(input, createChangeSchema);
}

/**
 * adminChangeSchema accepts a Change of a Setup page. It allows deletes,
 * preconditions, create, and force, under these rules. Each object that it
 * applies or deletes has a kind of kinds, with the apiVersion of that
 * kind. An object of a cluster-scoped kind has no namespace, and an object
 * of a namespaced kind has a namespace of namespaces. Each delete, and each
 * apply that create does not list, has a precondition. force is allowed
 * only on a change whose applies are all updates and that deletes nothing.
 * An applied object holds no status, and its metadata holds only name,
 * namespace, labels, and annotations. directOnly is refused.
 */
export function adminChangeSchema(
  kinds: readonly KindName[],
  namespaces: readonly string[] = [],
): z.ZodType<Change> {
  const placed = (o: {
    kind: string;
    apiVersion: string;
    namespace?: string;
  }) => {
    if (!(kinds as readonly string[]).includes(o.kind)) return false;
    const info = KindMap[o.kind as KindName];
    if (o.apiVersion !== apiVersionOf(info)) return false;
    return info.scope === "Cluster"
      ? o.namespace === undefined
      : o.namespace !== undefined && namespaces.includes(o.namespace);
  };
  return changeSchema.refine(
    (c) => {
      const created = c.create ?? [];
      const guarded = (key: RefKey) => c.preconditions?.[key] !== undefined;
      return (
        c.apply.every(
          (o) =>
            placed({ ...o, namespace: o.metadata.namespace }) &&
            o.status === undefined &&
            onlyKeys(o.metadata, [
              "name",
              "namespace",
              "labels",
              "annotations",
            ]) &&
            (created.includes(refKey(refOf(o))) || guarded(refKey(refOf(o)))),
        ) &&
        (c.delete ?? []).every((r) => placed(r) && guarded(refKey(r))) &&
        (c.force === undefined ||
          (created.length === 0 && (c.delete ?? []).length === 0)) &&
        c.directOnly === undefined
      );
    },
    { message: `The change must write only ${kinds.join(", ")}.` },
  );
}

/**
 * namespaceLabelChangeSchema accepts a Change that applies exactly one
 * Namespace, with a precondition on that Namespace, with directOnly true,
 * and with no delete and no force. Besides metadata.name, the Namespace
 * holds only string maps of labels and annotations. The Namespace may
 * carry the resourceVersion that the page read. The label
 * ENVIRONMENT_LABEL has a non-empty string value.
 */
export const namespaceLabelChangeSchema: z.ZodType<Change> =
  changeSchema.refine(
    (c) =>
      c.apply.length === 1 &&
      onlyLabelsAndAnnotations(c.apply[0]) &&
      c.preconditions?.[refKey(refOf(c.apply[0]))] !== undefined &&
      c.directOnly === true &&
      c.delete === undefined &&
      c.force === undefined,
    {
      message:
        "The change must write only the labels and annotations of one namespace.",
    },
  );

function onlyLabelsAndAnnotations(object: KubeObject): boolean {
  const { metadata } = object;
  const labels = metadata.labels ?? {};
  return (
    object.apiVersion === "v1" &&
    object.kind === "Namespace" &&
    onlyKeys(object, ["apiVersion", "kind", "metadata"]) &&
    onlyKeys(metadata, ["name", "labels", "annotations", "resourceVersion"]) &&
    isStringMap(labels) &&
    isStringMap(metadata.annotations ?? {}) &&
    typeof (labels as Record<string, unknown>)[ENVIRONMENT_LABEL] ===
      "string" &&
    (labels as Record<string, string>)[ENVIRONMENT_LABEL] !== ""
  );
}

function isStringMap(value: unknown): boolean {
  return (
    typeof value === "object" &&
    value !== null &&
    !Array.isArray(value) &&
    Object.values(value).every((v) => typeof v === "string")
  );
}

function onlyKeys(object: object, keys: readonly string[]): boolean {
  return Object.keys(object).every((key) => keys.includes(key));
}

/**
 * runChange is the one helper that a server action calls to write. It
 * checks input with schema, applies it with the writer of getWriter, and
 * returns the result. It never throws: input that schema refuses gives one
 * Invalid per issue, and a writer that throws gives Unavailable.
 */
export async function runChange(
  input: unknown,
  schema: z.ZodType<Change> = changeSchema,
): Promise<WriteResult> {
  const parsed = schema.safeParse(input);
  if (!parsed.success) return { ok: false, errors: issuesOf(parsed.error) };
  try {
    return await (await getWriter()).apply(parsed.data);
  } catch (error) {
    return { ok: false, errors: [unavailable(error)] };
  }
}

/**
 * runPreview is the one helper that a server action calls for a dry run.
 * It checks input as runChange does and previews it with the writer of
 * getWriter. It never throws: a refused input or a writer that throws gives
 * no objects and the errors of runChange.
 */
export async function runPreview(
  input: unknown,
  schema: z.ZodType<Change> = changeSchema,
): Promise<Preview> {
  const parsed = schema.safeParse(input);
  if (!parsed.success) return { objects: [], errors: issuesOf(parsed.error) };
  try {
    return await (await getWriter()).preview(parsed.data);
  } catch (error) {
    return { objects: [], errors: [unavailable(error)] };
  }
}

/** issuesOf gives one Invalid error for each issue of a refused input, as runChange and runPreview return them. */
export function issuesOf(error: z.ZodError): ApiError[] {
  return error.issues.map((issue) => ({
    code: "Invalid",
    message: issue.message,
    ...(issue.path.length === 0 ? {} : { path: issue.path.join(".") }),
  }));
}

function unavailable(error: unknown): ApiError {
  return {
    code: "Unavailable",
    message: error instanceof Error ? error.message : String(error),
  };
}
