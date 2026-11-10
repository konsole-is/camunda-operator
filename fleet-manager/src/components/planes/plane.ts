// This module runs in the browser and on the server. It imports only the
// isomorphic labels and policy modules and types.
import type { ConditionLike } from "@/components/fleet/status";
import type { Resource } from "@/lib/k8s/kinds";
import {
  CLAIM_LABEL,
  isManaged,
  type LabelSelector,
  matchesSelector,
} from "@/lib/labels";
import { environmentNameOf } from "@/lib/policy";

export type IdentityMode = "Keycloak" | "External Keycloak" | "OIDC";

/** PlaneSources are the lists that planesOf reads. */
export type PlaneSources = {
  clusters: Resource<"CamundaManagementCluster">[];
  claims: Resource<"CamundaManagementClusterClaim">[];
  namespaces: Resource<"Namespace">[];
  environments: Resource<"Environment">[];
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
 * distinct. servesNoEnvironment is true when a plane without a claim
 * serves a namespace that belongs to no Environment. ready is
 * undefined while no CamundaManagementCluster exists.
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
  servesNoEnvironment?: boolean;
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
 * is not in claims stays managed and serves its selector. Namespaces and
 * Environments decide the servedEnvironments of a plane without a claim.
 * While spec.clusterSelector is unset, a plane without a claim has no
 * servedEnvironments, and servesNoEnvironment is false.
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
            ...selected(cluster.spec, sources),
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
): Pick<Plane, "serves" | "servedEnvironments" | "servesNoEnvironment"> {
  return {
    serves: { environments: names },
    servedEnvironments: [...new Set(names)].sort(),
    servesNoEnvironment: false,
  };
}

function selected(
  spec: {
    clusterSelector?: LabelSelector;
    namespaceSelector?: LabelSelector;
  },
  sources: PlaneSources,
): Pick<Plane, "servedEnvironments" | "servesNoEnvironment"> {
  const names = new Set<string>();
  let none = false;
  // The operator selects no cluster while clusterSelector is unset.
  if (spec.clusterSelector) {
    for (const namespace of sources.namespaces) {
      if (!matchesSelector(spec.namespaceSelector, namespace.metadata?.labels))
        continue;
      const environment = environmentNameOf(namespace, sources.environments);
      if (environment === undefined) none = true;
      else names.add(environment);
    }
  }
  return { servedEnvironments: [...names].sort(), servesNoEnvironment: none };
}

/**
 * planesIn returns the planes of one fleet tab: every plane for undefined,
 * the planes with servesNoEnvironment for null, else the planes that serve
 * the environment of that name.
 */
export function planesIn(
  planes: readonly Plane[],
  environment?: string | null,
): Plane[] {
  if (environment === undefined) return [...planes];
  if (environment === null) {
    return planes.filter((plane) => plane.servesNoEnvironment === true);
  }
  return planes.filter((plane) => servesEnvironment(plane, environment));
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
