import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import { LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import { IMAGE_FIELDS, type ImageField } from "@/lib/merge";
import { parseHttpUrl } from "@/lib/planes/identity";
import type { ValidationError } from "@/lib/schema";
import { refKey } from "@/lib/write/refs";
import type { Change, KubeObject } from "@/lib/write/types";

const API_VERSION = "core.camunda.io/v1";
const KIND = "CamundaPlatformConfig";
const OIDC_PATH = "spec.auth.oidc";

// IMAGE_PATTERN is the Pattern of each field of ImagesSpec in
// api/v1/camundaplatformconfig_types.go.
const IMAGE_PATTERN =
  /^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]+(\/[a-z0-9]+([._-][a-z0-9]+)*)+|(\/[a-z0-9]+([._-][a-z0-9]+)*)*)\/?$/;

/** PlatformDraft is the spec of one platform config as the admin edits it. Its fields can be empty or incomplete. */
export type PlatformDraft = CamundaPlatformConfig["spec"];

type Oidc = NonNullable<NonNullable<PlatformDraft["auth"]>["oidc"]>;

/** ManagementClients are the OIDC clients of the management plane components. */
export type ManagementClients = NonNullable<Oidc["management"]>["clients"];

/** MANAGEMENT_CLIENTS are the management plane clients in the order of the form, with their label and whether they have a client secret. */
export const MANAGEMENT_CLIENTS: readonly {
  key: keyof ManagementClients;
  label: string;
  secret: boolean;
}[] = [
  { key: "identity", label: "Management Identity", secret: true },
  { key: "optimize", label: "Optimize", secret: true },
  { key: "webModeler", label: "Web Modeler", secret: false },
  { key: "webModelerApi", label: "Web Modeler API", secret: true },
  { key: "console", label: "Console", secret: false },
];

/** draftOf returns a copy of the spec of config, or an empty draft without a config. */
export function draftOf(
  config: CamundaPlatformConfig | undefined,
): PlatformDraft {
  return config ? structuredClone(config.spec) : {};
}

/**
 * specOf returns draft as a save writes it. In basic mode, auth is
 * { method: "basic" } with no oidc block, so the CRD rule refuses a save
 * that leaves an oidc block of another field manager. Empty strings, empty
 * management clients, and empty images are left out.
 */
export function specOf(draft: PlatformDraft): CamundaPlatformConfig["spec"] {
  const spec: CamundaPlatformConfig["spec"] = { ...draft };
  delete spec.licenseSecretRef;
  delete spec.images;
  const oidc = draft.auth?.oidc;
  spec.auth =
    draft.auth?.method === "oidc"
      ? { method: "oidc", oidc: oidcOf(oidc ?? ({} as Oidc)) }
      : { method: "basic" };
  if (draft.licenseSecretRef) spec.licenseSecretRef = draft.licenseSecretRef;
  const images = compact(draft.images ?? {});
  if (images) spec.images = images;
  return spec;
}

function oidcOf(oidc: Oidc): Oidc {
  const { management, ...fields } = oidc;
  const out = compact(fields as Record<string, unknown>) ?? {};
  const clients = Object.fromEntries(
    Object.entries(management?.clients ?? {}).flatMap(([key, client]) => {
      const kept = compact((client ?? {}) as Record<string, unknown>);
      return kept ? [[key, kept]] : [];
    }),
  );
  if (Object.keys(clients).length > 0) out.management = { clients };
  return out as Oidc;
}

/**
 * draftErrors returns one error for each rule of the CRD that draft
 * breaks, with the spec path of the field. The dry run of the review checks
 * the rules again.
 */
