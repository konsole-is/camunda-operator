import "server-only";
import { z } from "zod";
import { apiVersionOf, KindMap, type KindName } from "@/lib/k8s/kinds";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { equalJSON } from "@/lib/policy";
import { existsError } from "./errors";
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
import { firstExisting, getWriter, listObjects } from "./writer";

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
 * precondition.
 */
export const changeSchema: z.ZodType<Change> = z
  .object({
    intent: z.string().min(1),
    apply: z.array(kubeObjectSchema),
    delete: z.array(objectRefSchema).optional(),
    preconditions: z.record(refKeySchema, z.string()).optional(),
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
 * resourceVersion.
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
  if (!(restore.kind in BACKUP_OF)) return undefined;
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
  if (kind in BACKUP_OF) {
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

function issuesOf(error: z.ZodError): ApiError[] {
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
