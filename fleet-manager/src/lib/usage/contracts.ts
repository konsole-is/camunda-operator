// This module runs in the browser and on the server.
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaManagementCluster } from "@/generated/crds/camundamanagementcluster";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { CamundaOptimize } from "@/generated/crds/camundaoptimize";
import type { Database } from "@/generated/crds/database";
import type { DatabaseConfig } from "@/generated/crds/databaseconfig";
import type { DatabaseServer } from "@/generated/crds/databaseserver";
import type { DatabaseServerPreset } from "@/generated/crds/databaseserverpreset";
import type { ElasticsearchCluster } from "@/generated/crds/elasticsearchcluster";
import type { ElasticsearchClusterPreset } from "@/generated/crds/elasticsearchclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import type { LogicalBackupElasticsearch } from "@/generated/crds/logicalbackupelasticsearch";
import type { LogicalBackupRDBMS } from "@/generated/crds/logicalbackuprdbms";
import type { ObjectStorageConfig } from "@/generated/crds/objectstorageconfig";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { PointInTimeRestore } from "@/generated/crds/pointintimerestore";
import type { SecondaryStorageConfig } from "@/generated/crds/secondarystorageconfig";
import {
  MANAGEMENT_CLUSTER_LABEL,
  MANAGEMENT_CLUSTER_NAMESPACE_LABEL,
  REGISTRATION_LABEL,
  REQUEST_LABEL,
  REQUEST_NAMESPACE_LABEL,
} from "@/lib/labels";
import { SLOT_FIELDS } from "@/lib/fields/slots";
import { ownerName } from "@/lib/labels/owner-name";
import { mergeDatabaseServer, mergeElasticsearch } from "@/lib/merge";
import {
  BUILTIN_OPTIONS,
  byCodeUnit,
  decodeObject,
  environmentNameOf,
  nameOf,
  own,
  readyOutputs,
  type ResolvedPolicy,
} from "@/lib/policy";
import { requestedParameters } from "./requested";

/** CONTRACT_KINDS are the kinds of the connection contracts, in the order of the Connections tabs. */
export const CONTRACT_KINDS = [
  "ObjectStorageConfig",
  "SecondaryStorageConfig",
  "DatabaseServerConfig",
  "DatabaseConfig",
  "ManagementAuthConfig",
] as const;

/** ContractKind is one kind of CONTRACT_KINDS. ManagementAuthConfig is the only cluster-scoped one. */
export type ContractKind = (typeof CONTRACT_KINDS)[number];

/**
 * ContractWriter says who wrote a contract: a registration through its
 * request, a registration whose request is gone ("kept"), a management
 * plane, the controller that owns the contract, or a person.
 */
export type ContractWriter =
  | { by: "registration"; registration: string }
  | { by: "kept"; registration: string }
  | { by: "plane"; namespace: string; name: string }
  | { by: "owner"; kind: string; name: string }
  | { by: "hand" };

type NamedObject = { metadata: { namespace?: string; name?: string } };

type WriterMeta = {
  metadata: {
    namespace?: string;
    labels?: Record<string, string>;
    ownerReferences?: readonly {
      kind: string;
      name: string;
      controller?: boolean;
    }[];
  };
};

/**
 * contractWriter returns the writer of contract. The first rule that
 * matches wins: the plugin labels name a registration, the management
 * labels name a plane, a controller owner reference names its owner, and
 * else a person wrote it. A contract whose request label names a request
 * that requests does not hold is kept from its registration. With requests
 * undefined, the request list is not known, and a contract with the plugin
 * labels stays with its registration. A plane of planes gives its own name;
 * else the plane shows the label value, which differs from the plane name
 * when that name is longer than 63 characters.
 */
