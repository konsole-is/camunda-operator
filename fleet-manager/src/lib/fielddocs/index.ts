import "server-only";
import { type FieldDoc, fieldDocs } from "@/generated/fielddocs";

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

function own<T>(
  record: Partial<Record<string, T>>,
  key: string,
): T | undefined {
  return Object.hasOwn(record, key) ? record[key] : undefined;
}
