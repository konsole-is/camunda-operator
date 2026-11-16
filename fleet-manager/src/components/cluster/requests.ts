// This module runs in the browser and on the server.
import type { ConditionRow } from "@/components/fleet/conditions-table";
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
  isObject,
  own,
  splitFrom,
  targetServiceAccount,
} from "@/lib/policy";
import { REQUEST_LABEL } from "@/lib/labels";

/** Invalid stands for a row of a live object that has a value of the wrong type, with the reason. */
export type Invalid = { error: string };

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
 * GrantState is ready while Ready is True, oldTemplate while Ready is False
 * with Applied True and MatchesTemplate False, refused while Ready is
 * otherwise False, and waiting while Ready is neither, or its conditions
 * cannot be read.
 */
export type GrantState = "ready" | "oldTemplate" | "refused" | "waiting";

/**
 * GrantRow is one Grant. statement is spec.fragment as indented JSON.
 * request is the request that asked for it, set only in the grants of the
 * cloud identity. errors name each value of the wrong type, and a value
 * that has one is absent.
 */
export type GrantRow = {
  name: string;
  template: string;
  statement?: string;
  request?: string;
  conditions: ConditionRow[];
  ready?: ConditionRow;
  state: GrantState;
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
    summary: itemsOf(status?.summary, summaryRow, "Summary item", "summary"),
    ...(typeof status?.summaryTime === "string" && {
      summaryTime: status.summaryTime,
    }),
    outputs: outputRows(outputs, slot),
    inputs: (registration?.spec.inputs ?? []).map((i) => {
      const from = splitFrom(i.from);
      const value = isObject(request.spec.inputs)
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
    links: itemsOf(status?.links, linkRow, "Link", "links"),
    resources: connected
      ? []
      : itemsOf(status?.resources, resourceRow, "Resource", "resources").map(
          (row) =>
            "error" in row || !controlled
              ? row
              : { ...row, fate: resourceFate(row, policy) },
        ),
    controlled,
    policy,
    grants: grantRows(input.grants, request, slot, input.cluster),
    clusterGrants: slot === "workloadIdentity",
    ...(connected && { connected }),
    ...(slot === "connectorsIdentity" && {
      connectors: {
        ...(isObject(outputs) &&
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
    resourceRow,
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

const STATUSES: readonly unknown[] = ["True", "False", "Unknown"];

/**
 * conditionsOf returns the conditions of a status, or an error that names
 * the first item that is not a valid condition. No value gives no
 * conditions.
 */
export function conditionsOf(
  value: unknown,
): { conditions: ConditionRow[] } | { error: string } {
  if (value === undefined || value === null) return { conditions: [] };
  if (!Array.isArray(value)) return { error: "The conditions are not a list." };
  const bad = value.findIndex(
    (c) =>
      !isObject(c) ||
      typeof c.type !== "string" ||
      !STATUSES.includes(c.status) ||
      !optionalText(c.reason) ||
      !optionalText(c.message) ||
      !optionalText(c.lastTransitionTime),
  );
  if (bad >= 0)
    return { error: `Condition ${bad + 1} is not a valid condition.` };
  return { conditions: value as ConditionRow[] };
}

/** readyOf returns the valid Ready condition of a status, if it has one. */
export function readyOf(conditions: unknown): ConditionRow | undefined {
  const read = conditionsOf(conditions);
  return "conditions" in read
    ? read.conditions.find((c) => c.type === "Ready")
    : undefined;
}

function optionalText(value: unknown): boolean {
  return value === undefined || typeof value === "string";
}

// itemsOf checks each item of a status list before a block renders it, so
// that one item of the wrong type shows as an error of its row.
function itemsOf<T>(
  value: unknown,
  valid: (item: unknown) => T | undefined,
  noun: string,
  plural: string,
): (T | Invalid)[] {
  if (value === undefined || value === null) return [];
  if (!Array.isArray(value))
    return [{ error: `The ${plural} are not a list.` }];
  return value.map(
    (item, i) =>
      valid(item) ?? {
        error: `${noun} ${i + 1} is not a valid ${noun.toLowerCase()}.`,
      },
  );
}

function summaryRow(item: unknown): SummaryRow | undefined {
  if (
    !isObject(item) ||
    typeof item.label !== "string" ||
    typeof item.value !== "string"
  )
    return undefined;
  const severity = item.severity ?? "Info";
  if (severity !== "Info" && severity !== "Warning" && severity !== "Error")
    return undefined;
  return { label: item.label, value: item.value, severity };
}

function linkRow(item: unknown): LinkRow | undefined {
  return isObject(item) &&
    typeof item.name === "string" &&
    typeof item.url === "string"
    ? { name: item.name, url: item.url }
    : undefined;
}

function resourceRow(item: unknown): ResourceRow | undefined {
  if (
    !isObject(item) ||
    typeof item.kind !== "string" ||
    typeof item.name !== "string" ||
    (item.onOrphan !== "Kept" && item.onOrphan !== "Deleted") ||
    !optionalText(item.id) ||
    !optionalText(item.location) ||
    !optionalText(item.role)
  )
    return undefined;
  return item as ResourceRow;
}

function outputRows(
  outputs: unknown,
  slot: string | undefined,
): (OutputRow | Invalid)[] {
  if (outputs === undefined || outputs === null) return [];
  if (!isObject(outputs)) return [{ error: "The outputs are not an object." }];
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
): GrantRow[] {
  const ofCluster = slot === "workloadIdentity";
  const uid = request.metadata?.uid;
  return grants
    .filter((g) =>
      ofCluster
        ? isObject(g.spec?.cluster) && g.spec.cluster.name === cluster
        : uid !== undefined &&
          (g.metadata?.ownerReferences ?? []).some((o) => o.uid === uid),
    )
    .map((g) => grantRow(g, ofCluster))
    .sort((a, b) => byCodeUnit(a.name, b.name));
}

function grantRow(grant: Resource<"Grant">, withRequest: boolean): GrantRow {
  const errors: string[] = [];
  const fragment: unknown = grant.spec?.fragment;
  if (!isObject(fragment)) errors.push("The statement is not a JSON object.");
  const read = conditionsOf(grant.status?.conditions);
  if ("error" in read) errors.push(read.error);
  const conditions = "conditions" in read ? read.conditions : [];
  const find = (type: string) => conditions.find((c) => c.type === type);
  const ready = find("Ready");
  const request = grant.metadata?.labels?.[REQUEST_LABEL];
  const template: unknown = grant.spec?.template;
  return {
    name: grant.metadata?.name ?? "",
    template: typeof template === "string" ? template : "",
    ...(isObject(fragment) && {
      statement: JSON.stringify(fragment, null, 2),
    }),
    ...(withRequest && request && { request }),
    conditions,
    ...(ready && { ready }),
    state:
      ready?.status === "True"
        ? "ready"
        : ready?.status !== "False"
          ? "waiting"
          : find("Applied")?.status === "True" &&
              find("MatchesTemplate")?.status === "False"
            ? "oldTemplate"
            : "refused",
    errors,
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
    const name = isObject(parameters) ? parameters[parameter] : undefined;
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
