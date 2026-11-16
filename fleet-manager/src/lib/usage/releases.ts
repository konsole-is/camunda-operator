// This module runs in the browser and on the server.
import type { V1Namespace } from "@kubernetes/client-node";
import type { Environment } from "@/generated/crds/environment";
import { byCodeUnit, environmentOf } from "@/lib/policy";
import { allowedIn } from "./environments";

/** ReleaseUser is an object that names a release in spec.releaseRef. */
export type ReleaseUser = {
  metadata: { namespace?: string; name?: string };
  spec?: { releaseRef?: string };
};

/** ReleaseUsageSources holds the objects of each kind that can use a release. */
export type ReleaseUsageSources = {
  clusters: readonly ReleaseUser[];
  elasticsearchClusters: readonly ReleaseUser[];
  databaseServers: readonly ReleaseUser[];
  claims: readonly ReleaseUser[];
};

/** ReleaseUsage holds the <namespace>/<name> of each user of one release, by kind, sorted by code unit. */
export type ReleaseUsage = {
  clusters: string[];
  elasticsearchClusters: string[];
  databaseServers: string[];
  claims: string[];
};

/**
 * releaseUsage returns the objects of each kind whose spec.releaseRef names
 * release. A CamundaClusterClaim counts, so a release has a user before the
 * cluster of a new claim exists. An empty release name matches nothing.
 */
export function releaseUsage(
  release: string,
  sources: ReleaseUsageSources,
): ReleaseUsage {
  const users = (objects: readonly ReleaseUser[]) =>
    release === ""
      ? []
      : objects
          .filter((object) => object.spec?.releaseRef === release)
          .map(
            (object) =>
              `${object.metadata.namespace ?? ""}/${object.metadata.name ?? ""}`,
          )
          .sort(byCodeUnit);
  return {
    clusters: users(sources.clusters),
    elasticsearchClusters: users(sources.elasticsearchClusters),
    databaseServers: users(sources.databaseServers),
    claims: users(sources.claims),
  };
}

/** releaseUnused says whether usage holds no user of any kind, claims included. */
export function releaseUnused(usage: ReleaseUsage): boolean {
  return (
    usage.clusters.length === 0 &&
    usage.elasticsearchClusters.length === 0 &&
    usage.databaseServers.length === 0 &&
    usage.claims.length === 0
  );
}

/**
 * releaseSecretNamespaces returns the namespaces whose Secrets an env entry
 * of release can name, distinct and sorted. They are the namespaces of its
 * users. A release without a user gives the namespaces of the Environments
 * that allow it, which is empty when none allows it.
 */
export function releaseSecretNamespaces(
  release: string,
  sources: ReleaseUsageSources & {
    environments: readonly Environment[];
    namespaces: readonly V1Namespace[];
  },
): string[] {
  const usage = releaseUsage(release, sources);
  let names = [
    ...usage.clusters,
    ...usage.elasticsearchClusters,
    ...usage.databaseServers,
    ...usage.claims,
  ].map((ref) => ref.slice(0, ref.indexOf("/")));
  if (names.length === 0) {
    const allowing = new Set(
      allowedIn(sources.environments, "CamundaRelease", release),
    );
    names = sources.namespaces
      .filter((namespace) => {
        const name = environmentOf(namespace, sources.environments).environment
          ?.metadata.name;
        return name !== undefined && allowing.has(name);
      })
      .map((namespace) => namespace.metadata?.name ?? "");
  }
  return [...new Set(names)].sort(byCodeUnit);
}
