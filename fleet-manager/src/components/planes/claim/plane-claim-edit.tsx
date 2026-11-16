"use client";

import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  ClaimEditProvider,
  useClaimEdit,
  type ClaimActions,
  type ClaimEdit,
  type ClaimEditHost,
  type ClaimReviewInput,
} from "@/components/fleet/claim-edit";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import {
  type FieldLock,
  IDENTITY_INITIAL_CLAIM_ANNOTATION,
  type PlaneField,
  planeFields,
  type Step,
} from "@/lib/fields";
import {
  useResources,
  useWatchesFresh,
  useWatchesOverdue,
} from "@/lib/k8s/hooks";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import type { FieldPath, KeptField } from "@/lib/merge";
import { identityMoveEdits } from "@/lib/planes/edits";
import {
  type ExternalKeycloakDraft,
  type IdentityModeId,
  identityModeOf,
} from "@/lib/planes/identity";
import {
  type PlaneChangeInput,
  planeImpacts,
  planeRestarts,
  planeSignInMoves,
} from "@/lib/planes/impacts";
import { environmentNameOf, equalJSON } from "@/lib/policy";
import {
  appliedSession,
  claimAfter,
  type FieldError,
  type LockProvider,
  type PlaneClaim,
  type PlaneClaimContext,
  planeLocks,
  type Session,
  keptFields,
  saveManager,
  validatePlaneClaim,
  valueAt,
} from "@/lib/session";
import type { WriteMode } from "@/lib/write/types";
import type { PlanePageData } from "../plane-view";

/**
 * PlaneClaimSources are the objects that the Claim tab of a plane reads,
 * live. claim is undefined for a plane that no claim made, and while the
 * page cannot tell which claim made the plane. unread names each kind that
 * the checks or the impacts of a change read and that the page could not
 * read or whose list is not current. claimUnread says that one of them is
 * the plane or its claim. waiting names the kinds of unread whose list is
 * not current.
 */
export type PlaneClaimSources = {
  name: string;
  namespace: string;
  plane?: Resource<"CamundaManagementCluster">;
  claim?: PlaneClaim;
  environment?: Resource<"Environment">;
  environments: readonly Resource<"Environment">[];
  namespaces: readonly Resource<"Namespace">[];
  optimizes: readonly Resource<"CamundaOptimize">[];
  clusters: readonly Resource<"CamundaCluster">[];
  platformConfigs: readonly Resource<"CamundaPlatformConfig">[];
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  requests: readonly Resource<"PluginRequest">[];
  unread: readonly string[];
  waiting: readonly string[];
  claimUnread: boolean;
};

/**
 * PlaneClaimEdit is the claim edit state of the plane page. edit is
 * undefined without claim actions. writeMode is the write mode of a save. moveIdentity moves the identity mode of
 * the claim after the pending changes to to, with the edits of
 * identityMoveEdits, and returns the lock that refused one of them. A move
 * to the mode of the live claim takes back the identity changes instead.
 * The lock on the administrator claim pair follows the mode after the move.
 */
export type PlaneClaimEdit = PlaneClaimSources & {
  edit?: ClaimEdit<PlaneClaim>;
  writeMode?: WriteMode;
  moveIdentity(
    to: IdentityModeId,
    external?: ExternalKeycloakDraft,
  ): FieldLock | undefined;
};

const PlaneClaimEditContext = createContext<PlaneClaimEdit | undefined>(
  undefined,
);

const PAGE_KINDS: readonly KindName[] = [
  "CamundaManagementCluster",
  "CamundaManagementClusterClaim",
  "Namespace",
  "Environment",
  "CamundaOptimize",
];
const OBJECT_KINDS: readonly string[] = [
  "CamundaManagementCluster",
  "CamundaManagementClusterClaim",
];
const NONE: never[] = [];

