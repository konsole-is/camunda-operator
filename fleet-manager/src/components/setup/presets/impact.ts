import { RESTART_ALL } from "@/components/cluster/impacts";
import type { ReadError } from "@/components/planes/planes-table";
import type { CamundaClusterPresetSpec } from "@/generated/crds/camundaclusterpreset";
import type { DatabaseServerPresetSpec } from "@/generated/crds/databaseserverpreset";
import type { ElasticsearchClusterPresetSpec } from "@/generated/crds/elasticsearchclusterpreset";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import {
  mergeCluster,
  mergeDatabaseServer,
  mergeElasticsearch,
} from "@/lib/merge";
import { parseQuantity } from "@/lib/observe/quantity";
import { equalJSON } from "@/lib/policy";
import { fingerprintMoved } from "@/lib/presets/fingerprint";
import { valueAt } from "@/lib/session";
import { type PresetKind, presetResources } from "@/lib/usage";
import type { PresetObject } from "./kinds";
import type { PresetSources } from "./sources";

/**
 * RollMark says what a preset change does to the pods of one resource.
 * "Pods may roll" is the mark of an Elasticsearch cluster or a database
 * server whose merged spec changes: the app cannot tell which of those
 * changes roll pods.
 */
export type RollMark =
  | "Pods roll"
  | "Connectors restart"
  | "No roll"
  | "Unchanged"
  | "Pods may roll";

/** ResourceImpact is the mark of one resource that inherits the preset. connectorsRestart says that its Connectors pods restart. */
export type ResourceImpact = {
  kind: "CamundaCluster" | "ElasticsearchCluster" | "DatabaseServer";
  namespace: string;
  name: string;
  mark: RollMark;
  connectorsRestart: boolean;
};

/**
 * PresetImpact is the impact of one preset change: a mark for each
 * resource that inherits the preset, and the notes of the change. unread
 * names each list that the marks need and that is not read: while it is not
 * empty, rows is empty, because the users are not known.
 */
export type PresetImpact = {
  rows: ResourceImpact[];
  notes: string[];
  unread: string[];
};

/** CONNECTORS_RESTART is the note of a change that restarts the Connectors pods of a cluster. */
export const CONNECTORS_RESTART = "Connectors pods restart.";

/** STORAGE_NOTES holds, for each preset kind, the rule of a smaller storage size, of a new storage class, and of a cleared size whose volume stays, in the words of the docs of the kind. */
export const STORAGE_NOTES: Record<
  PresetKind,
  { shrink: string; storageClass?: string; cleared?: string }
> = {
  CamundaClusterPreset: {
    shrink:
      "A smaller storage size is ignored for clusters that grew. They keep their volumes and record StorageShrinkIgnored.",
    storageClass:
      "A new storage class reaches only clusters without a broker StatefulSet, a suspended one included. The others keep their class and record StorageClassChangeIgnored.",
  },
  ElasticsearchClusterPreset: {
    shrink:
      "A smaller storage size is ignored for Elasticsearch clusters that grew. They record StorageShrinkIgnored.",
    storageClass:
      "A new storage class reaches only Elasticsearch clusters without an ECK resource. The others keep their class and record StorageClassChangeIgnored.",
  },
  DatabaseServerPreset: {
    shrink:
      "A smaller storage size is ignored for database servers that grew. They record StorageShrinkIgnored.",
    cleared:
      "A cleared walStorageSize does not remove the write-ahead log volume of a running server. The server keeps the volume and records WALStorageKept.",
  },
};

// IMPACT_KINDS are the lists that the marks of each preset kind read.
const IMPACT_KINDS: Record<PresetKind, readonly KindName[]> = {
  CamundaClusterPreset: [
    "CamundaCluster",
    "CamundaRelease",
    "CamundaPlatformConfig",
  ],
  ElasticsearchClusterPreset: ["ElasticsearchCluster"],
  DatabaseServerPreset: ["DatabaseServer"],
};

// STORAGE_FIELDS are the paths in the body of each preset kind of the
// storage sizes, of the storage class, and of the size whose volume stays
// when it is cleared.
const STORAGE_FIELDS: Record<
  PresetKind,
  {
    sizes: string[][];
    sizeDefault?: string;
    storageClass?: string[];
    kept?: string[];
  }