export function contractWriter(
  contract: WriterMeta,
  requests: readonly NamedObject[] | undefined,
  planes: readonly NamedObject[] = [],
): ContractWriter {
  const { namespace, labels = {}, ownerReferences = [] } = contract.metadata;
  const registration = labels[REGISTRATION_LABEL];
  if (registration) {
    const request = labels[REQUEST_LABEL];
    // A cluster-scoped contract names the namespace of its request in a
    // label, because it has no namespace of its own.
    const requestNamespace = labels[REQUEST_NAMESPACE_LABEL] ?? namespace;
    const gone =
      request &&
      requests !== undefined &&
      !requests.some(
        (r) =>
          r.metadata.name === request &&
          r.metadata.namespace === requestNamespace,
      );
    return gone
      ? { by: "kept", registration }
      : { by: "registration", registration };
  }
  const plane = labels[MANAGEMENT_CLUSTER_LABEL];
  if (plane) {
    const planeNamespace =
      labels[MANAGEMENT_CLUSTER_NAMESPACE_LABEL] ?? namespace ?? "";
    const known = planes.find(
      (p) =>
        p.metadata.namespace === planeNamespace &&
        ownerName(p.metadata.name ?? "") === plane,
    );
    return {
      by: "plane",
      namespace: planeNamespace,
      name: known?.metadata.name ?? plane,
    };
  }
  const owner = ownerReferences.find((ref) => ref.controller === true);
  if (owner) return { by: "owner", kind: owner.kind, name: owner.name };
  return { by: "hand" };
}

/** writerLabel returns the words that the pages show for writer. */
export function writerLabel(writer: ContractWriter): string {
  switch (writer.by) {
    case "registration":
      return `Registration ${writer.registration}`;
    case "kept":
      return `Kept from ${writer.registration}`;
    case "plane":
      return `Management plane ${writer.namespace}/${writer.name}`;
    case "owner":
      return `${writer.kind} ${writer.name}`;
    case "hand":
      return "Created by hand";
  }
}

/** writerEditable says whether a person may change or delete a contract of writer: one created by hand or kept from a registration. */
export function writerEditable(writer: ContractWriter): boolean {
  return writer.by === "hand" || writer.by === "kept";
}

/** ContractRef names one contract. namespace is unset for a ManagementAuthConfig. */
export type ContractRef = {
  kind: ContractKind;
  namespace?: string;
  name: string;
};

/** ContractUser is one object that names a contract. fields are the paths that name it, sorted. */
export type ContractUser = {
  kind: string;
  namespace: string;
  name: string;
  fields: string[];
  /** preset names the preset whose layer sets every spec field of fields. */
  preset?: string;
  ready?: {
    status: "True" | "False" | "Unknown";
    reason?: string;
    message?: string;
  };
  environment?: string;
};

/**
 * ContractUserSources holds every list that can name a contract, in all
 * namespaces, with the lists that give a user its preset and its
 * environment, and the lists that resolve the parameters of a claim.
 */
export type ContractUserSources = {
  namespaces: readonly {
    metadata?: { name?: string; labels?: Record<string, string> };
  }[];
  environments: readonly Environment[];
  clusters: readonly CamundaCluster[];
  clusterClaims: readonly CamundaClusterClaim[];
  elasticsearchClusters: readonly ElasticsearchCluster[];
  elasticsearchPresets: readonly ElasticsearchClusterPreset[];
  databaseServers: readonly DatabaseServer[];
  databasePresets: readonly DatabaseServerPreset[];
  databases: readonly Database[];
  databaseConfigs: readonly DatabaseConfig[];
  secondaryStorageConfigs: readonly SecondaryStorageConfig[];
  objectStorageConfigs: readonly ObjectStorageConfig[];
  optimizes: readonly CamundaOptimize[];
  planes: readonly CamundaManagementCluster[];
  elasticsearchBackups: readonly LogicalBackupElasticsearch[];
  rdbmsBackups: readonly LogicalBackupRDBMS[];
  pointInTimeRestores: readonly PointInTimeRestore[];
  planeClaims: readonly CamundaManagementClusterClaim[];
  registrations: readonly PluginRegistration[];
  sections: readonly PluginSection[];
  requests: readonly SelectingRequest[];
};

