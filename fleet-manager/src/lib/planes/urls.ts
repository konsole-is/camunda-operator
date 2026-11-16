// This module runs in the browser and on the server.
import { SLOT_FIELDS } from "@/lib/fields/slots";
import type { Resource } from "@/lib/k8s/kinds";
import type { Running } from "./versions";

/** PlaneUrlId names one URL of a plane. */
export type PlaneUrlId =
  "keycloak" | "identity" | "console" | "webModeler" | "webModelerWebsockets";

/**
 * PlaneUrlField is one URL of a running app: names is its path under
 * spec.management of the claim, suffix the fixed path that an access
 * option gives it, and label and placeholder its copy.
 */
export type PlaneUrlField = {
  id: PlaneUrlId;
  label: string;
  names: readonly string[];
  suffix: string;
  placeholder: string;
};

/** PLANE_URL_LABEL is the name of the app of each URL of a plane. */
export const PLANE_URL_LABEL: Record<PlaneUrlId, string> = {
  keycloak: "Keycloak",
  identity: "Management Identity",
  console: "Console",
  webModeler: "Web Modeler",
  webModelerWebsockets: "Web Modeler websockets",
};

const EXAMPLE_HOST = "https://management.example.com";

// DOT_SEGMENT is the rule of the CRD: browsers remove a . or .. segment,
// and the operator does not.
const DOT_SEGMENT = /^[^?#]*\/[.][.]?([/?#]|$)/;

const RAW_URL = /^(https?):\/\/[^/?#]+(.*)$/i;

/**
 * planeUrlFields returns the URL fields of the running apps, from the
 * externalAccess rows of SLOT_FIELDS for the plane claim, in their order.
 */
export function planeUrlFields(running: Running): PlaneUrlField[] {
  return planeRows().flatMap((row) => {
    // The rows name the field of the CamundaManagementCluster. The claim
    // holds it under spec.management.
    const names = row.names.slice(1);
    const id = idOf(names);
    const runs =
      id === "keycloak"
        ? running.keycloak
        : id === "console"
          ? running.console
          : id === "identity" || running.webModeler;
    if (!runs) return [];
    const suffix = row.suffix ?? "";
    return [
      {
        id,
        label: PLANE_URL_LABEL[id],
        names,
        suffix,
        placeholder: `${EXAMPLE_HOST}${suffix}`,
      },
    ];
  });
}

/**
 * planeExternalUrls returns the external URL of each app as the spec of
 * plane holds it. An app without a URL, or with a value that is not a
 * non-empty string, has no entry. It is empty without a plane.
 */
export function planeExternalUrls(
  plane: Resource<"CamundaManagementCluster"> | undefined,
): Partial<Record<PlaneUrlId, string>> {
  const urls: Partial<Record<PlaneUrlId, string>> = {};
  for (const row of planeRows()) {
    // names starts with "spec".
    const value = row.names.reduce<unknown>(
      (at, name) =>
        typeof at === "object" && at !== null
          ? (at as Record<string, unknown>)[name]
          : undefined,
      plane,
    );
    if (typeof value === "string" && value !== "") {
      urls[idOf(row.names.slice(1))] = value;
    }
  }
  return urls;
}

/** accessUrls returns prefix + host + suffix of each field, as the claim controller writes them. */
export function accessUrls(
  host: string,
  fields: readonly PlaneUrlField[],
): Partial<Record<PlaneUrlId, string>> {
  return Object.fromEntries(
    fields.map((f) => [f.id, `https://${host}${f.suffix}`]),
  );
}

/**
 * urlErrors returns the errors of the typed URLs of "No access from
 * outside", at most one for each field, in field order. The rules are the
 * rules of the CRD for each field.
 */
export function urlErrors(
  urls: Partial<Record<PlaneUrlId, string>>,
  fields: readonly PlaneUrlField[],
): string[] {
  return fields.flatMap((field) => {
    const error = urlError(urls[field.id] ?? "", field);
    return error ? [error] : [];
  });
}

function urlError(value: string, field: PlaneUrlField): string | undefined {
  const name = `The ${field.label} URL`;
  if (value === "") return `Enter the ${field.label} URL.`;
  // The CRD checks the text as typed. The URL parser of the browser
  // repairs and normalizes it, so it can accept text that the CRD refuses.
  const raw = RAW_URL.exec(value);
  if (!raw || !URL.canParse(value) || new URL(value).hostname === "") {
    return `${name} must be an http or https URL.`;
  }
  const scheme = raw[1].toLowerCase();
  const path = raw[2].split(/[?#]/)[0];
  if (value.includes("\\")) return `${name} must carry no backslash.`;
  if (field.id === "keycloak") {
    if (value.includes("?") || value.includes("#")) {
      return `${name} must carry no query and no fragment.`;
    }
    return path === "/auth"
      ? undefined
      : `${name} must end in /auth, for example ${EXAMPLE_HOST}/auth.`;
  }
  if (value.includes("%")) {
    return `${name} must carry no percent-encoded character.`;
  }
  if (DOT_SEGMENT.test(value)) {
    return `${name} must carry no . or .. path segment.`;
  }
  if (field.id === "identity" && scheme === "http") {
    return path === "" || path === "/"
      ? undefined
      : `${name} needs https when it has a path.`;
  }
  return undefined;
}

function planeRows() {
  return SLOT_FIELDS.filter(
    (row) =>
      row.claimKind === "CamundaManagementClusterClaim" &&
      row.slot === "externalAccess",
  );
}

function idOf(names: readonly string[]): PlaneUrlId {
  if (names[0] === "identityProvider") return "keycloak";
  if (names.at(-1) === "websocketsExternalUrl") return "webModelerWebsockets";
  return names[0] as PlaneUrlId;
}
