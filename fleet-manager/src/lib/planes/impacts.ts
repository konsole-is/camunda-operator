// This module runs in the browser and on the server.
import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { Resource } from "@/lib/k8s/kinds";
import type { LabelSelector } from "@/lib/labels";
import { equalJSON } from "@/lib/policy";
import { claimAfter, type PlaneClaim, type Session } from "@/lib/session";
import { runningOfClaim } from "./components";
import {
  IDENTITY_MODE_LABEL,
  type IdentityModeId,
  type PlaneIssuer,
  externalKeycloakDraftOf,
  identityModeOf,
  keycloakIssuer,
  planeIssuer,
} from "./identity";
import { type PreviewRow, servedPreview } from "./preview";
import {
  type PlaneVersions,
  VERSION_LABEL,
  runningFields,
  versionChangeWarnings,
  versionMatrixWarning,
  versionWarnings,
} from "./versions";

/**
 * PlaneChangeInput is what the impacts of a plane change read. plane is the
 * live CamundaManagementCluster, unset for a plane that has only its claim
 * yet. environment is the Environment of the namespace of the claim.
 * clusters and namespaces are those of every namespace.
 */
export type PlaneChangeInput = {
  session: Session<PlaneClaim>;
  plane?: Resource<"CamundaManagementCluster">;
  environment?: Resource<"Environment">;
  platformConfigs: readonly Resource<"CamundaPlatformConfig">[];
  clusters: readonly Resource<"CamundaCluster">[];
  namespaces: readonly Resource<"Namespace">[];
};

/** FIRST_ADMIN_STAYS is the impact of every move of the identity mode. */
export const FIRST_ADMIN_STAYS =
  "The first administrator stays. Management Identity keeps the one in its database.";
/** CALLBACKS_MOVE is the impact of a move away from the externalKeycloak mode. */
export const CALLBACKS_MOVE =
  "The Optimize sign-in callbacks move from the old realm to the new one. Keep the old administrator Secret, and the old CA bundle Secret if there is one, until status.callbackRealm of the plane names the new realm or is gone.";
/** MOVE_WAITS is the impact of a move from the externalKeycloak mode to another Keycloak while the plane serves an Optimize. */
export const MOVE_WAITS =
  "The move waits while the plane serves an Optimize. Users keep signing in at the old Keycloak until its realm holds no callbacks.";
/** MOVE_AT_ONCE is the impact of every other move away from the externalKeycloak mode. */
export const MOVE_AT_ONCE = "The move happens at once.";
/** KEYCLOAK_DELETED is the impact of a move away from the keycloak mode. */
export const KEYCLOAK_DELETED =
  "The plane deletes the Keycloak that it runs. Its database keeps the realm.";
/** OIDC_CALLBACKS is the impact of a move to the oidc mode. */
export const OIDC_CALLBACKS =
  "In OIDC mode the plane registers no callbacks. Add the callback of each Optimize to the application at your identity provider.";
/** CONSOLE_ON is the impact of turning Console on. */
export const CONSOLE_ON =
  "Console starts. Served clusters get the Console settings and restart once.";
/** CONSOLE_OFF is the impact of turning Console off. */
export const CONSOLE_OFF =
  "Console stops. Served clusters lose their Console settings and restart once.";
/** WEB_MODELER_OFF is the impact of turning Web Modeler off. */
export const WEB_MODELER_OFF = "Removes Web Modeler. Its database stays.";
/** PREVIEW_NEEDS_ISSUER is the impact of a change of the served clusters while the issuer before or after is unknown. */
export const PREVIEW_NEEDS_ISSUER =
  "The attach preview needs the OIDC issuer of the platform config. The operator decides which clusters attach.";

type Spec = Partial<PlaneClaim["spec"]>;

/**
 * servedChanges returns the clusters that the change attaches and detaches,
 * and the expected row of each cluster that the plane serves after it. It
 * is undefined while the issuer of the plane before or after the change is
 * unknown.
 */
export function servedChanges(
  input: PlaneChangeInput,
):
  | { attach: PreviewRow[]; detach: PreviewRow[]; after: PreviewRow[] }
  | undefined {
  const before = rowsOf(input.session.claim, input);
  const after = rowsOf(claimAfter(input.session), input);
  if (!before || !after) return undefined;
  return {
    attach: attached(after, before),
    detach: attached(before, after),
    after,
  };
}

