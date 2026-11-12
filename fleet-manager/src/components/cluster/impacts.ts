import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import { itemStep, walk, type Step } from "@/lib/fields";
import { SLOT_FIELDS } from "@/lib/fields/slots";
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
import { podInputs, type PodContext } from "./pod-inputs";
import { topologyOf, type Process } from "./topology";

/**
 * ClusterChangeInput is a session over the claim of a cluster, with the
 * live cluster and the presets, releases, and sections that the change can
 * name. Without environment, a change that removes the release of the
 * claim has no release after it.
 */
export type ClusterChangeInput = PodContext & {
  session: Session<Resource<"CamundaClusterClaim">>;
  live: Resource<"CamundaCluster">;
  presets: readonly Resource<"CamundaClusterPreset">[];
  releases: readonly Resource<"CamundaRelease">[];
  sections: readonly Resource<"PluginSection">[];
  environment?: Resource<"Environment">;
};

const RESTART_ALL =
  "The brokers restart one at a time. The other processes restart.";

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

// These fields of a process change its workload but not its pod template.
// The operator grows the volume claims for storageSize, and keeps the
// storage class of the applied claim template.
const OUTSIDE_POD = new Set([
  "replicas",
  "storageSize",
  "storageClassName",
  "persistentVolumeClaimRetentionPolicy",
]);

// The operator applies these fields of an embedded process to its host.
const HOST_FIELDS = new Set(["extraEnv", "extraEnvFrom"]);

// The impact rules read a change of these keys of a claim spec. A change of
// another key, such as plugins, can reach the cluster in a way that the
// review cannot see.
const CLAIM_KEYS = new Set([
  "presetRef",
  "releaseRef",
  "platformConfigRef",
  "cluster",
  "suspend",
  "deletionPolicy",
  "attachments",
  "allowVersionDowngrade",
]);

// The operator keeps none of these keys of the user on a pod template.
const OPERATOR_ANNOTATIONS = [
  "camunda.io/config-hash",
  "camunda.io/service-account-hash",
];
const OPERATOR_LABELS = [
  "camunda.io/cluster",
  "camunda.io/cluster-uid",
  "camunda.io/component",
  "camunda.io/storage-claim",
];

/**
 * FieldReach says what a change of a top-level field of CamundaClusterSpec
 * can restart: "brokers" the brokers and other processes, "others" only
 * processes other than the brokers, "outside" no pod.
 */
export type FieldReach = "brokers" | "others" | "outside";

/**
 * SPEC_FIELDS gives the reach of each top-level field of CamundaClusterSpec.
 * restartsBrokers counts a change of a field that is not in it as a change
 * that restarts the brokers.
 */
export const SPEC_FIELDS = {
  platformConfigRef: "brokers",
  presetRef: "brokers",
  releaseRef: "brokers",
  version: "brokers",
  externalUrl: "brokers",
  serviceAccount: "brokers",
  auth: "brokers",
  zeebe: "brokers",
  gateway: "brokers",
  operate: "brokers",
  tasklist: "brokers",
  admin: "brokers",
  extraEnv: "brokers",
  extraEnvFrom: "brokers",
  podLabels: "brokers",
  podAnnotations: "brokers",
  scheduling: "brokers",
  storageRef: "brokers",
  indexReplicas: "brokers",
  backupStorageRef: "brokers",
  documentStorageRef: "brokers",
  backup: "brokers",
  suspend: "brokers",
  connectors: "brokers",
  monitoring: "outside",
  pause: "outside",
} as const satisfies Record<keyof CamundaClusterSpec, FieldReach>;

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
  return impactsOf(input).lines;
}

/**
 * restartsBrokers says whether the change restarts the broker pods. It is
 * true exactly when an impact line of the change says that the brokers
 * restart or can restart. It is true also when the review cannot decide: for
 * a change of a section, or a preset or a release that the input does not
 * hold.
 */
export function restartsBrokers(input: ClusterChangeInput): boolean {
  return impactsOf(input).brokers;
}