const IDENTITY_STEPS: Step[][] = [
  ["spec", "management", "identityProvider"],
  ["spec", "management", "identity", "admin"],
  ["spec", "versions", "keycloak"],
].map((names) => names.map((field) => ({ field })));

/**
 * changesWaitText is the reason that a change of the claim cannot be
 * reviewed while the page could not read the kinds of unread that are not
 * in waiting, or the lists of the kinds of waiting are not current.
 */
export function changesWaitText(
  unread: readonly string[],
  waiting: readonly string[] = [],
): string {
  const failed = unread.filter((k) => !waiting.includes(k));
  const parts = [
    ...(failed.length > 0
      ? [`the page could not read ${failed.join(", ")}`]
      : []),
    ...(waiting.length > 0
      ? [
          `${waiting.join(", ")} ${waiting.length === 1 ? "is" : "are"} not current`,
        ]
      : []),
  ];
  return `Review waits: ${parts.join(", and ")}, so what this change does is not known.`;
}

/**
 * PlaneClaimEditProvider holds the claim edit session of the plane page,
 * through ClaimEditProvider, over claim, the live claim of the plane. It
 * follows the lists of data.claimTab live. optimizes are the
 * CamundaOptimizes of the page. pageUnread are the kinds of the page whose
 * read failed or whose list is not current, and pageStale the latter. While
 * a kind that the checks or the impacts read is unread, the session refuses
 * review with one error that names the kinds, and an open review closes.
 * Without actions it opens no session. It needs a GitPushesProvider above
 * it.
 */
export function PlaneClaimEditProvider({
  data,
  plane,
  claim,
  namespaces,
  environments,
  optimizes = NONE,
  pageUnread,
  pageStale = NONE,
  writeMode,
  actions,
  children,
}: {
  data: PlanePageData;
  plane?: Resource<"CamundaManagementCluster">;
  claim?: PlaneClaim;
  namespaces: readonly Resource<"Namespace">[];
  environments: readonly Resource<"Environment">[];
  optimizes?: readonly Resource<"CamundaOptimize">[];
  pageUnread: readonly string[];
  pageStale?: readonly string[];
  writeMode: WriteMode | undefined;
  actions?: ClaimActions;
  children: ReactNode;
}): ReactNode {
  const lists = useClaimTabLists(data);
  const { namespace, name } = data;
  const namespaceObject = namespaces.find(
    (n) => n.metadata?.name === namespace,
  );
  const environmentName =
    namespaceObject && environmentNameOf(namespaceObject, environments);
  const environment = environments.find(
    (e) => e.metadata?.name === environmentName,
  );
  const unread = [
    ...new Set([
      ...pageUnread.filter((k) => PAGE_KINDS.includes(k as KindName)),
      ...lists.unread,
    ]),
  ];
  const sources: PlaneClaimSources = {
    name,
    namespace,
    plane,
    claim,
    environment,
    environments,
    namespaces,
    optimizes,
    clusters: lists.clusters,
    platformConfigs: lists.platformConfigs,
    sections: lists.sections,
    registrations: lists.registrations,
    requests: lists.requests,
    unread,
    waiting: [
      ...new Set([
        ...pageStale.filter((k) => PAGE_KINDS.includes(k as KindName)),
        ...lists.waiting,
      ]),
    ],
    claimUnread: unread.some((k) => OBJECT_KINDS.includes(k)),
  };

  // The locks follow the claim after the pending changes, which the bridge
  // below the session keeps here, and a mode that moveIdentity is moving to.
  const afterRef = useRef<PlaneClaim | undefined>(claim);
  const movingRef = useRef<IdentityModeId | undefined>(undefined);
  const liveModeRef = useRef(claim?.spec?.management?.identityProvider);
  useEffect(() => {
    liveModeRef.current = claim?.spec?.management?.identityProvider;
  });
  const sourcesRef = useRef(sources);
  useEffect(() => {
    sourcesRef.current = sources;
  });
  const [locks] = useState<LockProvider>(() => (path: FieldPath) => {
    const after = afterRef.current;
    if (!after) return undefined;
    const { plane, registrations, environment } = sourcesRef.current;
    const liveOidc = identityModeOf(liveModeRef.current) === "oidc";
    if (!liveOidc && PAIR_PATHS.includes(path)) return undefined;
    return lockedFields(after, movingRef.current, {
      plane,
      registrations,
      environment,
    })(path);
  });

  const context = planeClaimContext(sources);
  const validate = (s: Session<PlaneClaim>): readonly FieldError[] =>
    unread.length > 0
      ? [{ message: changesWaitText(unread, sources.waiting) }]
      : validatePlaneClaim(s, context);

  // The impacts read the claim that the save leaves: a field that another
  // field manager keeps stays.
  const review = (s: Session<PlaneClaim>): ClaimReviewInput => {
    const { kept, applied } = savedOf(s, writeMode);
    const nothing =
      kept.length > 0 && equalJSON(claimAfter(applied).spec, s.claim.spec);
    const input = planeChangeInput(sources, applied);
    return {
      impacts: [...planeImpacts(input), ...planeRestarts(input)],
      ...(planeSignInMoves(input) && { confirm: { typed: name } }),
      ...(kept.length > 0 && {
        caution: <KeptFields kept={kept} nothing={nothing} />,
      }),
    };
  };
  const host: ClaimEditHost<PlaneClaim> = {
    claim,
    resource:
      plane ??
      (claim ? { metadata: { labels: { [CLAIM_LABEL]: name } } } : undefined),
    writeMode: actions ? writeMode : undefined,
    locks,
    validate,
    review,
  };

  if (!actions)
    return (
      <PlaneClaimEditContext.Provider
        value={{ ...sources, moveIdentity: () => undefined }}
      >
        {children}
      </PlaneClaimEditContext.Provider>
    );
  return (
    <ClaimEditProvider host={host} actions={actions}>
      <EditBridge
        sources={sources}
        writeMode={writeMode}
        afterRef={afterRef}
        movingRef={movingRef}
      >
        {children}
      </EditBridge>
    </ClaimEditProvider>
  );
}

