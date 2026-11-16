// This module runs in the browser and on the server.
import type { ConditionRow } from "@/components/fleet/conditions-table";
import {
  type GrantView,
  grantTemplates,
  grantView,
} from "@/components/setup/plugins/grants";
import {
  type Invalid,
  conditionsOf,
  isInvalid,
  isRecord,
  isText,
  itemsOf,
} from "@/components/setup/plugins/values";
import type { Grant } from "@/generated/crds/grant";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { controlledBy, requestPolicy } from "@/lib/claims/deletion";
import { resourceFate, type ResourceRow } from "@/lib/claims/resources";
import type { SectionComponent } from "@/lib/claims/sections";
import { SLOT_FIELDS } from "@/lib/fields/slots";
import type { Resource } from "@/lib/k8s/kinds";
import {
  BUILTIN_OPTIONS,
  byCodeUnit,
  deletionPolicyOf,
  type DeletionPolicy,
  own,
  splitFrom,
  targetServiceAccount,
} from "@/lib/policy";
import { REGISTRATION_LABEL } from "@/lib/labels";

/** SummaryRow is one summary item of a request. A missing severity is Info. */
export type SummaryRow = {
  label: string;
  value: string;
  severity: "Info" | "Warning" | "Error";
};

/** OutputRow is one output of a request. fills names each field it fills, as "<kind> <path>". */
export type OutputRow = { key: string; value: string; fills: string[] };

/**
 * InputRow is one input of the registration of a request: the section and
 * output it comes from, and its value, which is absent until the claim
 * controller resolved it.
 */
export type InputRow = {
  name: string;
  section: string;
  sectionLabel: string;
  output: string;
  value?: string;
};

/** LinkRow is one link of a request. */
export type LinkRow = { name: string; url: string };

/** ResourceFateRow is one resource of a request, with its fate when the claim controls the request. */
export type ResourceFateRow = ResourceRow & { fate?: "Kept" | "Deleted" };

/**
 * GrantRow is the view of one Grant with all its conditions. statement is
 * spec.fragment as indented JSON. request is set only in the grants of the
 * cloud identity. templateGone is true when the registration of the Grant
 * is in the page data and no longer has its template. errors name each
 * value of the wrong type, and a value that has one is absent.
 */
export type GrantRow = Omit<GrantView, "error"> & {
  template: string;
  statement?: string;
  conditions: ConditionRow[];
  templateGone: boolean;
  errors: string[];
};

/** ContractKind is a kind of contract that an existing-* option connects. */
export type ContractKind =
  "SecondaryStorageConfig" | "ObjectStorageConfig" | "DatabaseConfig";

/**
 * ConnectedRef is one contract that a selection connects. found is false
 * when the contract is not in the namespace of the claim, and ready is its
 * Ready condition.
 */
export type ConnectedRef = {
  kind: ContractKind;
  name: string;
  found: boolean;
  ready?: ConditionRow;
};

/** ContractObject is the part of a contract that requestDetail reads. */
export type ContractObject = {
  metadata?: { namespace?: string; name?: string };
  status?: { conditions?: unknown };
};

/**
 * RequestDetail is what the Overview of a request shows. A status list or
 * item of the wrong type is an Invalid row. controlled is false when the
 * claim does not control the request: its resources then have no fate.
 * policy is the deletion policy that decides the fates. grants are the
 * Grants of the request, or for the cloud identity every Grant of the
 * cluster, by name. connected is set only for an option that connects
 * existing contracts, and resources are then empty. connectors is set only
 * for the Connectors permissions. usedBy are the labels of the other
 * selections that take an output of this one as an input.
 */
export type RequestDetail = {
  summary: (SummaryRow | Invalid)[];
  summaryTime?: string;
  outputs: (OutputRow | Invalid)[];
  inputs: InputRow[];
  links: (LinkRow | Invalid)[];
  resources: (ResourceFateRow | Invalid)[];
  controlled: boolean;
  policy: DeletionPolicy;
  grants: GrantRow[];
  clusterGrants: boolean;
  connected?: ConnectedRef[];
  connectors?: { principal?: string; serviceAccount: string };
  usedBy: string[];
};

/** BuiltinStorage is the object that the request of built-in storage made. object is absent when it is not in the namespace. */
export type BuiltinStorage =
  | {
      kind: "ElasticsearchCluster";
      name: string;
      object?: Resource<"ElasticsearchCluster">;
    }
  | {
      kind: "DatabaseServer";
      name: string;
      object?: Resource<"DatabaseServer">;
    };

