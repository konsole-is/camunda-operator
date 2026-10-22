import "server-only";
import { type FieldDoc, fieldDocs } from "@/generated/fielddocs";
import type { ClusterField } from "@/lib/fields";

export { apiReferenceUrl } from "./link";

/**
 * fieldDocFor returns the doc of a field path of a kind, or undefined when
 * the group, the kind, or the path has no doc, or its description is empty.
 * It reads own properties only, so a path such as constructor gives
 * undefined. The map is large, so this module is server-only: pass the doc
 * to the client component.
 */
export function fieldDocFor(
  group: string,
  kind: string,
  path: string,
): FieldDoc | undefined {
  const kinds = own(fieldDocs, group);
  const paths = kinds && own(kinds, kind);
  const doc = paths && own(paths, path);
  return doc?.description ? doc : undefined;
}

/**
 * nearestFieldDoc returns the doc of the longest prefix of names that has
 * one, for example spec.podLabels for spec.podLabels.team. names are JSON
 * field names from the root, without list items.
 */
export function nearestFieldDoc(
  group: string,
  kind: string,
  names: readonly string[],
): FieldDoc | undefined {
  for (let n = names.length; n > 0; n--) {
    const doc = fieldDocFor(group, kind, names.slice(0, n).join("."));
    if (doc) return doc;
  }
  return undefined;
}

/**
 * fieldDocsOf returns the doc of each field with docNames, by field key. A
 * field without docNames, such as a section parameter, or without a doc,
 * has no entry.
 */
export function fieldDocsOf(
  fields: readonly ClusterField[],
): Record<string, FieldDoc> {
  const docs: Record<string, FieldDoc> = {};
  for (const field of fields) {
    if (!field.docNames) continue;
    const doc = nearestFieldDoc(
      "core.camunda.io",
      field.target.kind,
      field.docNames,
    );
    if (doc) docs[field.key] = doc;
  }
  return docs;
}

function own<T>(
  record: Partial<Record<string, T>>,
  key: string,
): T | undefined {
  return Object.hasOwn(record, key) ? record[key] : undefined;
}
