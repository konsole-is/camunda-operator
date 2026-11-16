// This module runs in the browser and on the server.
import type { CamundaManagementClusterClaimSpec } from "@/generated/crds/camundamanagementclusterclaim";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { ValidationError } from "@/lib/schema";

/** IdentityModeId is where the users of a plane sign in, as the key of spec.management.identityProvider. */
export type IdentityModeId = "keycloak" | "externalKeycloak" | "oidc";

/** IDENTITY_MODE_LABEL is the name of each identity mode in the copy. */
export const IDENTITY_MODE_LABEL: Record<IdentityModeId, string> = {
  keycloak: "Keycloak",
  externalKeycloak: "External Keycloak",
  oidc: "OIDC",
};

/** PlaneManagement is spec.management of a plane claim. */
export type PlaneManagement = CamundaManagementClusterClaimSpec["management"];

type ExternalKeycloakSpec = NonNullable<
  PlaneManagement["identityProvider"]["externalKeycloak"]
>;

/**
 * ExternalKeycloakDraft is the External Keycloak part of a plane draft. An
 * empty realm means camunda-platform. adminSecret is the name of the Secret
 * with the administrator credentials, under the keys username and password.
 */
export type ExternalKeycloakDraft = {
  url: string;
  realm: string;
  adminSecret: string;
  caBundle?: { name: string; key: string };
};

/**
 * PlaneIssuer is the issuer of the tokens of a plane: a known URL, a new
 * issuer, or unknown. backendUrl is the address of the same issuer inside
 * the Kubernetes cluster, which a cluster can trust too.
 */
export type PlaneIssuer =
  | { kind: "known"; url: string; backendUrl?: string }
  | { kind: "new" }
  | { kind: "unknown" };

