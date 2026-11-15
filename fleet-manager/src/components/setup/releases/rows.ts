import { byCodeUnit } from "@/lib/policy";
import {
  allowedIn,
  defaultIn,
  type ReleaseUsage,
  releaseUnused,
  releaseUsage,
} from "@/lib/usage";
import { environmentLabel } from "./environments";
import type { ReleasesData } from "./sources";

/** EnvironmentName is the name of an environment and the label that the page shows. */
export type EnvironmentName = { name: string; label: string };

/**
 * ReleaseRow is one release of the Releases list. The optional versions are
 * unset when the release does not set them. deletable is true only for a
 * release that nothing uses, claims included, and that no environment
 * allows or has as its default.
 */
export type ReleaseRow = {
  name: string;
  camunda: string;
  connectors?: string;
  elasticsearch?: string;
  postgres?: string;
  defaultIn: EnvironmentName[];
  allowedIn: EnvironmentName[];
  usage: ReleaseUsage;
  deletable: boolean;
};

/** releaseRows returns one row for each release of sources, sorted by name. */
export function releaseRows(sources: ReleasesData): ReleaseRow[] {
  const names = (environments: string[]) =>
    environments.map((name) => {
      const environment = sources.environments.find(
        (e) => e.metadata.name === name,
      );
      return { name, label: environment ? environmentLabel(environment) : name };
    });
  return sources.releases
    .map((release) => {
      const name = release.metadata.name ?? "";
      const spec = release.spec;
      const usage = releaseUsage(name, sources);
      const defaults = names(
        defaultIn(sources.environments, "CamundaRelease", name),
      );
      const allowed = names(
        allowedIn(sources.environments, "CamundaRelease", name),
      );
      return {
        name,
        camunda: spec.version,
        ...(spec.connectors?.version && {
          connectors: spec.connectors.version,
        }),
        ...(spec.elasticsearch?.version && {
          elasticsearch: spec.elasticsearch.version,
        }),
        ...(spec.databaseServer?.version && {
          postgres: spec.databaseServer.version,
        }),
        defaultIn: defaults,
        allowedIn: allowed,
        usage,
        deletable:
          releaseUnused(usage) && allowed.length === 0 && defaults.length === 0,
      };
    })
    .sort((a, b) => byCodeUnit(a.name, b.name));
}

/**
 * usedByText counts the clusters, Elasticsearch clusters, and database
 * servers of usage. When none of them exists, it counts the claims, and
 * says "Unused" when there is no claim either.
 */
export function usedByText(usage: ReleaseUsage): string {
  const parts = [
    count(usage.clusters.length, "cluster"),
    count(usage.elasticsearchClusters.length, "Elasticsearch cluster"),
    count(usage.databaseServers.length, "database server"),
  ].filter((part) => part !== undefined);
  if (parts.length > 0) return parts.join(", ");
  return count(usage.claims.length, "claim") ?? "Unused";
}

/** defaultInText is the "Default in" fact of row, or undefined when no environment has it as default. */
export function defaultInText(row: ReleaseRow): string | undefined {
  return row.defaultIn.length > 0
    ? `Default in ${labels(row.defaultIn)}`
    : undefined;
}

/** allowedInText is the "Allowed in" fact of row, or "Not offered to new clusters" when no environment allows it. */
export function allowedInText(row: ReleaseRow): string {
  return row.allowedIn.length > 0
    ? `Allowed in ${labels(row.allowedIn)}`
    : "Not offered to new clusters";
}

function labels(environments: EnvironmentName[]): string {
  return environments.map((e) => e.label).join(", ");
}

function count(n: number, noun: string): string | undefined {
  if (n === 0) return undefined;
  return `${n} ${noun}${n === 1 ? "" : "s"}`;
}
