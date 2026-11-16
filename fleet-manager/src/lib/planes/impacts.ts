// This module runs in the browser and on the server.
import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { IDENTITY_INITIAL_CLAIM_ANNOTATION } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { ACCESS_SECTION, type LabelSelector } from "@/lib/labels";
import { equalJSON, type PluginSelection } from "@/lib/policy";
import { claimAfter, type PlaneClaim, type Session } from "@/lib/session";
import { listOf } from "@/lib/text";
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
import { isDatabaseSection } from "./storage";
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
 * clusters and namespaces are those of every namespace. sections are the
 * PluginSections: without them, only the access section is known by its
 * name.
 */
export type PlaneChangeInput = {
  session: Session<PlaneClaim>;
  plane?: Resource<"CamundaManagementCluster">;
  environment?: Resource<"Environment">;
  platformConfigs: readonly Resource<"CamundaPlatformConfig">[];
  clusters: readonly Resource<"CamundaCluster">[];
  namespaces: readonly Resource<"Namespace">[];
  sections?: readonly Resource<"PluginSection">[];
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
export const WEB_MODELER_OFF =
  "Removes Web Modeler. Its database stays. Each served cluster with basic authentication loses the Web Modeler user of the plane.";
/** WEB_MODELER_USER_REMOVED is the impact on a basic-auth cluster that detaches from a plane that runs Web Modeler. */
export const WEB_MODELER_USER_REMOVED =
  "It loses the Web Modeler user of the plane.";
/** ACCESS_ISSUER_MOVES is the impact of a change of the access settings of a plane that runs Keycloak. */
export const ACCESS_ISSUER_MOVES =
  "A new access host moves the public URL of Keycloak. Served clusters that sign in with OIDC and trust the old public URL then detach, lose their Console settings, and restart once. Clusters that trust the Keycloak Service address stay.";
/** SECOND_USER is the impact of a new administrator username in a Keycloak mode. */
export const SECOND_USER =
  "Management Identity creates a second administrator. The first one keeps its access.";
/** OPTIMIZE_RESTARTS is the restart line of the Optimize of the served clusters. */
export const OPTIMIZE_RESTARTS =
  "The Optimize of each served cluster restarts, because its sign-in settings change.";
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
 * planeSignInMoves reports whether the change moves where users and the
 * served clusters sign in: a move of the identity mode, another External
 * Keycloak realm, another platform config in OIDC mode, or a change of the
 * access settings in the keycloak mode.
 */
export function planeSignInMoves(input: PlaneChangeInput): boolean {
  const live: Spec = input.session.claim.spec ?? {};
  const spec: Spec = claimAfter(input.session).spec ?? {};
  return (
    planeModeMove(input.session) !== undefined ||
    realmMoves(input.session) ||
    (identityModeOf(spec.management?.identityProvider) === "oidc" &&
      effectiveConfig(live, input) !== effectiveConfig(spec, input)) ||
    accessLines(input, live, spec).length > 0
  );
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
    ...platformConfigLines(input, live, spec),
    ...moveLines(input),
    ...adminLines(input.session, live, spec),
    ...servedLines(input, live, spec),
    ...accessLines(input, live, spec),
    ...(live.management?.console == null && spec.management?.console != null
      ? [CONSOLE_ON]
      : []),
    ...(live.management?.console != null && spec.management?.console == null
      ? [CONSOLE_OFF]
      : []),
    ...(live.management?.webModeler != null &&
    spec.management?.webModeler == null
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
  // An escape of a reserved character, such as %2F, names another path.
  const path = rawPath(url, parsed).replace(
    /%([0-9a-f]{2})/gi,
    (escape, hex) => {
      const char = String.fromCharCode(parseInt(hex, 16));
      return /[A-Za-z0-9._~-]/.test(char) ? char : escape.toUpperCase();
    },
  );
  const host = parsed.hostname.replace(/\.$/, "");
  const port = parsed.port ? `:${parsed.port}` : "";
  return `${parsed.protocol}//${host}${port}${path.replace(/\/+$/, "")}`;
}

// rawPath returns the path of url as URL encodes it, with its dot segments
// kept: the operator does not remove them.
function rawPath(url: string, parsed: URL): string {
  const raw = /^[a-z][a-z0-9+.-]*:\/\/[^/?#]*([^?#]*)/i.exec(url)?.[1];
  if (raw === undefined) return parsed.pathname;
  return raw
    .split("/")
    .map((segment) =>
      /^(\.|%2e){1,2}$/i.test(segment)
        ? segment
        : new URL(`http://h/${segment}`).pathname.slice(1),
    )
    .join("/");
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
  return servedChangeLines(changes, input);
}

/**
 * servedChangeLines returns one line for each cluster of changes that
 * attaches, then one for each that detaches, as planeImpacts lists them
 * for the change of input.
 */
export function servedChangeLines(
  changes: { attach: PreviewRow[]; detach: PreviewRow[] },
  input: PlaneChangeInput,
): string[] {
  const live: Spec = input.session.claim.spec ?? {};
  const spec: Spec = claimAfter(input.session).spec ?? {};
  // The plane keeps a Web Modeler user only on a basic-auth cluster
  // (basicUserSecret of camundamanagementcluster).
  const user = (r: PreviewRow) =>
    live.management?.webModeler != null && basicAuth(r, input)
      ? ` ${WEB_MODELER_USER_REMOVED}`
      : "";
  // The plane writes the Console settings only while it runs Console.
  return [
    ...changes.attach.map((r) =>
      spec.management?.console != null
        ? `${r.namespace}/${r.name} attaches and restarts its pods once, because it gets the Console settings.`
        : `${r.namespace}/${r.name} attaches.`,
    ),
    ...changes.detach.map(
      (r) =>
        (live.management?.console != null
          ? `${r.namespace}/${r.name} detaches. It loses its Console settings and restarts once.`
          : `${r.namespace}/${r.name} detaches.`) + user(r),
    ),
  ];
}

function basicAuth(row: PreviewRow, input: PlaneChangeInput): boolean {
  const cluster = input.clusters.find(
    (c) =>
      c.metadata?.namespace === row.namespace && c.metadata?.name === row.name,
  );
  const ref = cluster?.spec?.platformConfigRef;
  const config = input.platformConfigs.find((c) => c.metadata?.name === ref);
  return config?.spec?.auth?.method === "basic";
}

function versionLines(live: Spec, spec: Spec): string[] {
  const running = runningOfClaim(spec.management);
  const before = (live.versions ?? {}) as PlaneVersions;
  const after = (spec.versions ?? {}) as PlaneVersions;
  const fields = runningFields(running);
  const set = fields.filter((f) => after[f] != null && after[f] !== before[f]);
  const was = runningFields(runningOfClaim(live.management));
  const turnedOn = fields.some((f) => f !== "keycloak" && !was.includes(f));
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
// unknown. An unset or null selector selects no cluster, as a nil
// LabelSelector does in the operator.
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
  if (spec.clusterSelector == null) return [];
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
  const pending = provider?.keycloak?.externalUrl;
  const publicUrl =
    (typeof pending === "string" && pending) || running?.externalUrl;
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

function platformConfigLines(
  input: PlaneChangeInput,
  live: Spec,
  spec: Spec,
): string[] {
  const before = effectiveConfig(live, input);
  const after = effectiveConfig(spec, input);
  if (before === after) return [];
  const name = (ref: string | undefined) => ref ?? "none";
  return [`The platform config moves from ${name(before)} to ${name(after)}.`];
}

// A claim without a platform config takes the one of its Environment.
function effectiveConfig(
  spec: Spec,
  input: PlaneChangeInput,
): string | undefined {
  return (
    stringOf(spec.platformConfigRef) ||
    stringOf(input.environment?.spec.platformConfigRef) ||
    undefined
  );
}

// A new username in a Keycloak mode makes Identity create a second user
// (KEYCLOAK_USERS_0 of camundamanagementcluster).
function adminLines(
  session: Session<PlaneClaim>,
  live: Spec,
  spec: Spec,
): string[] {
  const mode = identityModeOf(spec.management?.identityProvider);
  if (mode === undefined || mode === "oidc" || planeModeMove(session))
    return [];
  const before = live.management?.identity?.admin?.username;
  const after = spec.management?.identity?.admin?.username;
  return before && after && before !== after ? [SECOND_USER] : [];
}

function accessLines(
  input: PlaneChangeInput,
  live: Spec,
  spec: Spec,
): string[] {
  const keycloak = (s: Spec) =>
    identityModeOf(s.management?.identityProvider) === "keycloak";
  if (!keycloak(live) || !keycloak(spec)) return [];
  const changed = changedSelections(live, spec).some((p) => isAccess(p, input));
  return changed ? [ACCESS_ISSUER_MOVES] : [];
}

/**
 * planeRestarts returns the restart lines of the change of the session, in
 * this order: the plane apps that restart, the Optimize of the served
 * clusters when it restarts, then the apps that may restart with the
 * reason. An app that the change starts or stops gets no line here, and a
 * served cluster gets its line from planeImpacts. A change that reaches no
 * pod gives no line.
 */
export function planeRestarts(input: PlaneChangeInput): string[] {
  const live: Spec = input.session.claim.spec ?? {};
  const spec: Spec = claimAfter(input.session).spec ?? {};
  const restarts = restartsOf(input, live, spec);
  const before = runningOfClaim(live.management);
  const after = runningOfClaim(spec.management);
  const runs = (app: App) =>
    app === "identity" ||
    app === "database" ||
    (app === "optimize"
      ? (input.plane?.status?.optimize?.length ?? 0) > 0
      : before[app] && after[app]);
  const sure = APPS.filter((app) => runs(app) && restarts.sure.has(app));
  const reasons = new Map<string, App[]>();
  for (const app of APPS) {
    const reason = restarts.may.get(app);
    if (reason === undefined || sure.includes(app) || !runs(app)) continue;
    reasons.set(reason, [...(reasons.get(reason) ?? []), app]);
  }
  const plane = sure.filter((app) => app !== "optimize");
  return [
    ...(plane.length > 0
      ? [
          `${sentence(listOf(plane.map((app) => APP_LABEL[app])))} ${plane.length === 1 ? "restarts" : "restart"}.`,
        ]
      : []),
    ...(sure.includes("optimize") ? [OPTIMIZE_RESTARTS] : []),
    ...[...reasons].map(
      ([reason, apps]) =>
        `${sentence(listOf(apps.map((app) => APP_LABEL[app])))} may restart ${reason}.`,
    ),
  ];
}

type App =
  "keycloak" | "identity" | "console" | "webModeler" | "database" | "optimize";

const APPS: readonly App[] = [
  "keycloak",
  "identity",
  "console",
  "webModeler",
  "database",
  "optimize",
];

const APP_LABEL: Record<App, string> = {
  ...VERSION_LABEL,
  database: "the database of the plane",
  optimize: "the Optimize of each served cluster",
};

const IMAGE_REASON = "when the platform config sets another image";
const DATABASE_REASON =
  "when the database settings change, as the option decides";
const ACCESS_REASON = "when the access host changes";
const SERVED_REASON = "when the served clusters change";
const PATH_REASON = "when the path of the URL changes";
const SECTION_REASON = "as the option of the changed section decides";

// restartsOf gives the apps whose pod template the change moves, and the
// apps that it moves only in some cases, with the reason. Each rule names
// the camundamanagementcluster code of its pod input: the config hash of a
// Deployment covers its env, the generations it reads, and the Secrets it
// checks.
function restartsOf(
  input: PlaneChangeInput,
  live: Spec,
  spec: Spec,
): { sure: Set<App>; may: Map<App, string> } {
  const sure = new Set<App>();
  const may = new Map<App, string>();
  const add = (...apps: App[]) => apps.forEach((app) => sure.add(app));
  const maybe = (reason: string, ...apps: App[]) =>
    apps.forEach((app) => may.has(app) || may.set(app, reason));
  const before = live.management;
  const after = spec.management;
  const modeAfter = identityModeOf(after?.identityProvider);
  const keycloakFamily = modeAfter !== undefined && modeAfter !== "oidc";
  const differs = (a: unknown, b: unknown) =>
    !equalJSON(a ?? undefined, b ?? undefined);

  // The platform config is an input of every plane Deployment, and in OIDC
  // mode it gives the issuer of Optimize (precheck.go, input.go).
  if (effectiveConfig(live, input) !== effectiveConfig(spec, input)) {
    add("identity", "console", "webModeler");
    maybe(IMAGE_REASON, "keycloak");
    if (modeAfter === "oidc") add("optimize");
  }
  // A new mode or realm changes the issuer of every app and of Optimize
  // (keycloakenv.go, console.go, webmodeler.go).
  if (planeModeMove(input.session) || realmMoves(input.session))
    add("identity", "console", "webModeler", "optimize");
  if (
    modeAfter === "externalKeycloak" &&
    differs(
      before?.identityProvider?.externalKeycloak?.adminCredentialsSecretRef,
      after?.identityProvider?.externalKeycloak?.adminCredentialsSecretRef,
    )
  )
    add("identity");

  const admin = (s: typeof before) => s?.identity?.admin;
  const fields = keycloakFamily
    ? (["username", "email", "passwordSecretRef"] as const)
    : (["claimName", "claimValue"] as const);
  if (
    !planeModeMove(input.session) &&
    fields.some((f) => differs(admin(before)?.[f], admin(after)?.[f])) &&
    !(modeAfter === "oidc" && recordedClaim(input.plane))
  )
    add("identity");

  // Web Modeler lists the served clusters in its env (clusters.go).
  const served = servedChanges(input);
  if (served && served.attach.length + served.detach.length > 0)
    add("webModeler");
  if (
    !served &&
    (differs(live.environments, spec.environments) ||
      differs(live.clusterSelector, spec.clusterSelector))
  )
    maybe(SERVED_REASON, "webModeler");
  // The Keycloak modes register a client for Console and Web Modeler in
  // the env of Identity (keycloakenv.go).
  const turned = (app: "console" | "webModeler") =>
    (before?.[app] != null) !== (after?.[app] != null);
  if (keycloakFamily && (turned("console") || turned("webModeler")))
    add("identity");
  if (differs(before?.webModeler?.mail, after?.webModeler?.mail))
    add("webModeler");

  const versionsBefore = (live.versions ?? {}) as PlaneVersions;
  const versionsAfter = (spec.versions ?? {}) as PlaneVersions;
  for (const field of runningFields(runningOfClaim(after)))
    if (
      versionsAfter[field] != null &&
      versionsAfter[field] !== versionsBefore[field]
    )
      add(field);

  for (const selection of changedSelections(live, spec)) {
    if (isAccess(selection, input)) {
      maybe(
        ACCESS_REASON,
        "keycloak",
        "identity",
        "console",
        "webModeler",
        "optimize",
      );
    } else if (
      input.sections === undefined ||
      isDatabaseSection(
        generated<PluginSection[]>(input.sections),
        selection.section,
      )
    ) {
      maybe(
        DATABASE_REASON,
        "keycloak",
        "identity",
        "console",
        "webModeler",
        "database",
      );
    } else {
      maybe(
        SECTION_REASON,
        "keycloak",
        "identity",
        "console",
        "webModeler",
        "database",
      );
    }
  }

  // The URLs of the plane go into the env of the apps that use them
  // (keycloak.go, identity.go, console.go, webmodeler.go, contract.go).
  const url = (s: typeof before, ...names: string[]) =>
    names.reduce<unknown>(
      (at, name) =>
        typeof at === "object" && at !== null
          ? (at as Record<string, unknown>)[name]
          : undefined,
      s,
    );
  const moved = (...names: string[]) =>
    differs(url(before, ...names), url(after, ...names));
  if (moved("identityProvider", "keycloak", "externalUrl"))
    add("keycloak", "identity", "console", "webModeler", "optimize");
  if (moved("identity", "externalUrl")) {
    add("identity", "optimize");
    maybe(PATH_REASON, "console", "webModeler");
  }
  if (moved("console", "externalUrl")) {
    maybe(PATH_REASON, "console");
    if (keycloakFamily) add("identity");
  }
  if (moved("webModeler", "externalUrl")) {
    add("webModeler");
    if (keycloakFamily) add("identity");
  }
  if (moved("webModeler", "websocketsExternalUrl")) add("webModeler");
  return { sure, may };
}

// changedSelections returns the selections after the change whose
// parameters differ from the same selection before it.
function changedSelections(live: Spec, spec: Spec): PluginSelection[] {
  const before = selectionsOf(live);
  return selectionsOf(spec).filter(
    (p) =>
      !equalJSON(
        p.parameters ?? {},
        before.find(
          (q) => q.section === p.section && q.registration === p.registration,
        )?.parameters ?? {},
      ),
  );
}

function selectionsOf(spec: Spec): PluginSelection[] {
  const plugins: unknown = spec.plugins;
  return Array.isArray(plugins)
    ? plugins.filter(
        (p): p is PluginSelection =>
          typeof p === "object" &&
          p !== null &&
          typeof p.section === "string" &&
          typeof p.registration === "string",
      )
    : [];
}

function isAccess(
  selection: PluginSelection,
  input: PlaneChangeInput,
): boolean {
  const section = input.sections?.find(
    (s) => s.metadata?.name === selection.section,
  );
  return section
    ? section.spec.slot === "externalAccess"
    : selection.section === ACCESS_SECTION;
}

function recordedClaim(plane: PlaneChangeInput["plane"]): boolean {
  return Boolean(
    plane?.metadata?.annotations?.[IDENTITY_INITIAL_CLAIM_ANNOTATION],
  );
}

function sentence(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

function stringOf(value: unknown): string {
  return typeof value === "string" ? value : "";
}