function impactsOf(input: ClusterChangeInput): {
  lines: string[];
  brokers: boolean;
} {
  const { session, live } = input;
  const { before, after } = mergedSpecs(input);
  const refs = clusterAfter(input);
  const was = session.claim.spec;
  const is = claimAfter(session).spec;
  const changed = (name: keyof typeof was) => !equalJSON(was[name], is[name]);
  const cluster = clusterChanges(input);
  const lines = new Set<string>();
  let brokers = false;
  const restart = (p: Process) => {
    lines.add(`${PROCESS_NAME[p]} pods restart.`);
    if (p === "zeebe") brokers = true;
  };

  // The config hash of every process takes the name and the generation of
  // the release, the platform config, and the storage of the cluster, and
  // the name and the fingerprint of the preset.
  const restartsAll =
    (["presetRef", "releaseRef", "platformConfigRef"] as const).some(
      (name) => changed(name) && refs[name] !== live.spec[name],
    ) || before.spec.storageRef !== after.spec.storageRef;
  if (restartsAll) {
    lines.add(RESTART_ALL);
    brokers = true;
  }

  if (changed("presetRef"))
    for (const line of leafChanges(
      { ...before.spec, presetRef: undefined },
      { ...after.spec, presetRef: undefined },
    ))
      lines.add(line);

  const boxes = topologyOf(after.spec, []);
  const had = new Set(topologyOf(before.spec, []).map((b) => b.process));
  const shrinks = shrinksStorage(before.spec, after.spec);
  const applies: string[] = [];
  for (const steps of cluster) {
    const [first, second] = steps;
    if (!("field" in first) || !Object.hasOwn(PROCESS_NAME, first.field))
      continue;
    const process = first.field as Process;
    const field = second && "field" in second ? second.field : undefined;
    if (isSwitch(process, field)) continue;
    const host = boxes.find((b) =>
      b.embedded.some((e) => e.process === process),
    );
    if (boxes.some((b) => b.process === process)) {
      if (had.has(process) && !OUTSIDE_POD.has(field ?? "")) restart(process);
    } else if (host && field !== undefined && HOST_FIELDS.has(field))
      restart(host.process);
    else applies.push(`Applies when ${PROCESS_NAME[process]} runs standalone.`);
  }

  // A new version changes the image of every process except the connectors,
  // which take connectors.version.
  const newImage = (process: Process) =>
    process !== "connectors" && before.spec.version !== after.spec.version;
  if (!restartsAll)
    for (const { process } of boxes)
      if (
        had.has(process) &&
        (newImage(process) ||
          !equalJSON(
            podInputs(before.spec, input, process),
            podInputs(after.spec, input, process),
          ))
      )
        restart(process);
  for (const line of applies) lines.add(line);

  if (
    cluster.some(([first]) => "field" in first && CLUSTER_WIDE.has(first.field))
  ) {
    if (!equalJSON(brokerPod(before.spec), brokerPod(after.spec)))
      restart("zeebe");
    lines.add(
      "Every process restarts, except the processes that set the field themselves.",
    );
  }

  const modes = modeLines(input, before.spec, after.spec);
  for (const line of modes.lines) lines.add(line);
  if (modes.brokers) brokers = true;

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

  if (is.suspend === true && was.suspend !== true) {
    lines.add("Every workload goes to zero. The volumes stay.");
    brokers = true;
  }

  if (
    !equalJSON(was.attachments?.backupSchedule, is.attachments?.backupSchedule)
  )
    lines.add("Nothing restarts.");

  if (!brokers && undecided(input)) {
    lines.add("Broker pods can restart.");
    brokers = true;
  }

  return { lines: [...lines], brokers };
}

// undecided says whether the review cannot tell if the change restarts the
// brokers. A preset or a release that the input does not hold makes the
// merged specs wrong. A section fills slot fields of the cluster only after
// its outputs change.
function undecided(input: ClusterChangeInput): boolean {
  const refs = clusterAfter(input);
  const held = (
    list: readonly { metadata?: { name?: string } }[],
    name: string | undefined,
  ) => !name || list.some((r) => r.metadata?.name === name);
  if (
    !held(input.presets, input.live.spec.presetRef) ||
    !held(input.presets, refs.presetRef) ||
    !held(input.releases, input.live.spec.releaseRef) ||
    !held(input.releases, refs.releaseRef)
  )
    return true;
  const was: Record<string, unknown> = input.session.claim.spec;
  const is: Record<string, unknown> = claimAfter(input.session).spec;
  const keys = new Set([...Object.keys(was), ...Object.keys(is)]);
  if ([...keys].some((k) => !CLAIM_KEYS.has(k) && !equalJSON(was[k], is[k])))
    return true;
  return clusterChanges(input).some(
    ([first]) =>
      !("field" in first) || !Object.hasOwn(SPEC_FIELDS, first.field),
  );
}

type PodBlock = {
  extraEnv?: { name: string }[];
  extraEnvFrom?: unknown[];
  podLabels?: Record<string, string>;
  podAnnotations?: Record<string, string>;
};

// brokerPod returns the cluster-wide settings of the broker pod template as
// workloadMutations in pkg/components/camundacluster/mutations.go and userEnv
// in render.go render them: a scheduling block of the brokers replaces the
// cluster one, and the last variable of a name wins.
function brokerPod(spec: CamundaClusterSpec): unknown {
  const zeebe: PodBlock = spec.zeebe ?? {};
  const embedded =
    topologyOf(spec, [])
      .find((b) => b.process === "zeebe")
      ?.embedded.map((e): PodBlock => spec[e.process] ?? {}) ?? [];
  const sources = [...embedded, zeebe];
  const env = new Map<string, unknown>();
  for (const e of [
    ...(spec.extraEnv ?? []),
    ...sources.flatMap((s) => s.extraEnv ?? []),
  ])
    env.set(e.name, e);
  const annotations: Record<string, string> = {
    ...spec.podAnnotations,
    ...zeebe.podAnnotations,
  };
  for (const key of OPERATOR_ANNOTATIONS) delete annotations[key];
  const labels: Record<string, string> = {
    ...spec.podLabels,
    ...zeebe.podLabels,
  };
  for (const key of OPERATOR_LABELS) delete labels[key];
  return {
    scheduling: spec.zeebe?.scheduling ?? spec.scheduling,
    podLabels: labels,
    podAnnotations: annotations,
    extraEnv: [...env.values()],
    extraEnvFrom: [
      ...(spec.extraEnvFrom ?? []),
      ...sources.flatMap((s) => s.extraEnvFrom ?? []),
    ],
  };
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
  return refAfter(
    input,
    "releaseRef",
    input.environment?.spec.releases?.default,
  );
}

