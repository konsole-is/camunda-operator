import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { Resource } from "@/lib/k8s/kinds";
import { isManaged } from "@/lib/labels";
import {
  APP_FIELD_MANAGER,
  child,
  fieldManagers,
  isClaimManager,
  indexItem,
  keyedItem,
  mergeCluster,
  type FieldPath,
  type Layer,
  type ManagedObject,
  type Merged,
  wholeFieldManagers,
} from "@/lib/merge";
import { LOCK_FIELDS, type LockKind, type LockRow } from "./locks";
import { parameterFields } from "./parameters";
import { SLOT_FIELDS } from "./slots";
import type { ClusterField, FieldGroup, FieldLock, FieldState } from "./types";
import { pathFrom, walk, type Step, type WalkNode } from "./walk";

/** ResolvedPolicy is the policy that the claim controller resolved for a claim. */
export type ResolvedPolicy = NonNullable<
  NonNullable<CamundaClusterClaim["status"]>["resolvedPolicy"]
>;

/**
 * ClusterFieldsInput is what clusterFields reads. cluster is the live object
 * with metadata.managedFields. merged is mergeCluster of its preset, its
 * release, and cluster.spec. policy is the resolved policy of the claim,
 * which a create flow can pass before the claim has a status. sections give
 * the labels and slots of the selections.
 */
export type ClusterFieldsInput = {
  cluster: Resource<"CamundaCluster">;
  merged: Merged<CamundaClusterSpec>;
  claim?: Resource<"CamundaClusterClaim">;
  policy?: ResolvedPolicy;
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  optimize?: Resource<"CamundaOptimize">;
  backupSchedule?: Resource<"BackupSchedule">;
};

/**
 * clusterFields returns every field of a cluster: the CamundaCluster first,
 * then Optimize, the backup schedule, and the section parameters. A field
 * of a CRD target gets the first state that holds, on a cluster with the
 * claim label:
 *
 *  1. "Filled by <section>": a slot of a selected section fills it. It shows
 *     also before it has a value.
 *  2. "Set by <manager>": a manager other than the claim and the app owns it.
 *  3. "Changed in the claim": the claim sets it.
 *  4. "From release".
 *  5. "From preset".
 *  6. "Changed in the claim": the claim controller owns it.
 *  7. "Default value": no manager owns it.
 *  8. "Set by" with no known writer.
 *
 * Its lock is the section of a filled field, else the lock of its path:
 * set by the release, fixed by the claim controller, or set by a section.
 * On a cluster without the claim label, a field is From release, From
 * preset, Set by its manager, or a Default value when no manager owns it,
 * and every field is locked as not managed.
 * A cluster with the label and no claim object is still managed.
 */
export function clusterFields(input: ClusterFieldsInput): ClusterField[] {
  const managed = isManaged(input.cluster);
  const selections = managed ? (input.claim?.spec.plugins ?? []) : [];
  const ctx: Context = {
    managed,
    filled: (kind) => filledPaths(kind, selections, input.sections),
  };
  const attachments = input.claim?.spec.attachments;
  return [
    ...camundaClusterFields(input, ctx),
    ...attachmentFields(ctx, {
      kind: "CamundaOptimize",
      group: "optimize",
      live: input.optimize,
      claimed: renameAuth(attachments?.optimize),
      claimRoot: child(child("spec", "attachments"), "optimize"),
    }),
    ...attachmentFields(ctx, {
      kind: "BackupSchedule",
      group: "backupSchedule",
      live: input.backupSchedule,
      claimed: attachments?.backupSchedule,
      claimRoot: child(child("spec", "attachments"), "backupSchedule"),
    }),
    ...parameterFields(input),
  ];
}

type Selected = {
  section: string;
  registration: string;
  sectionLabel: string;
  names: readonly string[];
};

type Context = {
  managed: boolean;
  filled: (kind: LockKind) => Map<FieldPath, Selected>;
};

