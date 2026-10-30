// This module runs in the browser and on the server.
import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import {
  ENVIRONMENT_LABEL,
  type LabelSelector,
  MANAGEMENT_CLUSTER_ANNOTATION,
  matchesSelector,
} from "@/lib/labels";
import type { PlaneIssuer } from "./identity";

/** PreviewResult is the attach result that the operator reports for a cluster, with Attach for an attached one. */
export type PreviewResult =
  "Attach" | "ClaimedElsewhere" | "NotReady" | "InvalidReference";

/** PreviewRow is the expected result of one cluster. message says why, for every result but Attach. */
export type PreviewRow = {
  namespace: string;
  name: string;
  result: PreviewResult;
  label: string;
  message?: string;
};

/** NEW_ISSUER_MESSAGE is why a plane that runs its own Keycloak cannot serve an OIDC cluster. */
export const NEW_ISSUER_MESSAGE =
  "This cluster trusts another issuer. A Keycloak that this plane runs is a new issuer.";

/**
 * normalizeIssuer is normalizeIssuer of internal/controller/camundamanagementcluster:
 * the scheme and the host in lower case and no trailing slash on the path.
 * The port, the case of the path, and the query stay, so an explicit
 * default port makes another issuer.
 */
export function normalizeIssuer(issuer: string): string {
  // new URL() drops a default port, which Go keeps, so the URL is split by
  // hand, in the order of url.Parse.
  const [beforeFragment, rawFragment] = cut(issuer, "#");
  const [main, rawQuery] = cut(beforeFragment, "?");
  const scheme = /^([a-zA-Z][a-zA-Z0-9+.-]*):/.exec(main)?.[1];
  const rest = scheme === undefined ? main : main.slice(scheme.length + 1);
  const prefix = scheme === undefined ? "" : `${scheme.toLowerCase()}:`;
  const fragment = rawFragment === undefined ? "" : escapeFragment(rawFragment);
  if (fragment === undefined) return issuer.replace(/\/+$/, "");
  const tail = `${rawQuery === undefined ? "" : `?${rawQuery}`}${fragment === "" ? "" : `#${fragment}`}`;
  // Go keeps the text after "scheme:" as it is when it does not start with
  // a slash, and trims no path then.
  if (scheme !== undefined && !rest.startsWith("/")) {
    return `${prefix}${rest}${tail}`;
  }
  let host = "";
  let rawPath = rest;
  if (rest.startsWith("//")) {
    const end = rest.slice(2).search(/[/]/);
    const authority = end < 0 ? rest.slice(2) : rest.slice(2, end + 2);
    rawPath = end < 0 ? "" : rest.slice(end + 2);
    const at = authority.lastIndexOf("@");
    const hostPort = authority.slice(at + 1);
    if (!VALID_PORT.test(hostPort)) return issuer.replace(/\/+$/, "");
    host = `//${authority.slice(0, at + 1)}${hostPort.toLowerCase()}`;
  }
  const path = normalizePath(rawPath);
  if (path === undefined) return issuer.replace(/\/+$/, "");
  return `${prefix}${host}${path}${tail}`;
}

// cut splits s at the first sep, as strings.Cut of Go does.
function cut(s: string, sep: string): [string, string | undefined] {
  const i = s.indexOf(sep);
  return i < 0 ? [s, undefined] : [s.slice(0, i), s.slice(i + 1)];
}

// normalizePath is the path that url.URL.String of Go writes after the
// trailing slashes are trimmed. It is undefined for a path with an invalid
// escape, which url.Parse refuses.
function normalizePath(raw: string): string | undefined {
  const decoded = decodeBytes(raw);
  if (decoded === undefined) return undefined;
  const trimmed = decoded.replace(/\/+$/, "");
  return trimmed === decoded && RAW_PATH.test(raw)
    ? raw
    : escapeBytes(trimmed, PATH_SAFE);
}

// escapeFragment is the fragment that url.URL.String of Go writes. It is
// undefined for a fragment with an invalid escape, which url.Parse refuses.
function escapeFragment(raw: string): string | undefined {
  const decoded = decodeBytes(raw);
  if (decoded === undefined) return undefined;
  return RAW_FRAGMENT.test(raw) ? raw : escapeBytes(decoded, FRAGMENT_SAFE);
}

// The bytes of the text as a string of char codes 0 to 255, as Go keeps them.
function decodeBytes(raw: string): string | undefined {
  const bytes = String.fromCharCode(...new TextEncoder().encode(raw));
  if (/%(?![0-9a-fA-F]{2})/.test(bytes)) return undefined;
  return bytes.replace(/%([0-9a-fA-F]{2})/g, (_, hex: string) =>
    String.fromCharCode(parseInt(hex, 16)),
  );
}