export function draftErrors(draft: PlatformDraft): ValidationError[] {
  const errors: ValidationError[] = [];
  const add = (path: string, message: string) => errors.push({ path, message });
  if (draft.auth?.method === "oidc") {
    const oidc = draft.auth.oidc ?? ({} as Partial<Oidc>);
    if (!oidc.issuerUrl) add(`${OIDC_PATH}.issuerUrl`, "Enter the issuer URL.");
    else if (!parseHttpUrl(oidc.issuerUrl))
      add(`${OIDC_PATH}.issuerUrl`, "The issuer URL is an http or https URL.");
    for (const [field, label] of [
      ["jwksUrl", "The JWKS URL"],
      ["tokenUrl", "The token URL"],
      ["authUrl", "The authorization URL"],
    ] as const) {
      const value = oidc[field];
      if (value && !parseHttpUrl(value))
        add(`${OIDC_PATH}.${field}`, `${label} is an http or https URL.`);
    }
    if (!oidc.clientId) add(`${OIDC_PATH}.clientId`, "Enter the client ID.");
    if (!completeRef(oidc.clientSecretRef))
      add(
        `${OIDC_PATH}.clientSecretRef`,
        "Pick the Secret key of the client secret.",
      );
    const clients = (oidc.management?.clients ?? {}) as Record<
      string,
      Record<string, unknown> | undefined
    >;
    for (const { key, label, secret } of MANAGEMENT_CLIENTS) {
      const client = compact(clients[key] ?? {});
      if (!client) continue;
      const path = `${OIDC_PATH}.management.clients.${key}`;
      if (!client.clientId)
        add(`${path}.clientId`, `Enter the client ID of ${label}.`);
      else if (
        secret &&
        !completeRef(client.clientSecretRef as SecretRef | undefined)
      )
        add(
          `${path}.clientSecretRef`,
          `Pick the Secret key of the client secret of ${label}.`,
        );
    }
  }
  for (const field of Object.keys(IMAGE_FIELDS) as ImageField[]) {
    const value = draft.images?.[field];
    if (value && !IMAGE_PATTERN.test(value))
      add(
        `spec.images.${field}`,
        "A repository is lowercase, with a registry and no tag, as in mirror.example.com/camunda/optimize.",
      );
  }
  return errors;
}

/**
 * updateChange applies live with the spec of specOf, with the
 * resourceVersion of live as its precondition, and without force. It
 * keeps the labels and the annotations of live, without the last applied
 * file of kubectl.
 */
export function updateChange(
  live: CamundaPlatformConfig,
  draft: PlatformDraft,
): Change {
  const name = live.metadata.name ?? "";
  const { labels, annotations } = live.metadata;
  const kept = Object.fromEntries(
    Object.entries(annotations ?? {}).filter(
      ([key]) => key !== LAST_APPLIED_ANNOTATION,
    ),
  );
  const object: KubeObject = {
    apiVersion: API_VERSION,
    kind: KIND,
    metadata: {
      name,
      ...(labels && { labels }),
      ...(Object.keys(kept).length > 0 && { annotations: kept }),
    },
    spec: specOf(draft),
  };
  return {
    intent: `Change the platform config ${name}`,
    apply: [object],
    ...preconditionOf(name, live.metadata.resourceVersion),
  };
}

/** deleteChange deletes live when it still has its resourceVersion and its UID. */
export function deleteChange(live: CamundaPlatformConfig): Change {
  const name = live.metadata.name ?? "";
  const uid = live.metadata.uid;
  return {
    intent: `Delete the platform config ${name}`,
    apply: [],
    delete: [{ apiVersion: API_VERSION, kind: KIND, name }],
    ...preconditionOf(name, live.metadata.resourceVersion),
    ...(uid ? { uids: { [refKey({ kind: KIND, name })]: uid } } : {}),
  };
}

type SecretRef = { namespace?: string; name?: string; key?: string };

function completeRef(ref: SecretRef | undefined): boolean {
  return Boolean(ref?.namespace && ref.name && ref.key);
}

function preconditionOf(
  name: string,
  resourceVersion: string | undefined,
): Pick<Change, "preconditions"> {
  return resourceVersion === undefined
    ? {}
    : { preconditions: { [refKey({ kind: KIND, name })]: resourceVersion } };
}

// compact returns the entries of object whose value is not an empty string
// or undefined, or undefined when none is left.
function compact<T extends Record<string, unknown>>(object: T): T | undefined {
  const entries = Object.entries(object).filter(
    ([, value]) => value !== undefined && value !== "",
  );
  return entries.length > 0 ? (Object.fromEntries(entries) as T) : undefined;
}