> = {
  CamundaClusterPreset: {
    sizes: [["zeebe", "storageSize"]],
    // The defaultStorageSize of pkg/components/camundacluster/effective.go.
    sizeDefault: "10Gi",
    storageClass: ["zeebe", "storageClassName"],
  },
  ElasticsearchClusterPreset: {
    sizes: [["storageSize"]],
    storageClass: ["storageClassName"],
  },
  DatabaseServerPreset: {
    sizes: [["storageSize"], ["walStorageSize"]],
    kept: ["walStorageSize"],
  },
};

/**
 * impactUnread returns each list that the marks of a preset of kind read
 * and that is in errors (not read) or pending (not current), in the order
 * of the kinds.
 */
export function impactUnread(
  kind: PresetKind,
  errors: readonly ReadError[],
  pending: readonly KindName[],
): KindName[] {
  return IMPACT_KINDS[kind].filter(
    (k) => pending.includes(k) || errors.some((e) => e.kind === k),
  );
}

/**
 * presetImpact returns the impact of the change of the spec of the preset
 * name of kind from before to after on each resource that inherits it,
 * sorted by namespace, then name. A cluster gets "Pods roll" when the
 * preset fingerprint moves, whatever it sets itself. connectorsRestart is
 * set when the change restarts the Connectors pods of a cluster: a new
 * rotation under basic auth, or a change of what the pods take from the
 * Connectors service account. An Elasticsearch cluster or a database
 * server gets "Unchanged" when its merged spec stays, else "Pods may roll".
 * unread are the lists of impactUnread. The notes name, in this order, the
 * restart of the brokers when a cluster rolls, the restart of the
 * Connectors pods, and the storage rule of a smaller size, a new class, or
 * a cleared size that the change touches.
 */
export function presetImpact(input: {
  kind: PresetKind;
  name: string;
  before: PresetObject["spec"];
  after: PresetObject["spec"];
  sources: PresetSources;
  unread?: readonly string[];
}): PresetImpact {
  const { kind, before, after } = input;
  const unread = [...(input.unread ?? [])];
  const rows = unread.length > 0 ? [] : rowsOf(input);
  const notes = [
    ...(rows.some((r) => r.mark === "Pods roll") ? [RESTART_ALL] : []),
    ...(rows.some((r) => r.connectorsRestart) ? [CONNECTORS_RESTART] : []),
    ...storageNotes(kind, bodyOf(kind, before), bodyOf(kind, after)),
  ];
  return { rows, notes, unread };
}

function rowsOf(input: {
  kind: PresetKind;
  name: string;
  before: PresetObject["spec"];
  after: PresetObject["spec"];
  sources: PresetSources;
}): ResourceImpact[] {
  const { kind, name, sources } = input;
  const users = presetResources(kind, name, sources);
  switch (kind) {
    case "CamundaClusterPreset": {
      const before = input.before as CamundaClusterPresetSpec;
      const after = input.after as CamundaClusterPresetSpec;
      const moved = fingerprintMoved(before, after);
      return users.flatMap((user) => {
        const cluster = sources.clusters.find(
          (c) =>
            c.metadata.namespace === user.namespace &&
            c.metadata.name === user.name,
        );
        if (!cluster) return [];
        const release = sources.releases.find(
          (r) => r.metadata.name === cluster.spec.releaseRef,
        )?.spec;
        const was = mergeCluster(before, release, cluster.spec).spec;
        const will = mergeCluster(after, release, cluster.spec).spec;
        const rotation = will.auth?.basic?.passwordRotation;
        const connectorsRestart =
          will.connectors?.enabled === true &&
          ((basicAuth(cluster, sources) &&
            !!rotation &&
            rotation !== was.auth?.basic?.passwordRotation) ||
            !equalJSON(connectorsAccount(was), connectorsAccount(will)));
        const mark: RollMark = moved
          ? "Pods roll"
          : equalJSON(was, will)
            ? "Unchanged"
            : connectorsRestart
              ? "Connectors restart"
              : "No roll";
        return [{ ...user, kind: "CamundaCluster", mark, connectorsRestart }];
      });
    }
    case "ElasticsearchClusterPreset": {
      const before = input.before as ElasticsearchClusterPresetSpec;
      const after = input.after as ElasticsearchClusterPresetSpec;
      return users.flatMap((user) => {
        const own = sources.elasticsearchClusters.find(
          (c) =>
            c.metadata.namespace === user.namespace &&
            c.metadata.name === user.name,
        );
        if (!own) return [];
        const same = equalJSON(
          mergeElasticsearch(before, own.spec).spec,
          mergeElasticsearch(after, own.spec).spec,
        );
        return [
          {
            ...user,
            kind: "ElasticsearchCluster",
            mark: same ? "Unchanged" : "Pods may roll",
            connectorsRestart: false,
          },
        ];
      });
    }
    case "DatabaseServerPreset": {
      const before = input.before as DatabaseServerPresetSpec;
      const after = input.after as DatabaseServerPresetSpec;
      return users.flatMap((user) => {
        const own = sources.databaseServers.find(
          (s) =>
            s.metadata.namespace === user.namespace &&
            s.metadata.name === user.name,
        );
        if (!own) return [];
        const same = equalJSON(
          mergeDatabaseServer(before, own.spec).spec,
          mergeDatabaseServer(after, own.spec).spec,
        );
        return [
          {
            ...user,
            kind: "DatabaseServer",
            mark: same ? "Unchanged" : "Pods may roll",
            connectorsRestart: false,
          },
        ];
      });
    }
  }
}

