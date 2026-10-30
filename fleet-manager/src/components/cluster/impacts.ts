import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import { walk, type Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { mergeCluster, type Merged } from "@/lib/merge";
import { parseQuantity } from "@/lib/observe/quantity";
import { equalJSON } from "@/lib/policy";
import {
  claimAfter,
  without,
  withValue,
  type PendingChange,
  type Session,
} from "@/lib/session";
import { topologyOf, type Process } from "./topology";

/**
 * ClusterChangeInput is a session over the claim of a cluster, with the
 * live cluster and the presets, releases, and sections that the change can
 * name.
 */
export type ClusterChangeInput = {
  session: Session<Resource<"CamundaClusterClaim">>;
  live: Resource<"CamundaCluster">;
  presets: readonly Resource<"CamundaClusterPreset">[];
  releases: readonly Resource<"CamundaRelease">[];
  sections: readonly Resource<"PluginSection">[];
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
]);

/**
 * mergedSpecs returns the merged spec of the cluster before the change and
 * after it. after applies each pending change under spec.cluster to the
 * live spec, with the preset and the release of the claim after the change.
 */
export function mergedSpecs(input: ClusterChangeInput): {
  before: Merged<CamundaClusterSpec>;
  after: Merged<CamundaClusterSpec>;
} {
  const { live, session } = input;
  const claim = claimAfter(session);
  let overrides = live.spec;
  for (const entry of session.pending.values()) {
    const rest = clusterSteps(entry);
    if (rest === undefined) continue;
    overrides =
      "reset" in entry
        ? without(overrides, rest)
        : withValue(overrides, rest, entry.value);
  }
  return {
    before: merged(input, live.spec.presetRef, live.spec.releaseRef, live.spec),
    after: merged(
      input,
      claim.spec.presetRef,
      claim.spec.releaseRef,
      overrides,
    ),
  };
}

/**
 * clusterImpacts returns the impact lines of a change, in the order of the
 * impact table of the review, each once.
 */
export function clusterImpacts(input: ClusterChangeInput): string[] {
  const { session, live } = input;
  const { before, after } = mergedSpecs(input);
  const entries = [...session.pending.values()];
  const claim = claimAfter(session);
  const lines = new Set<string>();

  const release = top(entries, "releaseRef");
  if (release && !("reset" in release)) {
    const version = (name: string | undefined) =>
      input.releases.find((r) => r.metadata?.name === name)?.spec.version;
    if (version(live.spec.releaseRef) !== version(claim.spec.releaseRef))
      lines.add(
        "The brokers restart one at a time. The other processes restart.",
      );
  }

  if (top(entries, "presetRef"))
    for (const line of leafChanges(before.spec, after.spec)) lines.add(line);

  const standaloneAfter = new Set(
    topologyOf(after.spec, []).map((b) => b.process),
  );
  let shrinks = false;
  for (const entry of entries) {
    const rest = clusterSteps(entry);
    const [first, second] = rest ?? [];
    if (!first || !("field" in first)) continue;
    const process = first.field as Process;
    if (!Object.hasOwn(PROCESS_NAME, process)) continue;
    if (second && "field" in second && second.field === "mode") continue;
    if (shrinksStorage(entry, rest!, before.spec)) {
      shrinks = true;
      continue;
    }
    lines.add(
      standaloneAfter.has(process)
        ? `${PROCESS_NAME[process]} pods restart.`
        : `Applies when ${PROCESS_NAME[process]} runs standalone.`,
    );
  }

  if (
    entries.some((e) => {
      const [first] = clusterSteps(e) ?? [];
      return first && "field" in first && CLUSTER_WIDE.has(first.field);
    })
  )
    lines.add(
      "Every process restarts, except the processes that set the field themselves.",
    );

  for (const line of modeLines(input, before.spec, after.spec)) lines.add(line);

  if (shrinks)
    lines.add("Volumes cannot shrink. The operator keeps the current size.");

  for (const entry of entries) {
    const section = parameterSection(entry);
    if (section === undefined) continue;
    const label =
      input.sections.find((s) => s.metadata?.name === section)?.spec.label ??
      section;
    lines.add(`The ${label} option applies the new value.`);
  }

  if (top(entries, "deletionPolicy"))
    lines.add(
      "Nothing restarts. The policy applies when the cluster is deleted.",
    );

  const suspend = top(entries, "suspend");
  if (suspend && !("reset" in suspend) && suspend.value === true)
    lines.add("Every workload goes to zero. The volumes stay.");

  if (
    entries.some((e) =>
      startsWith(e.steps, ["spec", "attachments", "backupSchedule"]),
    )
  )
    lines.add("Nothing restarts.");

  return [...lines];
}

/** showsMergedDiff says whether the review shows the merged diff: a pending presetRef or releaseRef, or a reset. */
export function showsMergedDiff(s: Session): boolean {
  const entries = [...s.pending.values()];
  return (
    entries.some((e) => "reset" in e) ||
    top(entries, "presetRef") !== undefined ||
    top(entries, "releaseRef") !== undefined
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

// clusterSteps returns the steps of entry below spec.cluster, or undefined
// for an entry outside it.
function clusterSteps(entry: PendingChange): Step[] | undefined {
  return startsWith(entry.steps, ["spec", "cluster"]) && entry.steps.length > 2
    ? entry.steps.slice(2)
    : undefined;
}

// top returns the pending change of the top-level claim field name.
function top(
  entries: readonly PendingChange[],
  name: string,
): PendingChange | undefined {
  return entries.find(
    (e) => e.steps.length === 2 && startsWith(e.steps, ["spec", name]),
  );
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
  entry: PendingChange,
  rest: readonly Step[],
  before: CamundaClusterSpec,
): boolean {
  if ("reset" in entry || rest.length !== 2) return false;
  if (!startsWith(rest, ["zeebe", "storageSize"])) return false;
  const current = before.zeebe?.storageSize;
  if (current === undefined) return false;
  try {
    return parseQuantity(String(entry.value)) < parseQuantity(String(current));
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

// parameterSection returns the section of a pending section parameter.
function parameterSection(entry: PendingChange): string | undefined {
  const [, , selection, parameters] = entry.steps;
  if (
    !startsWith(entry.steps, ["spec", "plugins"]) ||
    !selection ||
    !("item" in selection) ||
    !parameters ||
    !("field" in parameters && parameters.field === "parameters")
  )
    return undefined;
  const section = selection.item.section;
  return typeof section === "string" ? section : undefined;
}
