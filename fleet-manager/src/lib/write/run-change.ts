import "server-only";
import { z } from "zod";
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

/** changeSchema accepts a Change with an intent and at least one object to apply or delete. */
export const changeSchema: z.ZodType<Change> = z
  .object({
    intent: z.string().min(1),
    apply: z.array(kubeObjectSchema),
    delete: z.array(objectRefSchema).optional(),
    preconditions: z
      .record(
        z.templateLiteral([z.string(), "/", z.string(), "/", z.string()]),
        z.string(),
      )
      .optional(),
    force: z.boolean().optional(),
    directOnly: z.boolean().optional(),
  })
  .refine((c) => c.apply.length + (c.delete?.length ?? 0) > 0, {
    message: "The change has no object to apply or delete.",
  });

const CLAIM_KINDS: readonly string[] = [
  "CamundaClusterClaim",
  "CamundaManagementClusterClaim",
];

/**
 * claimChangeSchema accepts a Change that applies exactly one claim of a
 * cluster or a management plane, with a namespace, and that has no delete,
 * no force, and no directOnly.
 */
export const claimChangeSchema: z.ZodType<Change> = changeSchema.refine(
  (c) =>
    c.apply.length === 1 &&
    CLAIM_KINDS.includes(c.apply[0].kind) &&
    c.apply[0].metadata.namespace !== undefined &&
    c.delete === undefined &&
    c.force === undefined &&
    c.directOnly === undefined,
  { message: "The change must apply one claim and nothing else." },
);

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
