// This module runs in the browser and on the server.
import type { CamundaManagementClusterClaimSpec } from "@/generated/crds/camundamanagementclusterclaim";
import type { Step } from "@/lib/fields";
import { EMPTY_MAIL } from "./components";
import { mailOf } from "./draft";
import {
  type ExternalKeycloakDraft,
  type IdentityModeId,
  externalKeycloakSpecOf,
  identityModeOf,
} from "./identity";

/** PlaneClaimSpec is the spec of a plane claim. */
export type PlaneClaimSpec = CamundaManagementClusterClaimSpec;

// The edits read only these fields, so the spec of a live claim from the
// cache passes too.
type EditedSpec = Pick<PlaneClaimSpec, "management" | "versions">;

/**
 * PlaneEdit is one edit of a claim session: a value at steps, or a reset
 * that removes the claim value at steps. A list of edits applies in its
 * order, each on the result of the one before.
 */
export type PlaneEdit =
  { steps: Step[]; value: unknown } | { steps: Step[]; reset: true };

const ADMIN = ["spec", "management", "identity", "admin"] as const;
const EMPTY_EXTERNAL: ExternalKeycloakDraft = {
  url: "",
  realm: "",
  adminSecret: "",
};

/**
 * identityMoveEdits returns the edits that move the plane of spec to the
 * mode to: the new identity provider first, then a reset of each admin
 * field that the CRD refuses in that mode, then a reset of the Keycloak
 * version when the plane leaves the keycloak mode. A reset comes only for
 * a field that spec holds. external gives the fields of an External
 * Keycloak, empty when it is unset. A move to the mode of spec gives no
 * edit.
 */
export function identityMoveEdits(
  spec: EditedSpec,
  to: IdentityModeId,
  external: ExternalKeycloakDraft = EMPTY_EXTERNAL,
): PlaneEdit[] {
  if (identityModeOf(spec.management?.identityProvider) === to) return [];
  const edits: PlaneEdit[] = [
    {
      steps: at("spec", "management", "identityProvider"),
      value: providerOf(to, external),
    },
  ];
  const admin: Record<string, unknown> = spec.management?.identity?.admin ?? {};
  const refused =
    to === "oidc"
      ? ["username", "passwordSecretRef"]
      : ["claimName", "claimValue"];
  for (const name of refused) {
    if (admin[name] !== undefined) edits.push(reset(...ADMIN, name));
  }
  if (to !== "keycloak" && spec.versions?.keycloak !== undefined) {
    edits.push(reset("spec", "versions", "keycloak"));
  }
  return edits;
}

/**
 * componentEdits returns the edits that turn app on or off in the plane of
 * spec. On sets the object of app, with the default mail for Web Modeler,
 * and leaves the version for the admin to enter. Off removes the object
 * and the version of app. An app that is already in that state gives no
 * edit. A null object counts as off.
 */
export function componentEdits(
  spec: EditedSpec,
  app: "console" | "webModeler",
  on: boolean,
): PlaneEdit[] {
  if ((spec.management?.[app] != null) === on) return [];
  if (on) {
    return [
      {
        steps: at("spec", "management", app),
        value: app === "webModeler" ? { mail: mailOf(EMPTY_MAIL) } : {},
      },
    ];
  }
  return [
    reset("spec", "management", app),
    ...(spec.versions?.[app] !== undefined
      ? [reset("spec", "versions", app)]
      : []),
  ];
}

function providerOf(
  mode: IdentityModeId,
  external: ExternalKeycloakDraft,
): PlaneClaimSpec["management"]["identityProvider"] {
  switch (mode) {
    case "keycloak":
      return { keycloak: {} };
    case "oidc":
      return { oidc: {} };
    case "externalKeycloak":
      return { externalKeycloak: externalKeycloakSpecOf(external) };
  }
}

function at(...names: string[]): Step[] {
  return names.map((field) => ({ field }));
}

function reset(...names: string[]): PlaneEdit {
  return { steps: at(...names), reset: true };
}
