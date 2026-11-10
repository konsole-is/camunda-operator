import "server-only";
import { z } from "zod";
import { refKey, refOf } from "./refs";
import type {
  ApiError,
  Change,
  KubeObject,
  ObjectRef,
  Preview,
  WriteResult,
} from "./types";
import { getWriter } from "./writer";

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
