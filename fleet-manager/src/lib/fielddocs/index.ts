import { type FieldDoc, fieldDocs } from "@/generated/fielddocs";

const DOCS_SITE = "https://konsole-is.github.io/camunda-operator/";

/**
 * fieldDocFor returns the doc of a field path of a kind, or undefined when
 * the group, the kind, or the path has no doc, or its description is empty.
 * It reads own properties only, so a path such as constructor gives
 * undefined. The map is large: call this on the server and pass the doc to
 * the client component.
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
 * apiReferenceUrl links an anchor of the API reference in the docs of an
 * operator version. A release vX.Y.Z has the docs of its minor, X.Y. Any
 * other version, such as a development build or a release candidate, links
 * the docs of main, dev.
 */
export function apiReferenceUrl(
  operatorVersion: string,
  anchor: string,
): string {
  const release = /^v?(\d+)\.(\d+)\.\d+$/.exec(operatorVersion);
  const docsVersion = release ? `${release[1]}.${release[2]}` : "dev";
  return `${DOCS_SITE}${docsVersion}/crds/api-reference/${anchor}`;
}

function own<T>(
  record: Partial<Record<string, T>>,
  key: string,
): T | undefined {
  return Object.hasOwn(record, key) ? record[key] : undefined;
}