type MergedCluster = ReturnType<typeof mergeCluster>["spec"];

// basicAuth says whether the platform config of cluster sets basic auth or
// no method. Only then does the operator rotate the admin password.
function basicAuth(
  cluster: Resource<"CamundaCluster">,
  sources: PresetSources,
): boolean {
  const config = sources.platformConfigs.find(
    (p) => p.metadata.name === cluster.spec.platformConfigRef,
  );
  const method = config?.spec.auth?.method;
  return config !== undefined && (method === undefined || method === "basic");
}

// connectorsAccount returns what the Connectors pod template takes from
// connectors.serviceAccount in pkg/components/camundacluster.
function connectorsAccount(spec: MergedCluster): unknown {
  const account = spec.connectors?.serviceAccount;
  if (!account) return undefined;
  const annotations = account.annotations ?? {};
  return {
    hashed:
      account.create !== false && Object.keys(annotations).length > 0
        ? annotations
        : undefined,
    azure: AZURE_CLIENT_ID in annotations,
  };
}

const AZURE_CLIENT_ID = "azure.workload.identity/client-id";

function bodyOf(kind: PresetKind, spec: PresetObject["spec"]): unknown {
  const s = spec as { cluster?: unknown; server?: unknown };
  return kind === "DatabaseServerPreset" ? s.server : s.cluster;
}

function storageNotes(
  kind: PresetKind,
  before: unknown,
  after: unknown,
): string[] {
  const fields = STORAGE_FIELDS[kind];
  const at = (body: unknown, path: string[]) =>
    valueAt(
      body,
      path.map((field) => ({ field })),
    );
  const shrinks = fields.sizes.some((path) =>
    smaller(
      at(before, path) ?? fields.sizeDefault,
      at(after, path) ?? fields.sizeDefault,
    ),
  );
  const storageClass = fields.storageClass;
  const moves =
    storageClass !== undefined &&
    !equalJSON(at(before, storageClass), at(after, storageClass));
  const kept = fields.kept;
  const cleared =
    kept !== undefined &&
    at(before, kept) !== undefined &&
    at(after, kept) === undefined;
  return [
    ...(shrinks ? [STORAGE_NOTES[kind].shrink] : []),
    ...(moves && STORAGE_NOTES[kind].storageClass
      ? [STORAGE_NOTES[kind].storageClass]
      : []),
    ...(cleared && STORAGE_NOTES[kind].cleared
      ? [STORAGE_NOTES[kind].cleared]
      : []),
  ];
}

// smaller says whether two quantities are set and after is the smaller. A
// value that is not a quantity compares as not smaller: the dry run refuses
// it.
function smaller(before: unknown, after: unknown): boolean {
  const isQuantity = (v: unknown) =>
    typeof v === "string" || typeof v === "number";
  if (!isQuantity(before) || !isQuantity(after)) return false;
  try {
    return parseQuantity(String(after)) < parseQuantity(String(before));
  } catch {
    return false;
  }
}