// CONNECTING are the built-in options that connect contracts that exist
// already, in place of making resources.
const CONNECTING = [
  "existing-secondary-storage",
  "existing-object-storage",
  "existing-database-configs",
];

/**
 * requestDetail returns what the Overview of the request of component
 * shows, or undefined when the component has no request. claim is the claim
 * of the component, cluster the name of its cluster, and components every
 * section component of the claim. grants and contracts hold the objects of
 * the namespace of the claim.
 */
export function requestDetail(input: {
  component: SectionComponent;
  claim: Resource<"CamundaClusterClaim">;
  cluster: string;
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  components: readonly SectionComponent[];
  grants: readonly Resource<"Grant">[];
  contracts: Partial<Record<ContractKind, readonly ContractObject[]>>;
}): RequestDetail | undefined {
  const { component, claim, sections, registrations } = input;
  const request = component.request;
  if (!request) return undefined;
  const status = request.status;
  const section = sections.find((s) => s.metadata?.name === component.section);
  const slot = section?.spec.slot;
  const labelOf = (name: string) =>
    sections.find((s) => s.metadata?.name === name)?.spec.label ?? name;

  const controlled = controlledBy(request, claim.metadata?.uid);
  const policy = requestPolicy(
    request,
    deletionPolicyOf(claim.spec.deletionPolicy, claim.status?.resolvedPolicy),
  );
  const connected = connectedRefs(component, claim, input.contracts);
  const registration = registrations.find(
    (r) => r.metadata?.name === request.spec.registration,
  );
  const outputs = status?.outputs;

  return {
    summary: itemsOf(
      status?.summary,
      isSummaryItem,
      "Summary item",
      "summary",
    ).map((item) =>
      isInvalid(item)
        ? item
        : {
            label: item.label,
            value: item.value,
            severity: item.severity ?? "Info",
          },
    ),
    ...(typeof status?.summaryTime === "string" && {
      summaryTime: status.summaryTime,
    }),
    outputs: outputRows(outputs, slot),
    inputs: (registration?.spec.inputs ?? []).map((i) => {
      const from = splitFrom(i.from);
      const value = isRecord(request.spec.inputs)
        ? request.spec.inputs[i.name]
        : undefined;
      return {
        name: i.name,
        section: from.section,
        sectionLabel: labelOf(from.section),
        output: from.output,
        ...(typeof value === "string" && { value }),
      };
    }),
    links: itemsOf(status?.links, isLink, "Link", "links").map((item) =>
      isInvalid(item) ? item : { name: item.name, url: item.url },
    ),
    resources: connected
      ? []
      : itemsOf(status?.resources, isResource, "Resource", "resources").map(
          (row) =>
            isInvalid(row) || !controlled
              ? row
              : { ...row, fate: resourceFate(row, policy) },
        ),
    controlled,
    policy,
    grants: grantRows(
      input.grants,
      request,
      slot,
      input.cluster,
      registrations,
    ),
    clusterGrants: slot === "workloadIdentity",
    ...(connected && { connected }),
    ...(slot === "connectorsIdentity" && {
      connectors: {
        ...(isRecord(outputs) &&
          typeof outputs.principal === "string" && {
            principal: outputs.principal,
          }),
        serviceAccount: targetServiceAccount(
          "CamundaClusterClaim",
          claim.metadata?.name ?? "",
          // The cache object differs from the generated type only in its
          // timestamps, which targetServiceAccount does not read.
          section as unknown as PluginSection | undefined,
        ),
      },
    }),
    usedBy: usedBy(component, input.components, registrations),
  };
}

/**
 * builtinStorageOf returns the ElasticsearchCluster or DatabaseServer that
 * the first such row of status.resources of request names, looked up in
 * the namespace of request. It returns undefined for a request without
 * such a row.
 */
export function builtinStorageOf(
  request: Resource<"PluginRequest">,
  elasticsearchClusters: readonly Resource<"ElasticsearchCluster">[],
  databaseServers: readonly Resource<"DatabaseServer">[],
): BuiltinStorage | undefined {
  const rows = itemsOf(
    request.status?.resources,
    isResource,
    "Resource",
    "resources",
  );
  const row = rows.find(
    (r): r is ResourceRow =>
      !("error" in r) &&
      (r.kind === "ElasticsearchCluster" || r.kind === "DatabaseServer"),
  );
  if (!row) return undefined;
  const namespace = request.metadata?.namespace;
  const named = <
    O extends { metadata?: { namespace?: string; name?: string } },
  >(
    objects: readonly O[],
  ) =>
    objects.find(
      (o) =>
        o.metadata?.namespace === namespace && o.metadata?.name === row.name,
    );
  if (row.kind === "ElasticsearchCluster") {
    const object = named(elasticsearchClusters);
    return { kind: row.kind, name: row.name, ...(object && { object }) };
  }
  const object = named(databaseServers);
  return { kind: "DatabaseServer", name: row.name, ...(object && { object }) };
}