/**
 * KEPT_FIELDS_NOTE leads the list of the fields that a change leaves out
 * while another field manager owns them.
 */
export const KEPT_FIELDS_NOTE =
  "The save keeps these fields, because another tool also set them, and the fleet manager does not take a field over. Remove them with that tool:";
/** NOTHING_CHANGES is the note of a save that every kept field undoes. */
export const NOTHING_CHANGES =
  "This save changes nothing, because each field that it removes stays.";

// KeptFields shows each field of kept with its field managers.
function KeptFields({
  kept,
  nothing,
}: {
  kept: readonly KeptField[];
  nothing: boolean;
}) {
  return (
    <div className="flex flex-col gap-1">
      {nothing && <p>{NOTHING_CHANGES}</p>}
      <p>{KEPT_FIELDS_NOTE}</p>
      <ul className="list-disc pl-5">
        {kept.map((k) => (
          <li key={k.path}>
            <code className="text-xs">{k.path}</code> ({k.managers.join(", ")})
          </li>
        ))}
      </ul>
    </div>
  );
}

/** usePlaneClaimEdit returns the claim edit state of the plane page. It throws without a PlaneClaimEditProvider above it. */
export function usePlaneClaimEdit(): PlaneClaimEdit {
  const edit = useContext(PlaneClaimEditContext);
  if (!edit)
    throw new Error("usePlaneClaimEdit needs a PlaneClaimEditProvider.");
  return edit;
}

/**
 * savedOf returns the fields that a save of s in writeMode keeps for
 * another field manager, and a session whose change gives the claim that
 * the save leaves. It keeps no field when the manager of the save is not
 * known.
 */