type SelectingRequest = {
  metadata: { namespace?: string; name?: string; deletionTimestamp?: unknown };
  spec: { section?: string; registration: string; parameters?: unknown };
  status?: {
    outputs?: Record<string, string>;
    conditions?: readonly Condition[];
  };
};

type SelectingClaim = {
  metadata: { namespace?: string; name?: string };
  spec: {
    plugins?: { section: string; registration: string; parameters?: unknown }[];
  };
  status?: {
    resolvedPolicy?: ResolvedPolicy;
    conditions?: readonly Condition[];
  };
};

/** contractKey is the key of a contract in the map of contractUsage: "<kind>/<namespace>/<name>". */
export function contractKey(ref: ContractRef): string {
  return `${ref.kind}/${ref.namespace ?? ""}/${ref.name}`;
}

// A reference is one field of a user that names a contract. fromPreset is
// set on a spec field whose value the preset layer gives.
type Reference = {
  kind: ContractKind;
  name: string | undefined;
  field: string;
  fromPreset?: boolean;
};

type User = {
  kind?: string;
  metadata: { namespace?: string; name?: string };
  status?: { conditions?: readonly Condition[] };
};

type Condition = {
  type: string;
  status: "True" | "False" | "Unknown";
  reason?: string;
  message?: string;
};

/**
 * contractUsage returns the users of each contract that any object names, by
 * contractKey. A namespaced reference resolves in the namespace of the user.
 * A claim of either kind names a contract through a parameter of a built-in
 * registration that publishes contracts already in its namespace, such as
 * existing-object-storage: by the parameter that its request gets from the
 * claim policy, else, when the policy does not resolve, by the one of its
 * selection and by the one that the policy alone gives its request. It names
 * the contract this way before its cluster or plane exists. A request of
 * such a registration names it by its parameter. A request of any other
 * registration names each contract that a status output of the slot of its
 * section gives, unless the request wrote that contract.
 * Each list is sorted by kind, namespace, and name.
 */
