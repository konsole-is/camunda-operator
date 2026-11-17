import { clusterImpacts, RESTART_ALL } from "@/components/cluster/impacts";
import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { Resource } from "@/lib/k8s/kinds";
import { type SourceReads, unreadKinds } from "@/lib/k8s/unread";
import { CLAIM_LABEL, ENVIRONMENT_LABEL, isManaged } from "@/lib/labels";
import { managerKind } from "@/lib/merge";
import {
  byCodeUnit,
  declaresRelease,
  environmentOf as policyEnvironmentOf,
  type MoveStatus,
  moveStatus,
  releaseMove,
} from "@/lib/policy";
import {
  changeOf,
  claimAfter,
  type LockProvider,
  openSession,
  type Session,
  setValue,
  validateClusterClaim,
} from "@/lib/session";
import type { ApiError, Change, WriteResult } from "@/lib/write/types";
import { RELEASE_SOURCE_KINDS, ROLLOUT_SOURCE_KINDS } from "./source-kinds";

/** ClusterClaim is a CamundaClusterClaim as the watch gives it. */
export type ClusterClaim = Resource<"CamundaClusterClaim">;

const ROLLOUT_KINDS = {
  releases: RELEASE_SOURCE_KINDS.releases,
  environments: RELEASE_SOURCE_KINDS.environments,
  namespaces: RELEASE_SOURCE_KINDS.namespaces,
  clusters: RELEASE_SOURCE_KINDS.clusters,
  claims: RELEASE_SOURCE_KINDS.claims,
  databaseServers: RELEASE_SOURCE_KINDS.databaseServers,
  ...ROLLOUT_SOURCE_KINDS,
};

/**
 * RolloutData holds the lists that a move of clusters to a release reads,
 * as the watch gives them: lists of ReleasesData and every list of
 * RolloutLists.
 */
export type RolloutData = {
  [K in keyof typeof ROLLOUT_KINDS]: readonly Resource<
    (typeof ROLLOUT_KINDS)[K]
  >[];
};

/**
 * RolloutStatus is the eligibility of a cluster for a move: a MoveStatus,
 * or a cluster that no claim made, a claim label that names no claim, a
 * cluster that runs the release already, a move to the release that the
 * operator refused, or lists that the checks need and that are not read.
 * downgradeRefused is a refused downgrade that allowVersionDowngrade set
 * to the version to can permit.
 */
export type RolloutStatus =
  | MoveStatus
  | { kind: "notManaged"; text: string }
  | { kind: "noClaim"; text: string }
  | { kind: "onRelease"; text: string }
  | { kind: "refused"; text: string }
  | { kind: "downgradeRefused"; to: string; text: string }
  | { kind: "unread"; text: string };

/** RolloutAction is the action that a row offers. */
export type RolloutAction = "move" | "confirmDowngrade" | "openEnvironments";

/**
 * RolloutRow is one cluster of the Rollout tab. key is
 * "<namespace>/<name>". environment is undefined for a namespace of no
 * Environment. currentRelease is the release of the claim, else the release
 * of the cluster. refusal is the message of a Ready condition with reason
 * VersionSkipRefused, unless status holds it. action is undefined when the
 * row offers none.
 */
export type RolloutRow = {
  key: string;
  name: string;
  namespace: string;
  environment?: string;
  currentRelease?: string;
  runningVersion?: string;
  status: RolloutStatus;
  action?: RolloutAction;
  refusal?: string;
  cluster: Resource<"CamundaCluster">;
  claim?: ClusterClaim;
};

/** STATUS_LABELS are the values of the eligibility filter, by status kind. */
export const STATUS_LABELS: Record<RolloutStatus["kind"], string> = {
  canMove: "Can move",
  notAllowed: "Not allowed in the environment",
  downgrade: "Downgrade",
  skipsMinor: "Skips a Camunda minor",
  backupRunning: "A backup is pending or running",
  notManaged: "Not managed by the fleet manager",
  noClaim: "Claim missing",
  onRelease: "Runs this release",
  refused: "Refused by the operator",
  downgradeRefused: "Refused by the operator",
  waitsForResume: "Checks wait",
  waitsForVersion: "Checks wait",
  unread: "Lists not read",
};