export function savedOf(
  s: Session<PlaneClaim>,
  writeMode: WriteMode | undefined,
): { kept: KeptField[]; applied: Session<PlaneClaim> } {
  const manager = saveManager(s.claim, writeMode);
  if (manager === undefined) return { kept: [], applied: s };
  return {
    kept: keptFields(s, manager),
    applied: appliedSession(s, manager),
  };
}

/** planeClaimContext returns what validatePlaneClaim reads of sources. */
export function planeClaimContext(
  sources: PlaneClaimSources,
): PlaneClaimContext {
  return {
    plane: sources.plane,
    environment: sources.environment,
    environments: sources.environments,
    platformConfigs: sources.platformConfigs,
    sections: sources.sections,
    registrations: sources.registrations,
    requests: sources.requests,
  };
}

/** planeChangeInput returns what planeImpacts reads of sources for session. */
export function planeChangeInput(
  sources: PlaneClaimSources,
  session: Session<PlaneClaim>,
): PlaneChangeInput {
  return {
    session,
    plane: sources.plane,
    environment: sources.environment,
    platformConfigs: sources.platformConfigs,
    clusters: sources.clusters,
    namespaces: sources.namespaces,
    sections: sources.sections,
    optimizes: sources.optimizes,
  };
}

/**
 * planeFieldsOf returns planeFields of claim, with the objects of sources
 * that planeFields reads.
 */
export function planeFieldsOf(
  claim: PlaneClaim,
  sources: Pick<PlaneClaimSources, "plane" | "registrations" | "environment">,
): PlaneField[] {
  return planeFields({
    claim,
    plane: sources.plane,
    registrations: generated<PluginRegistration[]>(sources.registrations),
    environment: generated<Environment | undefined>(sources.environment),
  });
}

function EditBridge({
  sources,
  writeMode,
  afterRef,
  movingRef,
  children,
}: {
  sources: PlaneClaimSources;
  writeMode: WriteMode | undefined;
  afterRef: { current: PlaneClaim | undefined };
  movingRef: { current: IdentityModeId | undefined };
  children: ReactNode;
}) {
  const edit = useClaimEdit<PlaneClaim>();
  const session = edit.session;
  const after = useMemo(
    () => (session ? claimAfter(session) : sources.claim),
    [session, sources.claim],
  );
  useEffect(() => {
    afterRef.current = after;
  });
  const { set, reset } = edit;
  const live = session?.claim;
  const moveIdentity = useCallback(
    (to: IdentityModeId, external?: ExternalKeycloakDraft) => {
      const from = afterRef.current;
      if (!from?.spec) return undefined;
      movingRef.current = to;
      try {
        // A move back to the mode of the live claim takes back the
        // identity changes, so the live administrator returns.
        const edits =
          live && identityModeOf(live.spec?.management?.identityProvider) === to
            ? IDENTITY_STEPS.map((steps) => {
                const value = valueAt(live, steps);
                return value === undefined
                  ? { steps, reset: true as const }
                  : { steps, value };
              })
            : [
                ...identityMoveEdits(from.spec, to, external),
                ...(to === "oidc" ? recordedPairEdits(sources.plane) : []),
              ];
        for (const e of edits) {
          const lock = "reset" in e ? reset(e.steps) : set(e.steps, e.value);
          if (lock) return lock;
        }
        return undefined;
      } finally {
        movingRef.current = undefined;
      }
    },
    [afterRef, movingRef, live, set, reset, sources.plane],
  );
  return (
    <PlaneClaimEditContext.Provider
      value={{ ...sources, edit, writeMode, moveIdentity }}
    >
      {children}
    </PlaneClaimEditContext.Provider>
  );
}

const PAIR_PATHS: readonly FieldPath[] = [
  "spec.management.identity.admin.claimName",
  "spec.management.identity.admin.claimValue",
];