export function contractUsage(
  sources: ContractUserSources,
): Map<string, ContractUser[]> {
  // presetFields holds, for each user of a contract, whether the preset
  // layer gives each of its spec fields.
  type Found = { user: ContractUser; presetFields: boolean[] };
  const found = new Map<string, Map<string, Found>>();
  const add = (
    kind: string,
    user: User,
    references: readonly Reference[],
    preset?: string,
  ) => {
    const namespace = user.metadata.namespace ?? "";
    const name = user.metadata.name ?? "";
    for (const reference of references) {
      if (!reference.name) continue;
      const key = contractKey({
        kind: reference.kind,
        namespace:
          reference.kind === "ManagementAuthConfig" ? undefined : namespace,
        name: reference.name,
      });
      const users = found.get(key) ?? new Map<string, Found>();
      found.set(key, users);
      const userKey = `${kind}/${namespace}/${name}`;
      let entry = users.get(userKey);
      if (!entry) {
        const ready = readyOf(user);
        const environment = environmentOfNamespace(namespace);
        entry = {
          user: {
            kind,
            namespace,
            name,
            fields: [],
            ...(preset && { preset }),
            ...(ready && { ready }),
            ...(environment && { environment }),
          },
          presetFields: [],
        };
        users.set(userKey, entry);
      }
      entry.user.fields.push(reference.field);
      if (reference.fromPreset !== undefined) {
        entry.presetFields.push(reference.fromPreset);
      }
    }
  };

  const environments = new Map<string, string | undefined>();
  const environmentOfNamespace = (namespace: string) => {
    if (!environments.has(namespace)) {
      const object = sources.namespaces.find(
        (n) => n.metadata?.name === namespace,
      );
      environments.set(
        namespace,
        object && environmentNameOf(object, sources.environments),
      );
    }
    return environments.get(namespace);
  };

  for (const cluster of sources.clusters) {
    add("CamundaCluster", cluster, [
      {
        kind: "SecondaryStorageConfig",
        name: cluster.spec.storageRef,
        field: "spec.storageRef",
      },
      {
        kind: "ObjectStorageConfig",
        name: cluster.spec.backupStorageRef,
        field: "spec.backupStorageRef",
      },
      {
        kind: "ObjectStorageConfig",
        name: cluster.spec.documentStorageRef,
        field: "spec.documentStorageRef",
      },
    ]);
  }
  for (const claim of sources.clusterClaims) {
    add("CamundaClusterClaim", claim, [
      {
        kind: "ObjectStorageConfig",
        name: claim.spec.cluster?.documentStorageRef,
        field: "spec.cluster.documentStorageRef",
      },
      {
        kind: "ManagementAuthConfig",
        name: claim.spec.attachments?.optimize?.managementAuthConfigRef,
        field: "spec.attachments.optimize.managementAuthConfigRef",
      },
    ]);
  }
  for (const es of sources.elasticsearchClusters) {
    const preset = sources.elasticsearchPresets.find(
      (p) => p.metadata.name === es.spec.presetRef,
    );
    const merged = mergeElasticsearch(preset?.spec, es.spec);
    add(
      "ElasticsearchCluster",
      es,
      [
        {
          kind: "ObjectStorageConfig",
          name: merged.spec.snapshotStorageRef,
          field: "spec.snapshotStorageRef",
          fromPreset: merged.source["spec.snapshotStorageRef"] === "preset",
        },
      ],
      preset?.metadata.name,
    );
  }
  for (const server of sources.databaseServers) {
    const preset = sources.databasePresets.find(
      (p) => p.metadata.name === server.spec.presetRef,
    );
    const merged = mergeDatabaseServer(preset?.spec, server.spec);
    const archive = server.status?.archive;
    add(
      "DatabaseServer",
      server,
      [
        {
          kind: "ObjectStorageConfig",
          name: merged.spec.archive?.objectStorageRef,
          field: "spec.archive.objectStorageRef",
          // spec.archive merges as one block, so the block has the source.
          fromPreset: merged.source["spec.archive"] === "preset",
        },
        {
          kind: "ObjectStorageConfig",
          name: archive?.boundary?.objectStorageRef,
          field: "status.archive.boundary.objectStorageRef",
        },
        ...(archive?.history ?? []).map((record): Reference => ({
          kind: "ObjectStorageConfig",
          name: record.objectStorageRef,
          field: "status.archive.history.objectStorageRef",
        })),
        {
          kind: "ObjectStorageConfig",
          name: server.status?.recovery?.archive?.objectStorageRef,
          field: "status.recovery.archive.objectStorageRef",
        },
        {
          kind: "DatabaseServerConfig",
          name: server.status?.recovery?.completedAt
            ? undefined
            : server.status?.recovery?.contract,
          field: "status.recovery.contract",
        },
      ],
      preset?.metadata.name,
    );
  }
  for (const backup of sources.elasticsearchBackups) {
    add("LogicalBackupElasticsearch", backup, [
      {
        kind: "ObjectStorageConfig",
        name: backup.status?.storage?.bucketRef,
        field: "status.storage.bucketRef",
      },
      {
        kind: "SecondaryStorageConfig",
        name: backup.status?.storage?.secondaryStorageConfig,
        field: "status.storage.secondaryStorageConfig",
      },
    ]);
  }
  for (const backup of sources.rdbmsBackups) {
    add("LogicalBackupRDBMS", backup, [
      {
        kind: "ObjectStorageConfig",
        name: backup.status?.bucketRef,
        field: "status.bucketRef",
      },
    ]);
  }
  for (const restore of sources.pointInTimeRestores) {
    const phase = restore.status?.phase;
    if (phase === "Completed" || phase === "Failed") continue;
    add("PointInTimeRestore", restore, [
      {
        kind: "DatabaseServerConfig",
        name: restore.status?.storage?.databaseServerConfig,
        field: "status.storage.databaseServerConfig",
      },
    ]);
  }
  for (const database of sources.databases) {
    add("Database", database, [
      {
        kind: "DatabaseServerConfig",
        name: database.spec.serverRef,
        field: "spec.serverRef",
      },
    ]);
  }
  for (const config of sources.databaseConfigs) {
    add("DatabaseConfig", config, [
      {
        kind: "DatabaseServerConfig",
        name: config.spec.serverRef,
        field: "spec.serverRef",
      },
    ]);
  }
  for (const config of sources.secondaryStorageConfigs) {
    add("SecondaryStorageConfig", config, [
      {
        kind: "DatabaseConfig",
        name: config.spec.rdbms?.databaseConfigRef,
        field: "spec.rdbms.databaseConfigRef",
      },
    ]);
  }
  for (const plane of sources.planes) {
    add("CamundaManagementCluster", plane, [
      {
        kind: "DatabaseConfig",
        name: plane.spec.identityProvider?.keycloak?.databaseConfigRef,
        field: "spec.identityProvider.keycloak.databaseConfigRef",
      },
      {
        kind: "DatabaseConfig",
        name: plane.spec.identity?.databaseConfigRef,
        field: "spec.identity.databaseConfigRef",
      },
      {
        kind: "DatabaseConfig",
        name: plane.spec.webModeler?.databaseConfigRef,
        field: "spec.webModeler.databaseConfigRef",
      },
    ]);
  }
  for (const claim of sources.clusterClaims) {
    add("CamundaClusterClaim", claim, selectedContracts(claim, sources));
  }
  for (const claim of sources.planeClaims) {
    add(
      "CamundaManagementClusterClaim",
      claim,
      selectedContracts(claim, sources),
    );
  }
  for (const request of sources.requests) {
    add("PluginRequest", request, [
      ...contractParameters(
        request.spec.registration,
        request.spec.parameters,
      ).map(([kind, name, parameter]) => ({
        kind,
        name,
        field: `spec.parameters.${parameter}`,
      })),
      ...outputContracts(request, sources),
    ]);
  }
  for (const optimize of sources.optimizes) {
    add("CamundaOptimize", optimize, [
      {
        kind: "ManagementAuthConfig",
        name: optimize.spec.managementAuthRef,
        field: "spec.managementAuthRef",
      },
    ]);
  }

  const usage = new Map<string, ContractUser[]>();
  for (const [key, users] of found) {
    usage.set(
      key,
      [...users.values()]
        .map(({ user, presetFields }) => {
          const { preset, ...rest } = user;
          const byPreset =
            presetFields.length > 0 && presetFields.every(Boolean);
          return {
            ...rest,
            fields: [...new Set(user.fields)].sort(byCodeUnit),
            ...(byPreset && preset && { preset }),
          };
        })
        .sort(
          (a, b) =>
            byCodeUnit(a.kind, b.kind) ||
            byCodeUnit(a.namespace, b.namespace) ||
            byCodeUnit(a.name, b.name),
        ),
    );
  }
  return usage;
}

