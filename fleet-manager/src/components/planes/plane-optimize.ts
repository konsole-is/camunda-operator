// This module runs in the browser and on the server.
import type { Resource } from "@/lib/k8s/kinds";
import { byCodeUnit } from "@/lib/policy";

/** OPTIMIZE_CALLBACK_PATH is the path that Optimize completes a login at. It is OptimizeCallbackPath of pkg/components/camundamanagementcluster. */
export const OPTIMIZE_CALLBACK_PATH = "/api/authentication/callback";

/**
 * OptimizeRow is one Optimize whose login callback the plane needs.
 * externalUrl and callbackUrl are undefined when the external URL is not a
 * string.
 */
export type OptimizeRow = {
  namespace: string;
  name: string;
  externalUrl?: string;
  callbackUrl?: string;
};

/** isOidc says whether plane runs in the oidc mode. It is false without a plane. */
export function isOidc(
  plane: Resource<"CamundaManagementCluster"> | undefined,
): boolean {
  const provider: unknown = plane?.spec?.identityProvider;
  return (
    typeof provider === "object" &&
    provider !== null &&
    (provider as { oidc?: unknown }).oidc != null
  );
}

/** contractReaders returns the CamundaOptimize objects whose spec.managementAuthRef is contract, sorted by namespace and name. None for an undefined contract. */
export function contractReaders(
  optimizes: readonly Resource<"CamundaOptimize">[],
  contract: string | undefined,
): Resource<"CamundaOptimize">[] {
  if (contract === undefined) return [];
  return optimizes
    .filter((o) => o.spec?.managementAuthRef === contract)
    .sort(
      (a, b) =>
        byCodeUnit(a.metadata?.namespace ?? "", b.metadata?.namespace ?? "") ||
        byCodeUnit(a.metadata?.name ?? "", b.metadata?.name ?? ""),
    );
}

/**
 * optimizeRows returns the Optimize instances whose login callback the
 * plane needs: status.optimize in the Keycloak modes, and in the oidc mode
 * the readers of status.managementAuthConfig that set spec.externalUrl, in
 * the order of contractReaders.
 */
export function optimizeRows(input: {
  plane: Resource<"CamundaManagementCluster"> | undefined;
  optimizes: readonly Resource<"CamundaOptimize">[];
}): OptimizeRow[] {
  const { plane, optimizes } = input;
  if (!plane) return [];
  if (isOidc(plane)) {
    const contract = plane.status?.managementAuthConfig;
    return contractReaders(
      optimizes,
      typeof contract === "string" && contract !== "" ? contract : undefined,
    )
      .filter((o) => {
        const url: unknown = o.spec.externalUrl;
        return url !== undefined && url !== null && url !== "";
      })
      .map((o) =>
        rowOf(o.metadata?.namespace, o.metadata?.name, o.spec.externalUrl),
      );
  }
  const listed: unknown = plane.status?.optimize;
  if (!Array.isArray(listed)) return [];
  return listed.flatMap((entry: unknown) => {
    if (typeof entry !== "object" || entry === null) return [];
    const e = entry as Record<string, unknown>;
    return [rowOf(e.namespace, e.name, e.externalUrl)];
  });
}

function rowOf(namespace: unknown, name: unknown, url: unknown): OptimizeRow {
  const externalUrl = typeof url === "string" && url !== "" ? url : undefined;
  return {
    namespace: typeof namespace === "string" ? namespace : "",
    name: typeof name === "string" ? name : "",
    ...(externalUrl !== undefined && {
      externalUrl,
      // The operator registers the plain concatenation, without a check of
      // a trailing slash.
      callbackUrl: externalUrl + OPTIMIZE_CALLBACK_PATH,
    }),
  };
}