/**
 * planeModeMove returns the move of the identity mode that the change of
 * session makes. It is undefined when the mode stays, and when the mode
 * before or after the change is not one mode.
 */
export function planeModeMove(
  session: Session<PlaneClaim>,
): { from: IdentityModeId; to: IdentityModeId } | undefined {
  const from = identityModeOf(session.claim.spec?.management?.identityProvider);
  const to = identityModeOf(
    claimAfter(session).spec?.management?.identityProvider,
  );
  return from && to && from !== to ? { from, to } : undefined;
}

/**
 * planeImpacts returns what the change of the session does, as lines for
 * the review, in this order: the move of the identity mode or of the
 * External Keycloak realm and where the callbacks go, each cluster that
 * attaches or detaches, Console on, Console off and Web Modeler off, then
 * each version move with the version warnings. A change that moves nothing
 * gives no line.
 */
export function planeImpacts(input: PlaneChangeInput): string[] {
  const live: Spec = input.session.claim.spec ?? {};
  const spec: Spec = claimAfter(input.session).spec ?? {};
  return [
    ...moveLines(input),
    ...servedLines(input, live, spec),
    ...(live.management?.console === undefined &&
    spec.management?.console !== undefined
      ? [CONSOLE_ON]
      : []),
    ...(live.management?.console !== undefined &&
    spec.management?.console === undefined
      ? [CONSOLE_OFF]
      : []),
    ...(live.management?.webModeler !== undefined &&
    spec.management?.webModeler === undefined
      ? [WEB_MODELER_OFF]
      : []),
    ...versionLines(live, spec),
  ];
}

function moveLines(input: PlaneChangeInput): string[] {
  const servesOptimize = (input.plane?.status?.optimize?.length ?? 0) > 0;
  const move = planeModeMove(input.session);
  if (!move) {
    return realmMoves(input.session)
      ? [CALLBACKS_MOVE, servesOptimize ? MOVE_WAITS : MOVE_AT_ONCE]
      : [];
  }
  const lines = [
    `Sign-in moves from ${IDENTITY_MODE_LABEL[move.from]} to ${IDENTITY_MODE_LABEL[move.to]}.`,
    FIRST_ADMIN_STAYS,
  ];
  if (move.from === "externalKeycloak") {
    lines.push(
      CALLBACKS_MOVE,
      move.to !== "oidc" && servesOptimize ? MOVE_WAITS : MOVE_AT_ONCE,
    );
  }
  if (move.from === "keycloak") lines.push(KEYCLOAK_DELETED);
  if (move.to === "oidc") lines.push(OIDC_CALLBACKS);
  return lines;
}

// realmMoves reports whether the change points the External Keycloak mode
// at another realm. A new administrator or certificate authority keeps the
// realm (SameRealm of camundamanagementcluster).
function realmMoves(session: Session<PlaneClaim>): boolean {
  const realm = (claim: PlaneClaim) => {
    const provider = claim.spec?.management?.identityProvider;
    if (identityModeOf(provider) !== "externalKeycloak") return undefined;
    const issuer = planeIssuer(
      "externalKeycloak",
      externalKeycloakDraftOf(provider?.externalKeycloak),
      undefined,
    );
    return issuer.kind === "known" ? realmKey(issuer.url) : undefined;
  };
  const before = realm(session.claim);
  const after = realm(claimAfter(session));
  return before !== undefined && after !== undefined && before !== after;
}

// realmKey folds the spellings of one realm URL as RealmIdentity of
// camundamanagementcluster does.
function realmKey(url: string): string {
  if (!URL.canParse(url)) return url;
  const parsed = new URL(url);
  let path = parsed.pathname;
  try {
    path = decodeURIComponent(path);
  } catch {
    // A malformed escape stays as it is.
  }
  const host = parsed.hostname.replace(/\.$/, "");
  const port = parsed.port ? `:${parsed.port}` : "";
  return `${parsed.protocol}//${host}${port}${path.replace(/\/+$/, "")}`;
}

function servedLines(
  input: PlaneChangeInput,
  live: Spec,
  spec: Spec,
): string[] {
  const changes = servedChanges(input);
  if (!changes) {
    const moved =
      !equalJSON(live.environments, spec.environments) ||
      !equalJSON(live.clusterSelector, spec.clusterSelector) ||
      !equalJSON(
        live.management?.identityProvider,
        spec.management?.identityProvider,
      );
    return moved ? [PREVIEW_NEEDS_ISSUER] : [];
  }
  // The plane writes the Console settings only while it runs Console.
  return [
    ...changes.attach.map((r) =>
      spec.management?.console !== undefined
        ? `${r.namespace}/${r.name} attaches and restarts its pods once, because it gets the Console settings.`
        : `${r.namespace}/${r.name} attaches.`,
    ),
    ...changes.detach.map((r) =>
      live.management?.console !== undefined
        ? `${r.namespace}/${r.name} detaches. It loses its Console settings and restarts once.`
        : `${r.namespace}/${r.name} detaches.`,
    ),
  ];
}