// selectedContracts returns the contracts that the selections of claim name.
function selectedContracts(
  claim: SelectingClaim,
  sources: ContractUserSources,
): Reference[] {
  const selections = claim.spec.plugins ?? [];
  const selects = selections.some((s) =>
    Object.values(own(BUILTIN_OPTIONS, s.registration)?.lists ?? {}).some(
      isContractKind,
    ),
  );
  if (!selects) return [];
  return selections.flatMap((s) =>
    requestedParameters(claim, s, sources).flatMap((parameters) =>
      contractParameters(s.registration, parameters).map(
        ([kind, name, parameter]) => ({
          kind,
          name,
          field: `spec.plugins.${s.registration}.parameters.${parameter}`,
        }),
      ),
    ),
  );
}

// contractParameters returns each parameter of parameters that names a
// contract by the built-in entry of registration, with its contract kind
// and the name.
function contractParameters(
  registration: string,
  parameters: unknown,
): [ContractKind, string, string][] {
  const lists = own(BUILTIN_OPTIONS, registration)?.lists ?? {};
  const values = decodeObject(parameters) ?? {};
  return Object.entries(lists).flatMap(([parameter, kind]) => {
    const name = nameOf(values[parameter]);
    return isContractKind(kind) && name ? [[kind, name, parameter]] : [];
  });
}