const DEFAULT_REALM = "camunda-platform";
const MAX_URL = 2048;
const MAX_REALM = 255;
const REALM = /^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$/;
// The CRD refuses "https:host" and "https:/host", so the scheme and the host
// are checked on the raw text too.
const HTTP_AUTHORITY = /^https?:\/\/[^/?#]/i;
const BAD_ESCAPE = /%(?![0-9a-fA-F]{2})/;
const USER_INFO = /^[a-zA-Z][a-zA-Z0-9+.-]*:\/\/[^/?#]*@/;
const OIDC_NEEDED = "OIDC needs a platform config with OIDC authentication.";

const MODES: readonly IdentityModeId[] = [
  "keycloak",
  "externalKeycloak",
  "oidc",
];

/** identityModeOf returns the one mode that provider sets, or undefined when it sets none or more than one. */
export function identityModeOf(
  provider: PlaneManagement["identityProvider"] | undefined,
): IdentityModeId | undefined {
  const set = MODES.filter((mode) => provider?.[mode] != null);
  return set.length === 1 ? set[0] : undefined;
}

/** sameFamily reports whether a and b are both OIDC or both a Keycloak mode. */
export function sameFamily(a: IdentityModeId, b: IdentityModeId): boolean {
  return (a === "oidc") === (b === "oidc");
}

/** externalKeycloakDraftOf returns the draft of spec, with empty fields for what spec lacks. A url that is not a string is empty too. */
export function externalKeycloakDraftOf(
  spec: ExternalKeycloakSpec | undefined,
): ExternalKeycloakDraft {
  const ca = spec?.caBundleSecretRef;
  return {
    url: typeof spec?.url === "string" ? spec.url : "",
    realm: spec?.realm ?? "",
    adminSecret: spec?.adminCredentialsSecretRef?.name ?? "",
    ...(ca && { caBundle: { name: ca.name, key: ca.key } }),
  };
}

/** externalKeycloakSpecOf returns the spec of draft. An empty realm stays out, so the CRD default applies. */
export function externalKeycloakSpecOf(
  draft: ExternalKeycloakDraft,
): ExternalKeycloakSpec {
  const { url, realm, adminSecret, caBundle } = draft;
  // No usernameKey or passwordKey: the CRD defaults apply.
  return {
    url,
    ...(realm !== "" && { realm }),
    adminCredentialsSecretRef: { name: adminSecret },
    ...(caBundle && { caBundleSecretRef: { ...caBundle } }),
  };
}

/** authMethod is the method of a platform config: "basic" when auth or auth.method is unset, and without a config. */
export function authMethod(
  config: CamundaPlatformConfig | undefined,
): "basic" | "oidc" {
  return config?.spec.auth?.method === "oidc" ? "oidc" : "basic";
}

/** oidcDisabledReason is why the OIDC card is off for config, or undefined when it is on. */
export function oidcDisabledReason(
  config: CamundaPlatformConfig | undefined,
): string | undefined {
  if (authMethod(config) === "oidc") return undefined;
  return config
    ? `${OIDC_NEEDED} ${config.metadata.name} uses basic authentication.`
    : OIDC_NEEDED;
}

/**
 * externalKeycloakErrors returns one message for each CRD rule that the
 * fields of draft break, with the paths url, realm, adminSecret, and
 * caBundle.
 */
export function externalKeycloakErrors(
  draft: ExternalKeycloakDraft,
): ValidationError[] {
  const errors: ValidationError[] = [];
  const url = (message: string) => errors.push({ path: "url", message });
  const parsed = parseHttpUrl(draft.url);
  if (!parsed) url("Use an http or https URL with a host.");
  if (draft.url.includes("?") || draft.url.includes("#")) {
    url("The URL must have no query and no fragment.");
  }
  if (BAD_ESCAPE.test(draft.url)) {
    url("The URL has a % that does not start an escape such as %20.");
  }
  if (USER_INFO.test(draft.url)) {
    url("The URL must have no user and no password.");
  }
  if (draft.url.length > MAX_URL) {
    url(`The URL must be at most ${MAX_URL} characters.`);
  }
  if (
    draft.realm !== "" &&
    (!REALM.test(draft.realm) || draft.realm.length > MAX_REALM)
  ) {
    errors.push({
      path: "realm",
      message: `The realm holds letters, digits, dots, hyphens, and underscores, and starts and ends with a letter or a digit. It has at most ${MAX_REALM} characters.`,
    });
  }
  if (draft.adminSecret === "") {
    errors.push({
      path: "adminSecret",
      message: "Name the Secret with the Keycloak administrator credentials.",
    });
  }
  // The CRD checks the scheme only of a URL that parses.
  if (draft.caBundle && parsed && parsed.protocol !== "https:") {
    errors.push({
      path: "caBundle",
      message: "A CA bundle needs an https URL.",
    });
  }
  return errors;
}

/**
 * planeIssuer is the issuer whose tokens the plane hands out. In OIDC mode
 * it is the issuer of config, and unknown when config has none. Keycloak
 * mode starts a new issuer. Without a mode it is unknown.
 */
export function planeIssuer(
  mode: IdentityModeId | undefined,
  external: ExternalKeycloakDraft,
  config: CamundaPlatformConfig | undefined,
): PlaneIssuer {
  switch (mode) {
    case "oidc": {
      const url =
        authMethod(config) === "oidc"
          ? config?.spec.auth?.oidc?.issuerUrl
          : undefined;
      return url ? { kind: "known", url } : { kind: "unknown" };
    }
    case "externalKeycloak":
      return {
        kind: "known",
        url: `${external.url.replace(/\/+$/, "")}/realms/${external.realm || DEFAULT_REALM}`,
      };
    case "keycloak":
      return { kind: "new" };
    default:
      return { kind: "unknown" };
  }
}

/**
 * keycloakIssuer is the issuer of the realm of the Keycloak that a plane
 * runs: publicUrl is spec.identityProvider.keycloak.externalUrl of the
 * CamundaManagementCluster, and serviceUrl the address of its Keycloak
 * Service with the base path.
 */
export function keycloakIssuer(
  publicUrl: string,
  serviceUrl: string | undefined,
): PlaneIssuer {
  const realm = (url: string) =>
    `${url.replace(/\/+$/, "")}/realms/${DEFAULT_REALM}`;
  return {
    kind: "known",
    url: realm(publicUrl),
    ...(serviceUrl && { backendUrl: realm(serviceUrl) }),
  };
}

/** parseHttpUrl returns value as a URL when it is an http or https URL with a host, as the CRD rules of the URL fields accept, else undefined. */
export function parseHttpUrl(value: string): URL | undefined {
  if (!HTTP_AUTHORITY.test(value)) return undefined;
  try {
    const url = new URL(value);
    return (url.protocol === "http:" || url.protocol === "https:") &&
      url.hostname !== ""
      ? url
      : undefined;
  } catch {
    return undefined;
  }
}