/** VolumeRow is one volume of built-in storage, with its capacity as text. */
export type VolumeRow = { name: string; capacity: string };

/**
 * StorageFacts is what the status of built-in storage holds: a field that
 * the status does not hold is absent. The snapshot repository is set only
 * for an ElasticsearchCluster, and the archive and recovery only for a
 * DatabaseServer. errors name each value of the wrong type, and that value
 * is absent.
 */
export type StorageFacts = {
  version?: string;
  snapshotRepository?: string;
  archive?: { reachableFrom?: string; boundary?: string };
  recovery?: { requestID?: string; targetTime?: string; startedAt?: string };
  volumes: (VolumeRow | Invalid)[];
  conditions: ConditionRow[];
  errors: string[];
};

/** storageFacts returns the facts of the status of the object of storage, which must be set. */
export function storageFacts(storage: BuiltinStorage): StorageFacts {
  const status: unknown = storage.object?.status;
  const read = isRecord(status) ? status : {};
  const errors: string[] = [];
  const text = (
    record: Record<string, unknown>,
    key: string,
    noun: string,
  ): string | undefined => {
    const value = record[key];
    if (value === undefined) return undefined;
    if (typeof value === "string") return value;
    errors.push(`The ${noun} is not a text.`);
    return undefined;
  };
  const version = text(read, "version", "version");
  const facts: StorageFacts = {
    ...(version !== undefined && { version }),
    volumes: [],
    conditions: [],
    errors,
  };
  if (storage.kind === "ElasticsearchCluster") {
    const repository = text(read, "snapshotRepository", "snapshot repository");
    if (repository !== undefined) facts.snapshotRepository = repository;
  } else {
    if (isRecord(read.archive)) {
      const reachableFrom = text(
        read.archive,
        "reachableFrom",
        "archive start",
      );
      const boundary = read.archive.boundary;
      facts.archive = {
        ...(reachableFrom !== undefined && { reachableFrom }),
        ...(isRecord(boundary) &&
          typeof boundary.at === "string" &&
          typeof boundary.location === "string" && {
            boundary: `${boundary.at}, ${boundary.location}`,
          }),
      };
    }
    if (isRecord(read.recovery)) {
      const requestID = text(read.recovery, "requestID", "recovery request");
      const targetTime = text(read.recovery, "targetTime", "recovery target");
      const startedAt = text(read.recovery, "startedAt", "recovery start");
      facts.recovery = {
        ...(requestID !== undefined && { requestID }),
        ...(targetTime !== undefined && { targetTime }),
        ...(startedAt !== undefined && { startedAt }),
      };
    }
  }
  facts.volumes = itemsOf(read.volumes, isVolume, "Volume", "volumes").map(
    (item) =>
      isInvalid(item)
        ? item
        : { name: item.name, capacity: String(item.capacity) },
  );
  const conditions = conditionsOf(read.conditions);
  if ("error" in conditions) errors.push(conditions.error);
  else facts.conditions = conditions.conditions;
  return facts;
}

function isVolume(
  item: unknown,
): item is { name: string; capacity: string | number } {
  return (
    isRecord(item) &&
    isText(item.name) &&
    (typeof item.capacity === "string" || typeof item.capacity === "number")
  );
}

/** readyOf returns the valid Ready condition of a status, if it has one. */
export function readyOf(conditions: unknown): ConditionRow | undefined {
  const read = conditionsOf(conditions);
  return "conditions" in read
    ? read.conditions.find((c) => c.type === "Ready")
    : undefined;
}

const SEVERITIES: readonly unknown[] = ["Info", "Warning", "Error", null];

function isSummaryItem(item: unknown): item is {
  label: string;
  value: string;
  severity?: SummaryRow["severity"] | null;
} {
  return (
    isRecord(item) &&
    isText(item.label) &&
    isText(item.value) &&
    (item.severity === undefined || SEVERITIES.includes(item.severity))
  );
}

function isLink(item: unknown): item is LinkRow {
  return isRecord(item) && isText(item.name) && isText(item.url);
}