function versionLines(live: Spec, spec: Spec): string[] {
  const running = runningOfClaim(spec.management);
  const before = (live.versions ?? {}) as PlaneVersions;
  const after = (spec.versions ?? {}) as PlaneVersions;
  const fields = runningFields(running);
  const set = fields.filter(
    (f) => after[f] !== undefined && after[f] !== before[f],
  );
  const turnedOn = fields.some(
    (f) => !runningFields(runningOfClaim(live.management)).includes(f),
  );
  const matrix =
    set.length > 0 || turnedOn
      ? versionMatrixWarning(after, running)
      : undefined;
  return [
    ...set.flatMap((f) =>
      before[f]
        ? [`${VERSION_LABEL[f]} moves from ${before[f]} to ${after[f]}.`]
        : [],
    ),
    ...versionChangeWarnings(before, after, running),
    ...(matrix ? [matrix] : []),
    ...(set.includes("identity") || set.includes("keycloak")
      ? versionWarnings(after, running)
      : []),
  ];
}

// rowsOf returns the served rows of claim, undefined while its issuer is
// unknown. An unset selector selects no cluster.
function rowsOf(
  claim: PlaneClaim,
  input: PlaneChangeInput,
): PreviewRow[] | undefined {
  const spec: Spec = claim.spec ?? {};
  const ref =
    spec.platformConfigRef || input.environment?.spec.platformConfigRef;
  const config = input.platformConfigs.find((c) => c.metadata?.name === ref);
  const issuer = issuerOf(
    claim,
    input.plane,
    generated<CamundaPlatformConfig | undefined>(config),
  );
  if (issuer.kind === "unknown") return undefined;
  if (spec.clusterSelector === undefined) return [];
  return servedPreview({
    plane: {
      namespace: claim.metadata.namespace ?? "",
      name: claim.metadata.name ?? "",
    },
    environments: Array.isArray(spec.environments) ? spec.environments : [],
    clusterSelector: spec.clusterSelector as LabelSelector,
    issuer,
    clusters: generated<CamundaCluster[]>(input.clusters),
    namespaces: generated<V1Namespace[]>(input.namespaces),
    platformConfigs: generated<CamundaPlatformConfig[]>(input.platformConfigs),
  });
}

// issuerOf returns the issuer of the plane of claim. A Keycloak that the
// live plane runs already is no new issuer: a cluster can trust it, at its
// public or its Service address (trustsIssuer of camundamanagementcluster).
function issuerOf(
  claim: PlaneClaim,
  plane: PlaneChangeInput["plane"],
  config: CamundaPlatformConfig | undefined,
): PlaneIssuer {
  const provider = claim.spec?.management?.identityProvider;
  const mode = identityModeOf(provider);
  const running = plane?.spec?.identityProvider?.keycloak;
  const publicUrl = provider?.keycloak?.externalUrl || running?.externalUrl;
  if (mode === "keycloak" && running && publicUrl) {
    const service = plane.status?.endpoints?.find((e) => e.name === "keycloak");
    const namespace = plane.metadata?.namespace;
    return keycloakIssuer(
      publicUrl,
      service &&
        `http://${service.service}.${namespace}.svc:${service.port}${service.pathPrefix ?? ""}`,
    );
  }
  return planeIssuer(
    mode,
    externalKeycloakDraftOf(provider?.externalKeycloak),
    config,
  );
}

// attached returns the rows of rows that attach and do not attach in
// others.
function attached(rows: PreviewRow[], others: PreviewRow[]): PreviewRow[] {
  return rows.filter(
    (r) =>
      r.result === "Attach" &&
      !others.some(
        (o) =>
          o.namespace === r.namespace &&
          o.name === r.name &&
          o.result === "Attach",
      ),
  );
}

// generated gives an object of the cache the generated type that the
// planes library takes. The two differ only in their timestamps, string
// against Date, and the planes library reads no timestamp.
function generated<T>(value: unknown): T {
  return value as T;
}