// The fields of the claim that a claim sets at its own top level, not in
// spec.cluster.
const CLAIM_FIELDS = new Set([
  "presetRef",
  "releaseRef",
  "platformConfigRef",
  "suspend",
]);

const COMPONENTS = new Set<FieldGroup>([
  "zeebe",
  "gateway",
  "operate",
  "tasklist",
  "admin",
  "connectors",
]);

const RANK: Record<Layer, number> = { preset: 0, release: 1, overrides: 2 };

function camundaClusterFields(
  input: ClusterFieldsInput,
  ctx: Context,
): ClusterField[] {
  const { source } = input.merged;
  const nodes = walk(input.merged.spec, "spec");
  const filled = ctx.filled("CamundaCluster");
  const locks = lockPaths("CamundaCluster");

  // A slot or lock path is one field, also inside a block that one layer
  // sets as a whole, so its ancestors split into their fields.
  const special = new Set<FieldPath>(locks.keys());
  const specialAncestors = new Set<FieldPath>();
  for (const names of [
    ...SLOT_FIELDS.filter(isClusterSlot).map((row) => row.names),
    ...LOCK_FIELDS.filter((row) => row.kind === "CamundaCluster").map(
      (row) => row.names,
    ),
  ]) {
    special.add(pathOf(names));
    for (let i = 1; i < names.length; i++)
      specialAncestors.add(pathOf(names.slice(0, i)));
  }

  const ancestors = (path: FieldPath): FieldPath[] => {
    const chain: FieldPath[] = [];
    for (let at = nodes.get(path)?.parent; at !== undefined;) {
      chain.push(at);
      at = nodes.get(at)?.parent;
    }
    return chain;
  };
  const hasSource = (path: FieldPath) => Object.hasOwn(source, path);
  const isField = (node: WalkNode): boolean => {
    if (node.parent === undefined) return false;
    const chain = ancestors(node.path);
    if (chain.some((a) => special.has(a))) return false;
    if (special.has(node.path)) return true;
    if (hasSource(node.path))
      return !specialAncestors.has(node.path) || node.leaf;
    if (!node.leaf) return false;
    const owner = chain.find(hasSource);
    return owner !== undefined && specialAncestors.has(owner);
  };

  const units: WalkNode[] = [...nodes.values()].filter(isField);
  for (const [path, selected] of filled) {
    if (!nodes.has(path))
      units.push({
        path,
        steps: stepsOf(selected.names),
        value: undefined,
        leaf: true,
      });
  }
  const fields = new Set(units.map((u) => u.path));
  const fieldOf = (path: FieldPath): FieldPath | undefined => {
    for (let at: FieldPath | undefined = path; at !== undefined;) {
      if (fields.has(at)) return at;
      at = nodes.get(at)?.parent;
    }
    return undefined;
  };

  const below = new Map<FieldPath, Layer>();
  for (const [path, layer] of Object.entries(source)) {
    const field = nodes.has(path) ? fieldOf(path) : undefined;
    const current = field === undefined ? undefined : below.get(field);
    if (
      field !== undefined &&
      (current === undefined || RANK[layer] > RANK[current])
    )
      below.set(field, layer);
  }
  const layerOf = (path: FieldPath): Layer | undefined => {
    if (hasSource(path)) return source[path];
    const owner = ancestors(path).find(hasSource);
    return owner === undefined ? below.get(path) : source[owner];
  };
  // An inherited item of a merged list is not in the live object, so a
  // manager of the whole live list does not own it.
  const managers = managersByField(
    input.cluster,
    nodes,
    fieldOf,
    units.map((u) => u.path),
    (field) => layerOf(field) === "overrides",
  );

  const claim = input.claim;
  const claimNodes = walk(
    ctx.managed && claim
      ? mergeCluster(
          undefined,
          undefined,
          (claim.spec.cluster ?? {}) as CamundaClusterSpec,
        ).spec
      : {},
    "spec",
  );
  const topLevel = (steps: Step[]): string | undefined => {
    const [first] = steps;
    return steps.length === 1 &&
      "field" in first &&
      CLAIM_FIELDS.has(first.field)
      ? first.field
      : undefined;
  };
  const claimSets = (unit: WalkNode): boolean => {
    const name = topLevel(unit.steps);
    if (name !== undefined)
      return (
        (claim?.spec as Record<string, unknown> | undefined)?.[name] !==
        undefined
      );
    const local = claimSteps(unit.steps);
    return !local.inherited && claimNodes.has(pathFrom("spec", local.steps));
  };
  // claimSteps gives the steps of a field in spec.cluster of the claim. A
  // concatenated list holds the items of the preset and the release before
  // those of the cluster, so an index counts only the cluster items. An item
  // that a baseline adds is inherited, and its steps end at its list.
  const claimSteps = (steps: Step[]): { steps: Step[]; inherited: boolean } => {
    const local: Step[] = [];
    let path: FieldPath = "spec";
    for (const step of steps) {
      let next = step;
      if ("index" in step) {
        const layer = source[indexItem(path, step.index)];
        if (layer !== undefined && layer !== "overrides")
          return { steps: local, inherited: true };
        if (layer === "overrides") {
          let index = 0;
          for (let i = 0; i < step.index; i++)
            if (source[indexItem(path, i)] === "overrides") index++;
          next = { index };
        }
      }
      local.push(next);
      path = pathFrom(path, [step]);
    }
    return { steps: local, inherited: false };
  };

  return units.map((unit) => {
    const own = managers.get(unit.path) ?? [];
    const lockReason = locks.get(unit.path);
    const selected = filled.get(unit.path);
    const layer = layerOf(unit.path);
    const state: FieldState = ctx.managed
      ? managedState({
          selected,
          managers: own,
          claimSets: claimSets(unit),
          release: layer === "release" || lockReason === "release",
          preset: layer === "preset",
          locked: lockReason !== undefined,
        })
      : layer === "release"
        ? { kind: "release" }
        : layer === "preset"
          ? { kind: "preset" }
          : unmanagedState(own);
    const lock = lockOf(ctx.managed, selected, lockReason);
    const name = topLevel(unit.steps);
    const [first] = unit.steps;
    const component =
      unit.steps.length > 1 &&
      "field" in first &&
      COMPONENTS.has(first.field as FieldGroup)
        ? (first.field as FieldGroup)
        : undefined;
    return {
      key: `CamundaCluster:${unit.path}`,
      target: { kind: "CamundaCluster" },
      group: component ?? "cluster",
      path: unit.path,
      label: pathFrom("", component ? unit.steps.slice(1) : unit.steps),
      value: unit.value,
      state,
      ...(lock && { lock }),
      ...claimPathOf(ctx.managed, selected, lock, () =>
        name !== undefined
          ? child("spec", name)
          : pathFrom(child("spec", "cluster"), claimSteps(unit.steps).steps),
      ),
      docNames: docNamesOf(unit.steps),
    };
  });
}

