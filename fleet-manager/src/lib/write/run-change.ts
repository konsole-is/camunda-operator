import "server-only";
import { z } from "zod";
import type { Change, KubeObject, ObjectRef, WriteResult } from "./types";
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
    directOnly: z.boolean().optional(),
  })
  .refine((c) => c.apply.length + (c.delete?.length ?? 0) > 0, {
    message: "The change has no object to apply or delete.",
  });

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
  if (!parsed.success) {
    return {
      ok: false,
      errors: parsed.error.issues.map((issue) => ({
        code: "Invalid",
        message: issue.message,
        ...(issue.path.length === 0 ? {} : { path: issue.path.join(".") }),
      })),
    };
  }
  try {
    return await (await getWriter()).apply(parsed.data);
  } catch (error) {
    return {
      ok: false,
      errors: [
        {
          code: "Unavailable",
          message: error instanceof Error ? error.message : String(error),
        },
      ],
    };
  }
}