const ACTIONS: Partial<Record<RolloutStatus["kind"], RolloutAction>> = {
  canMove: "move",
  // The Claim tab also lets a change through while the checks wait.
  waitsForResume: "move",
  waitsForVersion: "move",
  downgrade: "confirmDowngrade",
  downgradeRefused: "confirmDowngrade",
  notAllowed: "openEnvironments",
};

// The operator sets these reasons on Ready when it refuses a move that
// skips a minor or that downgrades. No generated constant holds condition
// reasons.
const VERSION_SKIP_REFUSED = "VersionSkipRefused";
const VERSION_DOWNGRADE_REFUSED = "VersionDowngradeRefused";

/**
 * PARTIAL_MOVE_CAUTION is the caution of a move of many claims in direct
 * mode.
 */
export const PARTIAL_MOVE_CAUTION =
  "The claims are written one after another. When one fails, the claims before it keep the new release, and the table shows the release of each claim.";

/**
 * unreadRolloutKinds returns each kind of the lists of RolloutData that
 * reads holds as failed or stale, once. While it is not empty, no cluster
 * can move.
 */
export function unreadRolloutKinds(reads: SourceReads): string[] {
  const kinds: readonly string[] = Object.values(ROLLOUT_KINDS);
  return unreadKinds(reads).filter((kind) => kinds.includes(kind));
}

/** unreadMoveText says why no cluster can move while the lists of kinds are not read. */
export function unreadMoveText(kinds: readonly string[]): string {
  return `A move is possible when the fleet manager has read ${kinds.join(", ")}.`;
}

/**
 * rolloutRows returns one row for each cluster of data, sorted by
 * namespace, then name, with its eligibility for a move to release. The
 * eligibility comes from releaseMove, which reads the running version of
 * the cluster, and moveStatus. While unreadRolloutKinds of data is not
 * empty, each row has the status unread and no action.
 */
export function rolloutRows(
  release: Resource<"CamundaRelease">,
  data: RolloutData & SourceReads,
): RolloutRow[] {
  const target = release.metadata?.name ?? "";
  const unread = unreadRolloutKinds(data);
  const backups = [...data.elasticsearchBackups, ...data.rdbmsBackups];
  return data.clusters
    .map((cluster): RolloutRow => {
      const namespace = cluster.metadata?.namespace ?? "";
      const name = cluster.metadata?.name ?? "";
      const claim = claimOf(cluster, data.claims);
      const { environment, refusal: environmentRefusal } = environmentOf(
        namespace,
        data,
      );
      const currentRelease =
        text(claim?.spec?.releaseRef) || text(cluster.spec?.releaseRef);
      const runningVersion = text(cluster.status?.management?.version);
      const ready = readyOf(cluster);
      const status = statusOf();
      const action = ACTIONS[status.kind];
      const refusal =
        ready?.reason === VERSION_SKIP_REFUSED &&
        status.kind !== "refused" &&
        status.kind !== "downgradeRefused"
          ? ready.message
          : undefined;
      return {
        key: `${namespace}/${name}`,
        name,
        namespace,
        ...(environment && { environment: environment.metadata?.name }),
        ...(currentRelease && { currentRelease }),
        ...(runningVersion && { runningVersion }),
        status,
        ...(action && { action }),
        ...(refusal && { refusal }),
        cluster,
        ...(claim && { claim }),
      };

      function statusOf(): RolloutStatus {
        if (unread.length > 0)
          return {
            kind: "unread",
            text: `The checks wait until the fleet manager has read ${unread.join(", ")}.`,
          };
        if (!isManaged(cluster))
          return {
            kind: "notManaged",
            text: "Not managed by the fleet manager.",
          };
        if (!claim)
          return {
            kind: "noClaim",
            text: `The claim ${cluster.metadata?.labels?.[CLAIM_LABEL]} is missing.`,
          };
        if (environmentRefusal)
          return { kind: "notAllowed", text: environmentRefusal };
        if (currentRelease === target) return targetStatus(claim);
        return checked();
      }

      function moveCheck() {
        return releaseMove(
          cluster,
          generated<CamundaRelease>(release),
          generated<Environment | undefined>(environment),
          backups,
        );
      }

      function checked(): MoveStatus {
        return moveStatus(moveCheck(), cluster);
      }

      function targetStatus(claim: ClusterClaim): RolloutStatus {
        if (text(cluster.spec?.releaseRef) !== target || !ready?.message)
          return { kind: "onRelease", text: "Runs this release." };
        const refused = `The operator refused the move: ${ready.message}`;
        const to = release.spec?.version ?? "";
        if (
          ready.reason === VERSION_DOWNGRADE_REFUSED &&
          to !== "" &&
          claim.spec?.allowVersionDowngrade !== to
        ) {
          // runClaimMove refuses the confirm during a backup, whatever the
          // environment allows.
          const check = moveStatus({ ...moveCheck(), allowed: true }, cluster);
          return check.kind === "backupRunning"
            ? check
            : { kind: "downgradeRefused", to, text: refused };
        }
        if (
          ready.reason === VERSION_DOWNGRADE_REFUSED ||
          ready.reason === VERSION_SKIP_REFUSED
        )
          return { kind: "refused", text: refused };
        return { kind: "onRelease", text: "Runs this release." };
      }
    })
    .sort(
      (a, b) =>
        byCodeUnit(a.namespace, b.namespace) || byCodeUnit(a.name, b.name),
    );
}