type Attachment = {
  kind: "CamundaOptimize" | "BackupSchedule";
  group: FieldGroup;
  live?: { spec: object; metadata?: ManagedObject["metadata"] };
  claimed?: object;
  claimRoot: FieldPath;
};

// attachmentFields gives each leaf and each list item of the live spec of an
// attachment its state. An attachment has no preset or release layer.
function attachmentFields(ctx: Context, a: Attachment): ClusterField[] {
  if (a.live === undefined) return [];
  const nodes = walk(a.live.spec, "spec");
  const filled = ctx.filled(a.kind);
  const locks = lockPaths(a.kind);

  const units: WalkNode[] = [];
  const fields = new Set<FieldPath>();
  const fieldOf = (path: FieldPath): FieldPath | undefined => {
    for (let at: FieldPath | undefined = path; at !== undefined;) {
      if (fields.has(at)) return at;
      at = nodes.get(at)?.parent;
    }
    return undefined;
  };
  for (const node of nodes.values()) {
    if (node.parent === undefined) continue;
    const last = node.steps[node.steps.length - 1];
    if (!node.leaf && "field" in last) continue;
    if (fieldOf(node.parent) !== undefined) continue;
    units.push(node);
    fields.add(node.path);
  }
  for (const [path, selected] of filled) {
    if (!nodes.has(path)) {
      units.push({
        path,
        steps: stepsOf(selected.names),
        value: undefined,
        leaf: true,
      });
      fields.add(path);
    }
  }

  const managers = managersByField(
    a.live,
    nodes,
    fieldOf,
    units.map((u) => u.path),
    () => true,
  );
  const claimNodes = walk(ctx.managed ? (a.claimed ?? {}) : {}, "spec");

  return units.map((unit) => {
    const own = managers.get(unit.path) ?? [];
    const lockReason = locks.get(unit.path);
    const selected = filled.get(unit.path);
    const state: FieldState = ctx.managed
      ? managedState({
          selected,
          managers: own,
          claimSets: claimNodes.has(unit.path),
          release: lockReason === "release",
          preset: false,
          locked: lockReason !== undefined,
        })
      : unmanagedState(own);
    const lock = lockOf(ctx.managed, selected, lockReason);
    return {
      key: `${a.kind}:${unit.path}`,
      target: { kind: a.kind },
      group: a.group,
      path: unit.path,
      label: pathFrom("", unit.steps),
      value: unit.value,
      state,
      ...(lock && { lock }),
      ...claimPathOf(ctx.managed, selected, lock, () =>
        pathFrom(a.claimRoot, claimAuthSteps(unit.steps)),
      ),
      docNames: docNamesOf(unit.steps),
    };
  });
}

