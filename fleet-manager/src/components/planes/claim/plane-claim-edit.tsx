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
  type PlaneField,
  planeFields,
  type Step,
} from "@/lib/fields";
import { useResources, useWatchesFresh } from "@/lib/k8s/hooks";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import type { FieldPath } from "@/lib/merge";
import { identityMoveEdits } from "@/lib/planes/edits";
import {
  type ExternalKeycloakDraft,
  type IdentityModeId,
  identityModeOf,
} from "@/lib/planes/identity";
import {
  type PlaneChangeInput,
  planeImpacts,
  planeModeMove,
  planeRestarts,
} from "@/lib/planes/impacts";
import { environmentNameOf } from "@/lib/policy";
import {
  claimAfter,
  type FieldError,
  type LockProvider,
  type PlaneClaim,
  type PlaneClaimContext,
  planeLocks,
  type Session,
  keptFields,
  type KeptField,
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
 * read, or whose watch is out of date. claimUnread says that one of them is
 * the plane or its claim.
 */
export type PlaneClaimSources = {
  name: string;
  namespace: string;
  plane?: Resource<"CamundaManagementCluster">;
  claim?: PlaneClaim;
  environment?: Resource<"Environment">;
  environments: readonly Resource<"Environment">[];
  namespaces: readonly Resource<"Namespace">[];
  clusters: readonly Resource<"CamundaCluster">[];
  platformConfigs: readonly Resource<"CamundaPlatformConfig">[];
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  requests: readonly Resource<"PluginRequest">[];
  unread: readonly string[];
  claimUnread: boolean;
};

/**
 * PlaneClaimEdit is the claim edit state of the plane page. edit is
 * undefined without claim actions. moveIdentity moves the identity mode of
 * the claim after the pending changes to to, with the edits of
 * identityMoveEdits, and returns the lock that refused one of them. A move
 * to the mode of the live claim takes back the identity changes instead.
 * The lock on the administrator claim pair follows the mode after the move.
 */
export type PlaneClaimEdit = PlaneClaimSources & {
  edit?: ClaimEdit<PlaneClaim>;
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
 * reviewed while the page could not read kinds.
 */
export function changesWaitText(kinds: readonly string[]): string {
  return `Review waits: the page could not read ${kinds.join(", ")}, so what this change does is not known.`;
}

/**
 * PlaneClaimEditProvider holds the claim edit session of the plane page,
 * through ClaimEditProvider, over claim, the live claim of the plane. It
 * follows the lists of data.claimTab live. pageUnread are the kinds of the
 * page that it could not read, or whose watch is out of date. While a kind
 * that the checks or the impacts read is unread, the session refuses
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
  pageUnread,
  writeMode,
  actions,
  children,
}: {
  data: PlanePageData;
  plane?: Resource<"CamundaManagementCluster">;
  claim?: PlaneClaim;
  namespaces: readonly Resource<"Namespace">[];
  environments: readonly Resource<"Environment">[];
  pageUnread: readonly string[];
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
    clusters: lists.clusters,
    platformConfigs: lists.platformConfigs,
    sections: lists.sections,
    registrations: lists.registrations,
    requests: lists.requests,
    unread,
    claimUnread: unread.some((k) => OBJECT_KINDS.includes(k)),
  };

  // The locks follow the claim after the pending changes, which the bridge
  // below the session keeps here, and a mode that moveIdentity is moving to.
  const afterRef = useRef<PlaneClaim | undefined>(claim);
  const movingRef = useRef<IdentityModeId | undefined>(undefined);
  const sourcesRef = useRef(sources);
  useEffect(() => {
    sourcesRef.current = sources;
  });
  const [locks] = useState<LockProvider>(() => (path: FieldPath) => {
    const after = afterRef.current;
    if (!after) return undefined;
    const { plane, registrations, environment } = sourcesRef.current;
    return lockedFields(after, movingRef.current, {
      plane,
      registrations,
      environment,
    })(path);
  });

  const context = planeClaimContext(sources);
  const validate = (s: Session<PlaneClaim>): readonly FieldError[] =>
    unread.length > 0
      ? [{ message: changesWaitText(unread) }]
      : validatePlaneClaim(s, context);

  const review = (s: Session<PlaneClaim>): ClaimReviewInput => {
    const input = planeChangeInput(sources, s);
    const kept = keptFields(s);
    return {
      impacts: [...planeImpacts(input), ...planeRestarts(input)],
      ...(planeModeMove(s) && { confirm: { typed: name } }),
      ...(kept.length > 0 && { caution: <KeptFields kept={kept} /> }),
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
      <EditBridge sources={sources} afterRef={afterRef} movingRef={movingRef}>
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
  "These fields stay in the claim, because another tool set them. The fleet manager does not take a field over. Change them with that tool:";

// KeptFields shows each field of kept with its field manager.
function KeptFields({ kept }: { kept: readonly KeptField[] }) {
  return (
    <div className="flex flex-col gap-1">
      <p>{KEPT_FIELDS_NOTE}</p>
      <ul className="list-disc pl-5">
        {kept.map((k) => (
          <li key={k.path}>
            <code className="text-xs">{k.path}</code> ({k.manager})
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
  afterRef,
  movingRef,
  children,
}: {
  sources: PlaneClaimSources;
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
            : identityMoveEdits(from.spec, to, external);
        for (const e of edits) {
          const lock = "reset" in e ? reset(e.steps) : set(e.steps, e.value);
          if (lock) return lock;
        }
        return undefined;
      } finally {
        movingRef.current = undefined;
      }
    },
    [afterRef, movingRef, live, set, reset],
  );
  return (
    <PlaneClaimEditContext.Provider value={{ ...sources, edit, moveIdentity }}>
      {children}
    </PlaneClaimEditContext.Provider>
  );
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
// each kind whose read failed, and each kind whose watch answered once and
// is out of date now. Before its first answer, a list is the read of the
// server.
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
  const [answered, setAnswered] = useState<ReadonlySet<string>>(new Set());
  const added = followed
    .filter(({ kind }, i) => fresh[i] && !answered.has(kind))
    .map(({ kind }) => kind);
  if (added.length > 0) setAnswered(new Set([...answered, ...added]));
  const unread = [
    ...results.flatMap(([kind, r]) => (r.error ? [kind] : [])),
    ...(initial?.readErrors ?? [])
      .map((e) => e.kind)
      .filter((kind) => !answered.has(kind)),
    ...followed
      .filter(({ kind }, i) => answered.has(kind) && !fresh[i])
      .map(({ kind }) => kind),
  ];
  return {
    clusters: results[0][1].items,
    platformConfigs: results[1][1].items,
    sections: results[2][1].items,
    registrations: results[3][1].items,
    requests: results[4][1].items,
    unread: [...new Set(unread)],
  };
}

// generated gives an object of the cache the generated type that the
// fields library takes. The two differ only in their timestamps, which
// planeFields does not read.
function generated<T>(value: unknown): T {
  return value as T;
}