/** MovePick is one row to move, and whether the user confirmed its downgrade. */
export type MovePick = { row: RolloutRow; downgrade: boolean };

/**
 * MovePlan is the write of a move and what it does. change is undefined
 * when errors is not empty. Each error starts with "<namespace>/<name>: ".
 * impacts holds, for each cluster ("<namespace>/<name>"), what the move
 * does to it.
 */
export type MovePlan = {
  change?: Change;
  errors: string[];
  impacts: { cluster: string; lines: string[] }[];
};

// The claim refs of a move are top-level claim fields, and a cluster lock
// covers only fields under spec.cluster, the attachments, and the section
// parameters, so a move needs no lock.
const NO_LOCKS: LockProvider = () => undefined;

/**
 * planMove plans one write that moves the claim of each pick to release.
 * It sets spec.releaseRef of each claim, and for a pick with downgrade on a
 * downgrade row also spec.allowVersionDowngrade to the target version. It
 * keeps every other field of the claim, an earlier allowVersionDowngrade
 * included, and writes nothing else. Each claim carries its resourceVersion
 * as its precondition. A pick whose row offers no move, and a claim that
 * validateClusterClaim refuses, give an error.
 */
export function planMove(
  release: Resource<"CamundaRelease">,
  picks: readonly MovePick[],
  data: RolloutData,
): MovePlan {
  const target = release.metadata?.name ?? "";
  const intent =
    picks.length === 1
      ? `Move the cluster ${picks[0].row.key} to the release ${target}`
      : `Move ${picks.length} clusters to the release ${target}`;
  const errors: string[] = [];
  const impacts: MovePlan["impacts"] = [];
  const changes: Change[] = [];
  for (const { row, downgrade } of picks) {
    const named = (message: string) => `${row.key}: ${message}`;
    const session = movedSession(row, target, downgrade);
    if (typeof session === "string") {
      errors.push(named(session));
      continue;
    }
    const { environment } = environmentOf(row.namespace, data);
    const claimName = row.claim?.metadata.name;
    const requests = data.requests.filter(
      (r) =>
        r.metadata?.namespace === row.namespace &&
        r.metadata?.labels?.[CLAIM_LABEL] === claimName,
    );
    for (const error of validateClusterClaim(session, {
      environment,
      sections: data.sections,
      registrations: data.registrations,
      releases: data.releases,
      cluster: row.cluster,
      backups: [...data.elasticsearchBackups, ...data.rdbmsBackups],
      requests,
    }))
      errors.push(named(error.message));
    impacts.push({
      cluster: row.key,
      lines: [
        // Both specs name the release already, so the change of
        // allowVersionDowngrade alone shows no restart.
        ...(row.status.kind === "downgradeRefused" ? [RESTART_ALL] : []),
        ...clusterImpacts({
          session,
          live: row.cluster,
          presets: data.clusterPresets,
          releases: data.releases,
          sections: data.sections,
          environment,
        }),
        ...movedWithClaim(session, data.registrations),
        ...databaseServerRefusals(release, requests, data.databaseServers),
      ],
    });
    changes.push(changeOf(session, intent));
  }
  if (errors.length > 0) return { errors, impacts };
  return {
    change: {
      intent,
      apply: changes.flatMap((c) => c.apply),
      preconditions: Object.assign({}, ...changes.map((c) => c.preconditions)),
    },
    errors,
    impacts,
  };
}