// recordedPairEdits sets the claim pair that Management Identity of plane
// started with, as the operator records it: "<claimName>=<claimValue>".
function recordedPairEdits(
  plane: PlaneClaimSources["plane"],
): { steps: Step[]; value: string }[] {
  const recorded =
    plane?.metadata?.annotations?.[IDENTITY_INITIAL_CLAIM_ANNOTATION];
  const split = recorded?.indexOf("=") ?? -1;
  if (!recorded || split < 0) return [];
  const steps = (name: string) =>
    ["spec", "management", "identity", "admin", name].map((field) => ({
      field,
    }));
  return [
    { steps: steps("claimName"), value: recorded.slice(0, split) },
    { steps: steps("claimValue"), value: recorded.slice(split + 1) },
  ];
}

// lockedFields returns planeLocks of the fields of claim, read in the mode
// moving when it is set.
function lockedFields(
  claim: PlaneClaim,
  moving: IdentityModeId | undefined,
  sources: Pick<PlaneClaimSources, "plane" | "registrations" | "environment">,
): LockProvider {
  const spec = claim.spec;
  const read =
    moving && spec?.management
      ? {
          ...claim,
          spec: {
            ...spec,
            management: {
              ...spec.management,
              identityProvider: { [moving]: {} },
            },
          },
        }
      : claim;
  return planeLocks(planeFieldsOf(read as PlaneClaim, sources));
}

// useClaimTabLists follows the lists of data.claimTab live. unread names
// each kind whose read failed or whose list is not current, and waiting the
// kinds whose list is not current, by the rule of the cluster page. Before
// its first answer, a list is the read of the server.
function useClaimTabLists(data: PlanePageData) {
  const initial = data.claimTab;
  const { namespace, name } = data;
  const requestOpts = {
    namespace,
    labelSelector: `${CLAIM_LABEL}=${name}`,
  };
  const results = [
    [
      "CamundaCluster",
      useResources("CamundaCluster", {}, initial?.clusters ?? NONE),
    ],
    [
      "CamundaPlatformConfig",
      useResources(
        "CamundaPlatformConfig",
        {},
        initial?.platformConfigs ?? NONE,
      ),
    ],
    [
      "PluginSection",
      useResources("PluginSection", {}, initial?.sections ?? NONE),
    ],
    [
      "PluginRegistration",
      useResources("PluginRegistration", {}, initial?.registrations ?? NONE),
    ],
    [
      "PluginRequest",
      useResources("PluginRequest", requestOpts, initial?.requests ?? NONE),
    ],
  ] as const;
  const followed = [
    { kind: "CamundaCluster", opts: {} },
    { kind: "CamundaPlatformConfig", opts: {} },
    { kind: "PluginSection", opts: {} },
    { kind: "PluginRegistration", opts: {} },
    { kind: "PluginRequest", opts: requestOpts },
  ] as const;
  const fresh = useWatchesFresh(followed);
  const overdue = useWatchesOverdue(followed);
  const [answered, setAnswered] = useState<ReadonlySet<string>>(new Set());
  const added = followed
    .filter(({ kind }, i) => fresh[i] && !answered.has(kind))
    .map(({ kind }) => kind);
  if (added.length > 0) setAnswered(new Set([...answered, ...added]));
  const waiting = followed
    .filter(({ kind }, i) =>
      answered.has(kind) ? !fresh[i] : overdue[i] || !initial,
    )
    .map(({ kind }) => kind);
  const unread = [
    ...results.flatMap(([kind, r]) => (r.error ? [kind] : [])),
    ...(initial?.readErrors ?? [])
      .map((e) => e.kind)
      .filter((kind) => !answered.has(kind)),
    ...waiting,
  ];
  return {
    clusters: results[0][1].items,
    platformConfigs: results[1][1].items,
    sections: results[2][1].items,
    registrations: results[3][1].items,
    requests: results[4][1].items,
    unread: [...new Set(unread)],
    waiting,
  };
}

// generated gives an object of the cache the generated type that the
// fields library takes. The two differ only in their timestamps, which
// planeFields does not read.
function generated<T>(value: unknown): T {
  return value as T;
}