function escapeBytes(bytes: string, safe: RegExp): string {
  return [...bytes]
    .map((c) =>
      safe.test(c)
        ? c
        : `%${c.charCodeAt(0).toString(16).toUpperCase().padStart(2, "0")}`,
    )
    .join("");
}

// The characters that Go does not escape (shouldEscape), and the raw text
// that Go writes back as it is (validEncoded), for a path and a fragment.
const PATH_SAFE = /^[A-Za-z0-9\-_.~$&+,/:;=@]$/;
const FRAGMENT_SAFE = /^[A-Za-z0-9\-_.~$&+,/:;=@?!()*]$/;
const RAW_PATH = /^(?:[A-Za-z0-9\-_.~$&+,/:;=@!'()*[\]]|%[0-9a-fA-F]{2})*$/;
const RAW_FRAGMENT =
  /^(?:[A-Za-z0-9\-_.~$&+,/:;=@?!'()*[\]]|%[0-9a-fA-F]{2})*$/;
// url.Parse refuses a port that is not digits.
const VALID_PORT = /^(?:\[[^\]]*\]|[^:[\]]*)(?::\d*)?$/;

/**
 * servedPreview predicts the row that the operator reports for each
 * CamundaCluster that the plane will serve, sorted by namespace and name.
 * A cluster is served when the namespace carries one of environments in
 * ENVIRONMENT_LABEL and clusterSelector matches the labels of the cluster.
 * clusterSelector is the selector that the claim writes: an empty selector
 * matches every cluster, and an unset one selects no cluster, so do not
 * pass undefined.
 *
 * The rules and their order are those of attach in
 * internal/controller/camundamanagementcluster/attachment.go. A change
 * there must change this function too.
 */
export function servedPreview(input: {
  plane: { namespace: string; name: string };
  environments: readonly string[];
  clusterSelector: LabelSelector;
  issuer: Exclude<PlaneIssuer, { kind: "unknown" }>;
  clusters: readonly CamundaCluster[];
  namespaces: readonly V1Namespace[];
  platformConfigs: readonly CamundaPlatformConfig[];
}): PreviewRow[] {
  const served = new Set(
    input.namespaces
      .filter((ns) => {
        const env = ns.metadata?.labels?.[ENVIRONMENT_LABEL];
        return env !== undefined && input.environments.includes(env);
      })
      .map((ns) => ns.metadata?.name),
  );
  const own = `${input.plane.namespace}/${input.plane.name}`;
  return input.clusters
    .filter(
      (c) =>
        served.has(c.metadata?.namespace) &&
        matchesSelector(input.clusterSelector, c.metadata?.labels),
    )
    .map((c) => ({
      namespace: c.metadata?.namespace ?? "",
      name: c.metadata?.name ?? "",
      ...predict(c, own, input.issuer, input.platformConfigs),
    }))
    .sort((a, b) =>
      a.namespace === b.namespace
        ? compare(a.name, b.name)
        : compare(a.namespace, b.namespace),
    );
}

function predict(
  cluster: CamundaCluster,
  own: string,
  issuer: Exclude<PlaneIssuer, { kind: "unknown" }>,
  configs: readonly CamundaPlatformConfig[],
): Pick<PreviewRow, "result" | "label" | "message"> {
  const holder = cluster.metadata?.annotations?.[MANAGEMENT_CLUSTER_ANNOTATION];
  if (holder && holder !== own) {
    return {
      result: "ClaimedElsewhere",
      label: `Served by ${holder}`,
      message: "Another plane serves this cluster.",
    };
  }
  if (!cluster.status?.gateway) {
    return {
      result: "NotReady",
      label: "Attaches when it runs",
      message: "The cluster publishes no gateway endpoints yet.",
    };
  }
  const ref = cluster.spec.platformConfigRef;
  const config = configs.find((c) => c.metadata.name === ref);
  if (!ref || !config) {
    return {
      result: "InvalidReference",
      label: "No platform config",
      message: ref
        ? `Platform config ${ref} of this cluster does not exist.`
        : "This cluster names no platform config.",
    };
  }
  const auth = config.spec.auth;
  if (auth?.method !== "oidc" || !auth.oidc) return attach();
  const trusted = auth.oidc.issuerUrl;
  const refuse = (message: string) => ({
    result: "InvalidReference" as const,
    label: "Different issuer",
    message,
  });
  if (!trusted) {
    return refuse(
      `Platform config ${ref} of this cluster sets no OIDC issuer.`,
    );
  }
  if (issuer.kind === "new") return refuse(NEW_ISSUER_MESSAGE);
  if (normalizeIssuer(trusted) !== normalizeIssuer(issuer.url)) {
    return refuse(
      `This cluster trusts ${trusted}. This plane hands out tokens of ${issuer.url}.`,
    );
  }
  return attach();
}

function attach(): Pick<PreviewRow, "result" | "label"> {
  return { result: "Attach", label: "Will attach" };
}

function compare(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}