function managedState(f: {
  selected?: Selected;
  managers: string[];
  claimSets: boolean;
  release: boolean;
  preset: boolean;
  locked: boolean;
}): FieldState {
  if (f.selected)
    return {
      kind: "filled",
      section: f.selected.section,
      sectionLabel: f.selected.sectionLabel,
    };
  const [foreign] = f.managers
    .filter((m) => !isClaimManager(m) && m !== APP_FIELD_MANAGER)
    .sort();
  if (foreign !== undefined)
    return { kind: "setBy", manager: foreign, takeOver: !f.locked };
  if (f.claimSets) return { kind: "claim" };
  if (f.release) return { kind: "release" };
  if (f.preset) return { kind: "preset" };
  if (f.managers.some(isClaimManager)) return { kind: "claim" };
  if (f.managers.length === 0) return { kind: "default" };
  return { kind: "setBy", takeOver: !f.locked };
}

function unmanagedState(managers: string[]): FieldState {
  const [manager] = [...managers].sort();
  if (manager === undefined) return { kind: "default" };
  return { kind: "setBy", manager, takeOver: false };
}

function lockOf(
  managed: boolean,
  selected: Selected | undefined,
  reason: LockRow["reason"] | undefined,
): FieldLock | undefined {
  if (!managed) return { reason: "notManaged" };
  if (selected)
    return {
      reason: "filled",
      section: selected.section,
      sectionLabel: selected.sectionLabel,
    };
  return reason === undefined ? undefined : { reason };
}

// claimPathOf gives a filled field the path of its selection, and any other
// field the path that path() builds unless it is locked. An unmanaged
// cluster has no claim path.
function claimPathOf(
  managed: boolean,
  selected: Selected | undefined,
  lock: FieldLock | undefined,
  path: () => FieldPath,
): { claimPath?: FieldPath } {
  if (!managed) return {};
  if (selected)
    return {
      claimPath: keyedItem(child("spec", "plugins"), {
        section: selected.section,
        registration: selected.registration,
      }),
    };
  return lock === undefined ? { claimPath: path() } : {};
}

