import {
  type FieldSchema,
  type FullSchemaName,
  type SchemaRoot,
  fieldSchemas,
  fullSchemas,
} from "@/generated/fieldschema";
import type { OpenAPISchema } from "@/lib/schema";
import { REFUSED_CLUSTER_FIELDS } from "./claim";
import { COMPONENTS } from "./cluster";
import type { FieldGroup } from "./types";

/** SchemaGroup is a FieldGroup whose fields the field schema describes. */
export type SchemaGroup = Exclude<FieldGroup, "backupSchedule" | "parameters">;

/** fieldSchemaOf returns the schema of path under root, or undefined. It reads own properties only. */
export function fieldSchemaOf(
  root: SchemaRoot,
  path: string,
): FieldSchema | undefined {
  const fields = fieldSchemas[root];
  return Object.hasOwn(fields, path) ? fields[path] : undefined;
}

/** fullSchemaOf returns the whole CRD schema of a field named by FullSchemaName at the end of path, or undefined. */
export function fullSchemaOf(path: string): OpenAPISchema | undefined {
  const name = path.slice(path.lastIndexOf(".") + 1);
  return Object.hasOwn(fullSchemas, name)
    ? fullSchemas[name as FullSchemaName]
    : undefined;
}

const REFUSED_PATHS = REFUSED_CLUSTER_FIELDS.map((row) => row.names.join("."));

/**
 * componentFieldPaths returns the paths of the fields that group offers
 * when no layer sets them, sorted: each field of the field schema under the
 * group block (spec.cluster.<group> for a process, the top level of
 * spec.cluster outside the six process blocks for "cluster", and
 * spec.attachments.optimize for "optimize"), without the fields of
 * REFUSED_CLUSTER_FIELDS. A path is under its root, for example
 * "tasklist.replicas" or "importer.replicas". A block that holds other
 * fields, such as "tasklist.scheduling", is not a field of the result.
 */
export function componentFieldPaths(group: SchemaGroup): string[] {
  const root: SchemaRoot = group === "optimize" ? "optimize" : "cluster";
  const paths = Object.keys(fieldSchemas[root]);
  const fields = paths.filter(
    (path) => !paths.some((other) => other.startsWith(`${path}.`)),
  );
  return fields
    .filter((path) => inGroup(group, path))
    .filter(
      (path) =>
        root !== "cluster" ||
        !REFUSED_PATHS.some(
          (refused) => path === refused || path.startsWith(`${refused}.`),
        ),
    )
    .sort();
}

function inGroup(group: SchemaGroup, path: string): boolean {
  const first = path.split(".")[0] as FieldGroup;
  if (group === "optimize") return true;
  if (group === "cluster") return !COMPONENTS.includes(first);
  return first === group;
}