function refAfter(
  input: ClusterChangeInput,
  name: "releaseRef" | "platformConfigRef",
  fallback: string | undefined,
): string | undefined {
  const was = input.session.claim.spec[name];
  const is = claimAfter(input.session).spec[name];
  if (was === is) return input.live.spec[name];
  return is || fallback;
}

// clusterAfter returns the spec that cluster in
// pkg/components/camundaclusterclaim/cluster.go makes from the claim after
// the change. A slot field keeps its live value, as it does while the
// output of its section is not ready.
function clusterAfter(input: ClusterChangeInput): CamundaClusterSpec {
  const claim = claimAfter(input.session).spec;
  let spec: CamundaClusterSpec = structuredClone(claim.cluster ?? {});
  const slots = new Set(
    (claim.plugins ?? []).map(
      (p) =>
        input.sections.find((s) => s.metadata?.name === p.section)?.spec.slot,
    ),
  );
  const fromLive = [
    ["serviceAccount", "name"],
    ["connectors", "serviceAccount", "name"],
    ...SLOT_FIELDS.filter(
      (row) =>
        row.claimKind === "CamundaClusterClaim" &&
        row.kind === "CamundaCluster" &&
        slots.has(row.slot),
    ).map((row) => row.names.slice(1)),
  ].map((names) => names.map((field): Step => ({ field })));
  const fields: [Step[], unknown][] = [
    [[{ field: "presetRef" }], claim.presetRef],
    [[{ field: "releaseRef" }], releaseAfter(input)],
    [
      [{ field: "platformConfigRef" }],
      refAfter(
        input,
        "platformConfigRef",
        input.environment?.spec.platformConfigRef,
      ),
    ],
    [[{ field: "suspend" }], claim.suspend],
    ...fromLive.map((steps): [Step[], unknown] => [
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

// modeLines returns the lines of the processes that a mode switch creates or
// removes. brokers is true when a process moves into or out of the brokers.
function modeLines(
  input: ClusterChangeInput,
  before: CamundaClusterSpec,
  after: CamundaClusterSpec,
): { lines: string[]; brokers: boolean } {
  const name = input.live.metadata?.name ?? "";
  const was = topologyOf(before, []);
  const is = topologyOf(after, []);
  const lines: string[] = [];
  let brokers = false;
  for (const box of is) {
    if (was.some((b) => b.process === box.process)) continue;
    const p = box.process;
    lines.push(
      `Creates Deployment ${name}-${p} and Service ${name}-${p}.`,
      startLine(input, p),
    );
    const host = was.find((b) => b.embedded.some((e) => e.process === p));
    if (host && is.some((b) => b.process === host.process)) {
      lines.push(`${PROCESS_NAME[host.process]} pods restart.`);
      if (host.process === "zeebe") brokers = true;
    }
  }
  for (const box of was) {
    const p = box.process;
    if (is.some((b) => b.process === p)) continue;
    const host = is.find((b) => b.embedded.some((e) => e.process === p));
    if (host?.process === "zeebe") brokers = true;
    const removes = `Removes Deployment ${name}-${p} and Service ${name}-${p}.`;
    lines.push(
      host
        ? `${removes} ${PROCESS_NAME[p]} runs in ${host.process === "zeebe" ? "the brokers" : "the gateway"}, which restarts.`
        : removes,
    );
  }
  return { lines, brokers };
}

function startLine(input: ClusterChangeInput, p: Process): string {
  const prepared = claimAfter(input.session).spec.cluster?.[p];
  const n = [...walk(prepared ?? {}, "").values()].filter(
    (node) =>
      node.leaf &&
      node.parent !== undefined &&
      !(
        node.steps[0] &&
        "field" in node.steps[0] &&
        isSwitch(p, node.steps[0].field)
      ),
  ).length;
  return n > 0
    ? `${PROCESS_NAME[p]} starts with the settings prepared in the claim (${n} fields).`
    : `${PROCESS_NAME[p]} starts from the workload settings of the preset: replicas, CPU, and memory.`;
}

// isSwitch says whether field of process p creates or removes its workload.
function isSwitch(p: Process, field: string | undefined): boolean {
  return field === "mode" || (p === "connectors" && field === "enabled");
}