/**
 * namedErrors returns result with the name of the claim in front of the
 * message of each error of a CamundaClusterClaim, as
 * "<namespace>/<name>: <message>". An ok result comes back as it is.
 */
export function namedErrors(result: WriteResult): WriteResult {
  if (result.ok) return result;
  return { ...result, errors: namedApiErrors(result.errors) };
}

/** namedApiErrors names the claim of each error as namedErrors does. */
export function namedApiErrors(errors: readonly ApiError[]): ApiError[] {
  return errors.map((error) => {
    const ref = error.ref;
    if (ref?.kind !== "CamundaClusterClaim") return error;
    const { path, ...rest } = error;
    // The review shows a path in front of the message, so the path moves
    // behind the claim name. The review names a Conflict that it can take
    // over in its take-over choice, so this message is for the others.
    const message =
      error.code === "Conflict" && path
        ? conflictMessage(path, error.manager)
        : path
          ? `${path}: ${error.message}`
          : error.message;
    return { ...rest, message: `${ref.namespace}/${ref.name}: ${message}` };
  });
}

function conflictMessage(path: string, manager: string | undefined): string {
  if (manager !== undefined && managerKind(manager) === "operator")
    return `${path} belongs to ${manager}, a controller of the operator, and the move cannot change it. Leave this cluster out of the move.`;
  return `${path} belongs to another tool${manager ? ` (${manager})` : ""}, and this review cannot take it over. Change it with that tool, or leave this cluster out of the move.`;
}

// movedSession returns the session of the claim of row with the move set,
// or the reason that the row cannot move.
function movedSession(
  row: RolloutRow,
  target: string,
  downgrade: boolean,
): Session<ClusterClaim> | string {
  if (row.action !== "move" && row.action !== "confirmDowngrade")
    return row.status.text;
  const opened = openSession({
    claim: row.claim,
    resource: row.cluster,
    locks: NO_LOCKS,
  });
  if (!opened) return "The claim cannot be changed.";
  let session: Session<ClusterClaim> = opened;
  const values: [string, unknown][] = [["releaseRef", target]];
  if (
    downgrade &&
    (row.status.kind === "downgrade" || row.status.kind === "downgradeRefused")
  )
    values.push(["allowVersionDowngrade", row.status.to]);
  for (const [field, value] of values) {
    const set = setValue(session, [{ field: "spec" }, { field }], value);
    if ("refused" in set) return `The field spec.${field} is locked.`;
    session = set.session;
  }
  return session;
}