// OUTPUT_CONTRACTS gives the contract kind of each output of SLOT_FIELDS
// that fills a contract reference.
const OUTPUT_CONTRACTS: Readonly<Record<string, ContractKind>> = {
  objectStorageConfig: "ObjectStorageConfig",
  secondaryStorageConfig: "SecondaryStorageConfig",
  keycloakDatabaseConfig: "DatabaseConfig",
  identityDatabaseConfig: "DatabaseConfig",
  webModelerDatabaseConfig: "DatabaseConfig",
};

// outputContracts returns the contracts that the status outputs of request
// name through the slot of its section. A built-in registration names its
// contracts by its parameters instead.
function outputContracts(
  request: SelectingRequest,
  sources: ContractUserSources,
): Reference[] {
  if (own(BUILTIN_OPTIONS, request.spec.registration)) return [];
  const slot = sources.sections.find(
    (s) => s.metadata?.name === request.spec.section,
  )?.spec.slot;
  const outputs = request.status?.outputs ?? {};
  const outputKeys = new Set(
    SLOT_FIELDS.filter((row) => slot && row.slot === slot).map(
      (row) => row.output,
    ),
  );
  return [...outputKeys].flatMap((output) => {
    const kind = own(OUTPUT_CONTRACTS, output);
    const name = own(outputs, output);
    if (!kind || !name || writtenBy(request, kind, name, sources)) return [];
    return [{ kind, name, field: `status.outputs.${output}` }];
  });
}

type LabeledObject = {
  metadata: {
    namespace?: string;
    name?: string;
    labels?: Record<string, string>;
  };
};

// writtenBy says whether the request wrote the contract kind/name in its
// namespace.
function writtenBy(
  request: SelectingRequest,
  kind: ContractKind,
  name: string,
  sources: ContractUserSources,
): boolean {
  const lists: Partial<Record<ContractKind, readonly LabeledObject[]>> = {
    ObjectStorageConfig: sources.objectStorageConfigs,
    SecondaryStorageConfig: sources.secondaryStorageConfigs,
    DatabaseConfig: sources.databaseConfigs,
  };
  const namespace = request.metadata.namespace;
  const contract = (lists[kind] ?? []).find(
    (c) => c.metadata.name === name && c.metadata.namespace === namespace,
  );
  if (!contract) return false;
  const writer = contractWriter(contract, sources.requests, sources.planes);
  const labels = contract.metadata.labels ?? {};
  return (
    writer.by === "registration" &&
    writer.registration === request.spec.registration &&
    labels[REQUEST_LABEL] === request.metadata.name &&
    (labels[REQUEST_NAMESPACE_LABEL] ?? namespace) === namespace
  );
}

/**
 * WaitingRequest is a request whose outputs can name a contract that the
 * fleet manager does not know yet. request is "PluginRequest
 * <namespace>/<name>". section is set when the section of the request does
 * not exist.
 */
export type WaitingRequest = { request: string; section?: string };

/**
 * waitingRequests returns each request in the namespace of ref whose
 * outputs can name a contract of its kind and that the fleet manager cannot
 * check yet, sorted by request. The users of ref are not known while one
 * exists. A request counts while its section has a slot output of that kind
 * and readyOutputs does not give its outputs, whatever their values, and
 * while its section is not in sources. A request of a built-in registration
 * and a request that is being deleted do not count. It returns no request
 * while unread holds PluginRequest or PluginSection, the lists that the rule
 * reads.
 */
