// This module runs in the browser and on the server. It imports only the
// isomorphic labels module and types.
import type { ConditionLike } from "@/components/fleet/status";
import type { Resource } from "@/lib/k8s/kinds";
import {
  CLAIM_LABEL,
  ENVIRONMENT_LABEL,
  isManaged,
  type LabelSelector,
  matchesSelector,
} from "@/lib/labels";

export type IdentityMode = "Keycloak" | "External Keycloak" | "OIDC";

/** PlaneSources are the lists that planesOf reads. */
export type PlaneSources = {
  clusters: Resource<"CamundaManagementCluster">[];
  claims: Resource<"CamundaManagementClusterClaim">[];
  namespaces: Resource<"Namespace">[];
};

/** NotAttached is an orchestration cluster that a plane does not serve, and why. */
export type NotAttached = {
  namespace: string;
  name: string;
  reason?: string;
  message?: string;
};

/**
 * Plane is one row of the management plane tables. components starts with
 * "Identity". serves holds the environments of the claim, or the selector
 * text when the plane has no claim. servedEnvironments is sorted and
 * distinct. ready is undefined while no CamundaManagementCluster exists.
 */
export type Plane = {
  id: string;
  namespace: string;
  name: string;
  managed: boolean;
  mode?: IdentityMode;
  components: string[];
  serves: { environments: string[] } | { selector: string };
  servedEnvironments: string[];
  attached: number;
  notAttached: NotAttached[];
  optimize: number;
  contract?: string;
  ready?: ConditionLike;
};

type Settings = {
  identityProvider?: {
    keycloak?: unknown;
    externalKeycloak?: unknown;
    oidc?: unknown;
  };
  console?: unknown;
  webModeler?: unknown;
};

/**
 * planesOf returns one plane for each CamundaManagementCluster, and one for
 * each claim that no CamundaManagementCluster of its namespace names with
 * CLAIM_LABEL, sorted by namespace and name. A labelled cluster whose claim
 * is not in claims stays managed and serves its selector. Namespaces decide
 * the servedEnvironments of a plane without a claim.
 */
export function planesOf(sources: PlaneSources): Plane[] {
  const claims = new Map(
    sources.claims.map((c) => [
      key(c.metadata?.namespace, c.metadata?.name),
      c,
    ]),
  );
  const joined = new Set<string>();
  const planes = sources.clusters.map((cluster) => {
    const namespace = cluster.metadata?.namespace ?? "";
    const claimName = cluster.metadata?.labels?.[CLAIM_LABEL];
    const claim =
      claimName === undefined
        ? undefined
        : claims.get(key(namespace, claimName));
    if (claim) joined.add(key(namespace, claimName));
    const status = cluster.status;
    const rows = status?.clusters ?? [];
    return {
      id: `cluster/${key(namespace, cluster.metadata?.name)}`,
      namespace,
      name: cluster.metadata?.name ?? "",
      managed: isManaged(cluster),
      ...settings(cluster.spec),
      ...(claim
        ? environments(claim.spec.environments)
        : {
            serves: {
              selector: formatSelector(cluster.spec.namespaceSelector),
            },
            servedEnvironments: selected(
              cluster.spec.namespaceSelector,
              sources.namespaces,
            ),
          }),
      attached: rows.filter((r) => r.attached).length,
      notAttached: rows
        .filter((r) => !r.attached)
        .map(({ namespace, name, reason, message }) => ({
          namespace,
          name,
          ...(reason === undefined ? {} : { reason }),
          ...(message === undefined ? {} : { message }),
        })),
      optimize: status?.optimize?.length ?? 0,
      contract: status?.managementAuthConfig,
      ready: status?.conditions?.find((c) => c.type === "Ready"),
    } satisfies Plane;
  });
  const claimOnly = sources.claims
    .filter((c) => !joined.has(key(c.metadata?.namespace, c.metadata?.name)))
    .map((claim): Plane => ({
      id: `claim/${key(claim.metadata?.namespace, claim.metadata?.name)}`,
      namespace: claim.metadata?.namespace ?? "",
      name: claim.metadata?.name ?? "",
      managed: true,
      ...settings(claim.spec.management),
      ...environments(claim.spec.environments),
      attached: 0,
      notAttached: [],
      optimize: 0,
      contract: undefined,
      ready: undefined,
    }));
  return [...planes, ...claimOnly].sort(
    (a, b) =>
      a.namespace.localeCompare(b.namespace) || a.name.localeCompare(b.name),
  );
}

function key(namespace: string | undefined, name: string | undefined) {
  return `${namespace ?? ""}/${name ?? ""}`;
}

function settings(spec: Settings): Pick<Plane, "mode" | "components"> {
  const provider = spec.identityProvider ?? {};
  const mode: IdentityMode | undefined = provider.keycloak
    ? "Keycloak"
    : provider.externalKeycloak
      ? "External Keycloak"
      : provider.oidc
        ? "OIDC"
        : undefined;
  const components = ["Identity"];
  if (spec.console) components.push("Console");
  if (spec.webModeler) components.push("Web Modeler");
  return { mode, components };
}

function environments(
  names: string[],
): Pick<Plane, "serves" | "servedEnvironments"> {
  return {
    serves: { environments: names },
    servedEnvironments: [...new Set(names)].sort(),
  };
}

function selected(
  selector: LabelSelector | undefined,
  namespaces: Resource<"Namespace">[],
): string[] {
  const names = new Set<string>();
  for (const namespace of namespaces) {
    const labels = namespace.metadata?.labels;
    const environment = labels?.[ENVIRONMENT_LABEL];
    if (environment !== undefined && matchesSelector(selector, labels)) {
      names.add(environment);
    }
  }
  return [...names].sort();
}

/** servesEnvironment says whether plane serves the environment of that name. */
export function servesEnvironment(plane: Plane, environment: string): boolean {
  return plane.servedEnvironments.includes(environment);
}

/**
 * formatSelector returns selector in the form of kubectl, such as
 * "team=x, env in (prod,dev), !legacy", or "Every namespace" when it is
 * unset or empty.
 */
export function formatSelector(selector: LabelSelector | undefined): string {
  const labels = Object.entries(selector?.matchLabels ?? {})
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => `${k}=${v}`);
  const expressions = (selector?.matchExpressions ?? []).map(
    ({ key, operator, values = [] }) => {
      switch (operator) {
        case "In":
          return `${key} in (${values.join(",")})`;
        case "NotIn":
          return `${key} notin (${values.join(",")})`;
        case "Exists":
          return key;
        case "DoesNotExist":
          return `!${key}`;
        default:
          return `${key} ${operator} (${values.join(",")})`;
      }
    },
  );
  const parts = [...labels, ...expressions];
  return parts.length === 0 ? "Every namespace" : parts.join(", ");
}

/** optimizeLabel is the Optimize count of plane as text, or "0 (OIDC)" in OIDC mode. */
export function optimizeLabel(plane: Plane): string {
  return plane.mode === "OIDC" ? "0 (OIDC)" : String(plane.optimize);
}
