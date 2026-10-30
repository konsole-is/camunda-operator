import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import { itemStep, walk, type Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { mergeCluster, type Merged } from "@/lib/merge";
import { parseQuantity } from "@/lib/observe/quantity";
import { equalJSON } from "@/lib/policy";
import {
  claimAfter,
  without,
  valueAt,
  withValue,
  type Session,
} from "@/lib/session";
import { topologyOf, type Process } from "./topology";

/**
 * ClusterChangeInput is a session over the claim of a cluster, with the
 * live cluster and the presets, releases, and sections that the change can
 * name. Without environment, a change that removes the release of the
 * claim has no release after it.
 */
export type ClusterChangeInput = {
  session: Session<Resource<"CamundaClusterClaim">>;
  live: Resource<"CamundaCluster">;
  presets: readonly Resource<"CamundaClusterPreset">[];
  releases: readonly Resource<"CamundaRelease">[];
  sections: readonly Resource<"PluginSection">[];
  environment?: Resource<"Environment">;
};

const PROCESS_NAME: Record<Process, string> = {
  zeebe: "Broker",
  gateway: "Gateway",
  operate: "Operate",
  tasklist: "Tasklist",
  admin: "Admin",
  connectors: "Connectors",
};

const CLUSTER_WIDE = new Set([
  "extraEnv",
  "extraEnvFrom",
  "scheduling",
  "podLabels",
  "podAnnotations",
]);

// The operator applies these fields of an embedded process to its host.
const HOST_FIELDS = new Set(["extraEnv", "extraEnvFrom"]);

/**
 * mergedSpecs returns the merged spec of the cluster before the change and
 * after it. after is the cluster that the claim controller makes from the
 * claim after the change. It takes the fields that the controller sets from
 * the live cluster.
 */
export function mergedSpecs(input: ClusterChangeInput): {
  before: Merged<CamundaClusterSpec>;
  after: Merged<CamundaClusterSpec>;
} {
  const { live } = input;
  const after = clusterAfter(input);
  return {
    before: merged(input, live.spec.presetRef, live.spec.releaseRef, live.spec),
    after: merged(input, after.presetRef, after.releaseRef, after),
  };
}

/**
 * clusterImpacts returns the impact lines of a change, in the order of the
 * impact table of the review, each once.
 */
export function clusterImpacts(input: ClusterChangeInput): string[] {
  const { session, live } = input;
  const { before, after } = mergedSpecs(input);
  const was = session.claim.spec;
  const is = claimAfter(session).spec;
  const changed = (name: keyof typeof was) => !equalJSON(was[name], is[name]);
  const cluster = clusterChanges(input);
  const lines = new Set<string>();

  if (changed("releaseRef")) {
    const version = (name: string | undefined) =>
      input.releases.find((r) => r.metadata?.name === name)?.spec.version;
    if (version(live.spec.releaseRef) !== version(releaseAfter(input)))
      lines.add(
        "The brokers restart one at a time. The other processes restart.",
      );
  }

  if (changed("presetRef"))
    for (const line of leafChanges(
      { ...before.spec, presetRef: undefined },
      { ...after.spec, presetRef: undefined },
    ))
      lines.add(line);

  const boxes = topologyOf(after.spec, []);
  const shrinks = shrinksStorage(before.spec, after.spec);
  for (const steps of cluster) {
    const [first, second] = steps;
    if (!("field" in first) || !Object.hasOwn(PROCESS_NAME, first.field))
      continue;
    const process = first.field as Process;
    const field = second && "field" in second ? second.field : undefined;
    if (field === "mode") continue;
    if (shrinks && process === "zeebe" && field === "storageSize") continue;
    const host = boxes.find((b) =>
      b.embedded.some((e) => e.process === process),
    );
    if (boxes.some((b) => b.process === process))
      lines.add(`${PROCESS_NAME[process]} pods restart.`);
    else if (host && field !== undefined && HOST_FIELDS.has(field))
      lines.add(`${PROCESS_NAME[host.process]} pods restart.`);
    else lines.add(`Applies when ${PROCESS_NAME[process]} runs standalone.`);
  }

  if (
    cluster.some(([first]) => "field" in first && CLUSTER_WIDE.has(first.field))
  )
    lines.add(
      "Every process restarts, except the processes that set the field themselves.",
    );

  for (const line of modeLines(input, before.spec, after.spec)) lines.add(line);

  if (shrinks)
    lines.add("Volumes cannot shrink. The operator keeps the current size.");

  for (const steps of diff(was.plugins ?? [], is.plugins ?? [])) {
    const [selection, parameters] = steps;
    if (
      !selection ||
      !("item" in selection) ||
      !parameters ||
      !("field" in parameters) ||
      parameters.field !== "parameters"
    )
      continue;
    const section = String(selection.item.section);
    const label =
      input.sections.find((s) => s.metadata?.name === section)?.spec.label ??
      section;
    lines.add(`The ${label} option applies the new value.`);
  }

  if (changed("deletionPolicy"))
    lines.add(
      "Nothing restarts. The policy applies when the cluster is deleted.",
    );

  if (is.suspend === true && was.suspend !== true)
    lines.add("Every workload goes to zero. The volumes stay.");

  if (
    !equalJSON(was.attachments?.backupSchedule, is.attachments?.backupSchedule)
  )
    lines.add("Nothing restarts.");

  return [...lines];
}

/** showsMergedDiff says whether the review shows the merged diff: a change of presetRef or releaseRef, or a reset. */
export function showsMergedDiff(s: Session): boolean {
  const ref = (claim: unknown, name: string) =>
    valueAt(claim, [{ field: "spec" }, { field: name }]);
  const after = claimAfter(s);
  return (
    [...s.pending.values()].some((e) => "reset" in e) ||
    ref(s.claim, "presetRef") !== ref(after, "presetRef") ||
    ref(s.claim, "releaseRef") !== ref(after, "releaseRef")
  );
}

function merged(
  input: ClusterChangeInput,
  presetRef: string | undefined,
  releaseRef: string | undefined,
  overrides: CamundaClusterSpec,
): Merged<CamundaClusterSpec> {
  return mergeCluster(
    input.presets.find((p) => p.metadata?.name === presetRef)?.spec,
    input.releases.find((r) => r.metadata?.name === releaseRef)?.spec,
    overrides,
  );
}

function releaseAfter(input: ClusterChangeInput): string | undefined {
  const was = input.session.claim.spec.releaseRef;
  const is = claimAfter(input.session).spec.releaseRef;
  if (was === is) return input.live.spec.releaseRef;
  return is || input.environment?.spec.releases?.default;
}

// CONTROLLER_FIELDS are the fields of a CamundaCluster that the claim
// controller fills from outside the claim: the ServiceAccount names, the
// platform config, and the slot fields (cluster in
// pkg/components/camundaclusterclaim/cluster.go).
const CONTROLLER_FIELDS: Step[][] = [
  ["serviceAccount", "name"],
  ["serviceAccount", "annotations"],
  ["connectors", "serviceAccount", "name"],
  ["connectors", "serviceAccount", "annotations"],
  ["zeebe", "storageClassName"],
  ["externalUrl"],
  ["storageRef"],
  ["backupStorageRef"],
  ["platformConfigRef"],
].map((names) => names.map((field) => ({ field })));

function clusterAfter(input: ClusterChangeInput): CamundaClusterSpec {
  const claim = claimAfter(input.session).spec;
  let spec: CamundaClusterSpec = structuredClone(claim.cluster ?? {});
  const fields: [Step[], unknown][] = [
    [[{ field: "presetRef" }], claim.presetRef],
    [[{ field: "releaseRef" }], releaseAfter(input)],
    [[{ field: "suspend" }], claim.suspend],
    ...CONTROLLER_FIELDS.map((steps): [Step[], unknown] => [
      steps,
      valueAt(input.live.spec, steps),
    ]),
  ];
  for (const [steps, value] of fields)
    spec =
      value === undefined
        ? without(spec, steps)
        : withValue(spec, steps, value);
  return spec;
}

// clusterChanges returns the steps below spec.cluster of each field that
// the change sets or removes.
function clusterChanges(input: ClusterChangeInput): Step[][] {
  return diff(
    input.session.claim.spec.cluster ?? {},
    claimAfter(input.session).spec.cluster ?? {},
  );
}

function diff(was: unknown, is: unknown, steps: Step[] = []): Step[][] {
  if (equalJSON(was, is)) return [];
  const empty = (other: unknown) => (Array.isArray(other) ? [] : {});
  const children = childrenOf(was ?? empty(is), is ?? empty(was));
  if (!children || children.length === 0) return [steps];
  return children.flatMap(([step, w, i]) => diff(w, i, [...steps, step]));
}

// childrenOf pairs the children of two objects by field, or of two lists
// whose items all have keys by those keys. It returns undefined for other
// values, and for two lists that hold their common keys in a different
// order.
function childrenOf(
  was: unknown,
  is: unknown,
): [Step, unknown, unknown][] | undefined {
  const pairs = new Map<string, [Step, unknown, unknown]>();
  const add = (step: Step, value: unknown, side: 1 | 2) => {
    const key = JSON.stringify(step);
    const pair = pairs.get(key) ?? [step, undefined, undefined];
    pair[side] = value;
    pairs.set(key, pair);
  };
  if (isObject(was) && isObject(is)) {
    for (const [field, value] of Object.entries(was)) add({ field }, value, 1);
    for (const [field, value] of Object.entries(is)) add({ field }, value, 2);
    return [...pairs.values()];
  }
  if (!Array.isArray(was) || !Array.isArray(is)) return undefined;
  const steps = [was, is].map((list) =>
    list.map((item, index) => itemStep(item, index)),
  );
  if (steps.flat().some((step) => !("item" in step))) return undefined;
  // The order of a list such as extraEnv is part of its value.
  const [before, after] = steps.map((list) =>
    list.map((step) => JSON.stringify(step)),
  );
  const kept = before.filter((key) => after.includes(key));
  if (
    !equalJSON(
      after.filter((key) => before.includes(key)),
      kept,
    )
  )
    return undefined;
  was.forEach((item, i) => add(steps[0][i], item, 1));
  is.forEach((item, i) => add(steps[1][i], item, 2));
  return [...pairs.values()];
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function startsWith(steps: readonly Step[], fields: readonly string[]) {
  return fields.every((name, i) => {
    const step = steps[i];
    return step !== undefined && "field" in step && step.field === name;
  });
}

function leafChanges(
  before: CamundaClusterSpec,
  after: CamundaClusterSpec,
): string[] {
  const leaves = (spec: CamundaClusterSpec) =>
    new Map(
      [...walk(spec, "spec").values()]
        .filter((n) => n.leaf && n.parent !== undefined)
        .map((n) => [n.path, n.value]),
    );
  const was = leaves(before);
  const is = leaves(after);
  const shown = (value: unknown) =>
    value === undefined ? "unset" : JSON.stringify(value);
  return [...new Set([...was.keys(), ...is.keys()])]
    .filter((path) => !equalJSON(was.get(path), is.get(path)))
    .map(
      (path) => `${path}: ${shown(was.get(path))} to ${shown(is.get(path))}.`,
    );
}

function shrinksStorage(
  before: CamundaClusterSpec,
  after: CamundaClusterSpec,
): boolean {
  const was = before.zeebe?.storageSize;
  const is = after.zeebe?.storageSize;
  if (was === undefined || is === undefined) return false;
  try {
    return parseQuantity(String(is)) < parseQuantity(String(was));
  } catch {
    return false;
  }
}

function modeLines(
  input: ClusterChangeInput,
  before: CamundaClusterSpec,
  after: CamundaClusterSpec,
): string[] {
  const name = input.live.metadata?.name ?? "";
  const was = topologyOf(before, []);
  const is = topologyOf(after, []);
  const lines: string[] = [];
  for (const box of is) {
    if (was.some((b) => b.process === box.process)) continue;
    const p = box.process;
    lines.push(
      `Creates Deployment ${name}-${p} and Service ${name}-${p}.`,
      startLine(input, p),
    );
    const host = was.find((b) => b.embedded.some((e) => e.process === p));
    if (host && is.some((b) => b.process === host.process))
      lines.push(`${PROCESS_NAME[host.process]} pods restart.`);
  }
  for (const box of was) {
    const p = box.process;
    if (is.some((b) => b.process === p)) continue;
    const host = is.find((b) => b.embedded.some((e) => e.process === p));
    const removes = `Removes Deployment ${name}-${p} and Service ${name}-${p}.`;
    lines.push(
      host
        ? `${removes} ${PROCESS_NAME[p]} runs in ${host.process === "zeebe" ? "the brokers" : "the gateway"}, which restarts.`
        : removes,
    );
  }
  return lines;
}

function startLine(input: ClusterChangeInput, p: Process): string {
  const prepared = claimAfter(input.session).spec.cluster?.[p];
  const n = [...walk(prepared ?? {}, "").values()].filter(
    (node) =>
      node.leaf &&
      node.parent !== undefined &&
      !startsWith(node.steps, ["mode"]),
  ).length;
  return n > 0
    ? `${PROCESS_NAME[p]} starts with the settings prepared in the claim (${n} fields).`
    : `${PROCESS_NAME[p]} starts from the workload settings of the preset: replicas, CPU, and memory.`;
}