export function waitingRequests(
  ref: ContractRef,
  sources: ContractUserSources,
  unread: readonly string[],
): WaitingRequest[] {
  if (WAITING_KINDS.some((k) => unread.includes(k))) return [];
  const slots = new Set(
    SLOT_FIELDS.filter(
      (row) => own(OUTPUT_CONTRACTS, row.output) === ref.kind,
    ).map((row) => row.slot),
  );
  if (slots.size === 0) return [];
  return sources.requests
    .flatMap((request): WaitingRequest[] => {
      if (
        request.metadata.namespace !== ref.namespace ||
        request.metadata.deletionTimestamp ||
        own(BUILTIN_OPTIONS, request.spec.registration)
      ) {
        return [];
      }
      const name = `PluginRequest ${request.metadata.namespace}/${request.metadata.name}`;
      const section = sources.sections.find(
        (s) => s.metadata?.name === request.spec.section,
      );
      if (!section) {
        return [{ request: name, section: request.spec.section ?? "" }];
      }
      const slot = section.spec.slot;
      const ready = readyOutputs(
        [request as PluginRequest],
        request.spec.section ?? "",
        request.spec.registration,
      );
      return slot && slots.has(slot) && !ready ? [{ request: name }] : [];
    })
    .sort((a, b) => byCodeUnit(a.request, b.request));
}

const WAITING_KINDS = ["PluginRequest", "PluginSection"];

/**
 * waitingDeleteText says why a delete is off while requests wait: for each
 * request, that its outputs are not given for its current spec yet, or that
 * its section does not exist.
 */
export function waitingDeleteText(waiting: readonly WaitingRequest[]): string {
  return `Delete is possible when the fleet manager knows the users. ${waitingCauses(waiting)}`;
}

/** waitingUsersText says that the users of a contract are not known while requests wait, and why. */
export function waitingUsersText(waiting: readonly WaitingRequest[]): string {
  return `The users are not known. ${waitingCauses(waiting)}`;
}

function waitingCauses(waiting: readonly WaitingRequest[]): string {
  return waiting
    .map((w) =>
      w.section === undefined
        ? `${w.request} has not given its outputs for its current spec yet.`
        : `The section ${w.section} of ${w.request} does not exist.`,
    )
    .join(" ");
}

function isContractKind(kind: string): kind is ContractKind {
  return (CONTRACT_KINDS as readonly string[]).includes(kind);
}

// USER_KINDS holds, for each contract kind, the kinds whose objects
// contractUsage reads for its users, the presets of those objects included,
// and the lists that resolve the parameters of a claim.
// SELECTION_KINDS are the lists that give the contracts that a claim
// selects through a registration, and that a request names in its outputs.
const SELECTION_KINDS = [
  "Environment",
  "Namespace",
  "PluginRegistration",
  "PluginRequest",
  "PluginSection",
];

const USER_KINDS: Record<ContractKind, readonly string[]> = {
  ObjectStorageConfig: [
    "CamundaCluster",
    "CamundaClusterClaim",
    "DatabaseServer",
    "DatabaseServerPreset",
    "ElasticsearchCluster",
    "ElasticsearchClusterPreset",
    "LogicalBackupElasticsearch",
    "LogicalBackupRDBMS",
    ...SELECTION_KINDS,
  ],
  SecondaryStorageConfig: [
    "CamundaCluster",
    "CamundaClusterClaim",
    "LogicalBackupElasticsearch",
    ...SELECTION_KINDS,
  ],
  DatabaseServerConfig: [
    "Database",
    "DatabaseConfig",
    "DatabaseServer",
    "PointInTimeRestore",
  ],
  DatabaseConfig: [
    "CamundaManagementCluster",
    "CamundaManagementClusterClaim",
    "SecondaryStorageConfig",
    ...SELECTION_KINDS,
  ],
  ManagementAuthConfig: ["CamundaClusterClaim", "CamundaOptimize"],
};

/**
 * unreadUserKinds returns the kinds of unread that can use a contract of
 * kind, sorted. The users of such a contract that contractUsage gives are
 * then not complete.
 */
export function unreadUserKinds(
  kind: ContractKind,
  unread: readonly string[],
): string[] {
  return USER_KINDS[kind].filter((k) => unread.includes(k)).sort(byCodeUnit);
}

function readyOf(user: User): ContractUser["ready"] {
  const ready = user.status?.conditions?.find((c) => c.type === "Ready");
  if (!ready) return undefined;
  return {
    status: ready.status,
    ...(ready.reason && { reason: ready.reason }),
    ...(ready.message && { message: ready.message }),
  };
}