// managersByField gives each field the managers that own it or a value
// inside it. A manager of a whole node above the fields owns each field below
// it that inherits returns true for.
function managersByField(
  live: ManagedObject,
  nodes: Map<FieldPath, WalkNode>,
  fieldOf: (path: FieldPath) => FieldPath | undefined,
  fields: readonly FieldPath[],
  inherits: (field: FieldPath) => boolean,
): Map<FieldPath, string[]> {
  const byField = new Map<FieldPath, string[]>();
  const add = (field: FieldPath, manager: string) => {
    const list = byField.get(field) ?? [];
    if (!list.includes(manager)) list.push(manager);
    byField.set(field, list);
  };
  const isBelow = (field: FieldPath, path: FieldPath): boolean => {
    for (let at = nodes.get(field)?.parent; at !== undefined;) {
      if (at === path) return true;
      at = nodes.get(at)?.parent;
    }
    return false;
  };
  for (const [path, manager] of Object.entries(fieldManagers(live))) {
    const field = nodes.has(path) ? fieldOf(path) : undefined;
    if (field !== undefined) add(field, manager);
  }
  for (const [path, manager] of Object.entries(wholeFieldManagers(live))) {
    if (!nodes.has(path) || fieldOf(path) !== undefined) continue;
    for (const f of fields)
      if (isBelow(f, path) && inherits(f)) add(f, manager);
  }
  return byField;
}

function filledPaths(
  kind: LockKind,
  selections: NonNullable<CamundaClusterClaim["spec"]["plugins"]>,
  sections: readonly Resource<"PluginSection">[],
): Map<FieldPath, Selected> {
  const filled = new Map<FieldPath, Selected>();
  for (const s of selections) {
    const section = sections.find((x) => x.metadata?.name === s.section);
    const slot = section?.spec.slot;
    if (section === undefined || slot === undefined) continue;
    for (const row of SLOT_FIELDS) {
      if (
        row.claimKind !== "CamundaClusterClaim" ||
        row.kind !== kind ||
        row.slot !== slot
      )
        continue;
      filled.set(row.path, {
        section: s.section,
        registration: s.registration,
        sectionLabel: section.spec.label,
        names: row.names,
      });
    }
  }
  return filled;
}

function lockPaths(kind: LockKind): Map<FieldPath, LockRow["reason"]> {
  return new Map(
    LOCK_FIELDS.filter((row) => row.kind === kind).map((row) => [
      pathOf(row.names),
      row.reason,
    ]),
  );
}

function isClusterSlot(row: (typeof SLOT_FIELDS)[number]): boolean {
  return (
    row.claimKind === "CamundaClusterClaim" && row.kind === "CamundaCluster"
  );
}

function pathOf(names: readonly string[]): FieldPath {
  return names.reduce(child, "");
}

function stepsOf(names: readonly string[]): Step[] {
  return names.slice(1).map((field) => ({ field }));
}

function docNamesOf(steps: readonly Step[]): string[] {
  return ["spec", ...steps.flatMap((s) => ("field" in s ? [s.field] : []))];
}

// The claim names the auth reference of Optimize managementAuthConfigRef,
// and CamundaOptimize names it managementAuthRef.
const CLAIM_AUTH = "managementAuthConfigRef";
const LIVE_AUTH = "managementAuthRef";

function renameAuth(optimize: object | undefined): object | undefined {
  if (optimize === undefined) return undefined;
  const { [CLAIM_AUTH]: auth, ...rest } = optimize as Record<string, unknown>;
  return auth === undefined ? rest : { ...rest, [LIVE_AUTH]: auth };
}

function claimAuthSteps(steps: readonly Step[]): Step[] {
  const [first, ...rest] = steps;
  return first !== undefined && "field" in first && first.field === LIVE_AUTH
    ? [{ field: CLAIM_AUTH }, ...rest]
    : [...steps];
}