// movedWithClaim returns a line for each selection of the claim whose
// registration takes the release of the claim, and one for Optimize.
function movedWithClaim(
  session: Session<ClusterClaim>,
  registrations: readonly Resource<"PluginRegistration">[],
): string[] {
  const spec = claimAfter(session).spec;
  const plugins: unknown = spec?.plugins;
  const lines = (Array.isArray(plugins) ? plugins : []).flatMap(
    (selection: { registration?: unknown }) => {
      const registration = registrations.find(
        (r) => r.metadata?.name === selection?.registration,
      );
      if (
        !registration ||
        !declaresRelease(generated<PluginRegistration>(registration))
      )
        return [];
      const label = registration.spec.label || registration.metadata?.name;
      return [`${label} moves to the release too.`];
    },
  );
  if (spec?.attachments?.optimize)
    lines.push("Optimize moves to the version of the release.");
  return lines;
}

function databaseServerRefusals(
  release: Resource<"CamundaRelease">,
  requests: readonly Resource<"PluginRequest">[],
  servers: readonly Resource<"DatabaseServer">[],
): string[] {
  const target = text(release.spec?.databaseServer?.version);
  if (!target) return [];
  return requests.flatMap((request) => {
    const resources: unknown = request.status?.resources;
    return (Array.isArray(resources) ? resources : []).flatMap(
      (ref: { kind?: unknown; name?: unknown }) => {
        if (ref?.kind !== "DatabaseServer") return [];
        const server = servers.find(
          (s) =>
            s.metadata?.namespace === request.metadata?.namespace &&
            s.metadata?.name === ref.name,
        );
        const running = text(server?.status?.version);
        // The operator refuses a change of the major of a running server.
        if (!server || !running || running === target) return [];
        if (text(server.spec?.version)) return [];
        return [
          `The database server ${server.metadata?.name} runs PostgreSQL ${running} and refuses PostgreSQL ${target} of the release. It keeps PostgreSQL ${running}.`,
        ];
      },
    );
  });
}

function claimOf(
  cluster: Resource<"CamundaCluster">,
  claims: readonly ClusterClaim[],
): ClusterClaim | undefined {
  const name = cluster.metadata?.labels?.[CLAIM_LABEL];
  if (!name) return undefined;
  return claims.find(
    (c) =>
      c.metadata?.namespace === cluster.metadata?.namespace &&
      c.metadata?.name === name,
  );
}

// environmentOf gives the Environment of namespace, or, when the claim
// controller refuses every claim in it, the text of the refusal.
function environmentOf(
  namespace: string,
  data: Pick<RolloutData, "namespaces" | "environments">,
): { environment?: Resource<"Environment">; refusal?: string } {
  const object = data.namespaces.find((n) => n.metadata?.name === namespace);
  if (!object) return {};
  const { environment, reason } = policyEnvironmentOf(
    generated<V1Namespace>(object),
    generated<Environment[]>(data.environments),
  );
  if (reason === "NoEnvironment")
    return {
      refusal: `The namespace ${namespace} belongs to no Environment, so the claim takes no change.`,
    };
  if (reason === "UnknownEnvironment")
    return {
      refusal: `The namespace ${namespace} names the Environment ${object.metadata?.labels?.[ENVIRONMENT_LABEL]}, which does not exist, so the claim takes no change.`,
    };
  return {
    environment: data.environments.find(
      (e) => e.metadata?.name === environment?.metadata.name,
    ),
  };
}

function readyOf(
  cluster: Resource<"CamundaCluster">,
): { reason?: string; message?: string } | undefined {
  const conditions: unknown = cluster.status?.conditions;
  if (!Array.isArray(conditions)) return undefined;
  const ready = conditions.find(
    (c: { type?: unknown }) => c?.type === "Ready",
  ) as { reason?: unknown; message?: unknown } | undefined;
  if (!ready) return undefined;
  return { reason: text(ready.reason), message: text(ready.message) };
}

// text gives a string field of a live object, or undefined for a missing
// value or a value of another type.
function text(value: unknown): string | undefined {
  return typeof value === "string" && value !== "" ? value : undefined;
}

// generated gives an object of the cache the generated type that the policy
// library takes. The two differ only in their timestamps, which the policy
// library does not read.
function generated<T>(value: unknown): T {
  return value as T;
}