function isResource(item: unknown): item is ResourceRow {
  return (
    isRecord(item) &&
    isText(item.kind) &&
    isText(item.name) &&
    (item.onOrphan === "Kept" || item.onOrphan === "Deleted") &&
    isText(item.id, true) &&
    isText(item.location, true) &&
    isText(item.role, true)
  );
}

function outputRows(
  outputs: unknown,
  slot: string | undefined,
): (OutputRow | Invalid)[] {
  if (outputs === undefined || outputs === null) return [];
  if (!isRecord(outputs)) return [{ error: "The outputs are not an object." }];
  return Object.keys(outputs)
    .sort(byCodeUnit)
    .map((key) => {
      const value = outputs[key];
      if (typeof value !== "string")
        return { error: `The output ${key} is not a text.` };
      return {
        key,
        value,
        fills: SLOT_FIELDS.filter(
          (f) =>
            f.claimKind === "CamundaClusterClaim" &&
            f.slot === slot &&
            f.output === key,
        ).map((f) => `${f.kind} ${f.path}`),
      };
    });
}

function grantRows(
  grants: readonly Resource<"Grant">[],
  request: Resource<"PluginRequest">,
  slot: string | undefined,
  cluster: string,
  registrations: readonly Resource<"PluginRegistration">[],
): GrantRow[] {
  const ofCluster = slot === "workloadIdentity";
  const uid = request.metadata?.uid;
  return grants
    .filter((g) =>
      ofCluster
        ? isRecord(g.spec?.cluster) && g.spec.cluster.name === cluster
        : uid !== undefined &&
          (g.metadata?.ownerReferences ?? []).some((o) => o.uid === uid),
    )
    .map((g) => grantRow(g, ofCluster, registrations))
    .sort((a, b) => byCodeUnit(a.name, b.name));
}

function grantRow(
  grant: Resource<"Grant">,
  withRequest: boolean,
  registrations: readonly Resource<"PluginRegistration">[],
): GrantRow {
  // The cache objects differ from the generated types only in their
  // timestamps, which grantView and grantTemplates do not read.
  const generated = grant as unknown as Grant;
  const { error, request, ...view } = grantView(generated);
  const fragment: unknown = grant.spec?.fragment;
  const template: unknown = grant.spec?.template;
  const read = conditionsOf(grant.status?.conditions);
  const registrationName = grant.metadata?.labels?.[REGISTRATION_LABEL];
  const registration =
    registrationName === undefined
      ? undefined
      : registrations.find((r) => r.metadata?.name === registrationName);
  return {
    ...view,
    ...(withRequest && request && { request }),
    template: typeof template === "string" ? template : "",
    ...(isRecord(fragment) && {
      statement: JSON.stringify(fragment, null, 2),
    }),
    conditions: "conditions" in read ? read.conditions : [],
    templateGone:
      registration !== undefined &&
      grantTemplates(registration as unknown as PluginRegistration, [generated])
        .withoutTemplate.length > 0,
    errors: [
      ...(isRecord(fragment) ? [] : ["The statement is not a JSON object."]),
      ...(error ? [error] : []),
    ],
  };
}

function connectedRefs(
  component: SectionComponent,
  claim: Resource<"CamundaClusterClaim">,
  contracts: Partial<Record<ContractKind, readonly ContractObject[]>>,
): ConnectedRef[] | undefined {
  const { registration, parameters } = component.selection;
  if (!CONNECTING.includes(registration)) return undefined;
  const lists = own(BUILTIN_OPTIONS, registration)?.lists ?? {};
  const namespace = claim.metadata?.namespace;
  return Object.entries(lists).flatMap(([parameter, kind]) => {
    const name = isRecord(parameters) ? parameters[parameter] : undefined;
    if (typeof name !== "string") return [];
    const contractKind = kind as ContractKind;
    const object = (contracts[contractKind] ?? []).find(
      (o) => o.metadata?.namespace === namespace && o.metadata?.name === name,
    );
    const ready = object && readyOf(object.status?.conditions);
    return [
      {
        kind: contractKind,
        name,
        found: object !== undefined,
        ...(ready && { ready }),
      },
    ];
  });
}

function usedBy(
  component: SectionComponent,
  components: readonly SectionComponent[],
  registrations: readonly Resource<"PluginRegistration">[],
): string[] {
  const labels = components
    .filter((c) => c.id !== component.id)
    .filter((c) =>
      registrations
        .find((r) => r.metadata?.name === c.selection.registration)
        ?.spec.inputs?.some(
          (i) => splitFrom(i.from).section === component.section,
        ),
    )
    .map((c) => c.label);
  return [...new Set(labels)];
}
