// This module holds what a client component needs. It does not import the
// field docs map, so the map stays out of the browser bundle.

const DOCS_SITE = "https://konsole-is.github.io/camunda-operator/";

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
