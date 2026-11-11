import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import { pathFrom, type FieldLock, type Step } from "@/lib/fields";
import { valueAt, type Session } from "@/lib/session";
import { mergedSpecs, type ClusterChangeInput } from "./impacts";
import {
  modeOf,
  PROCESS_LABEL,
  topologyOf,
  type ComponentMode,
  type Process,
  type TopologyBox,
} from "./topology";

/** MODE_PROCESSES are the processes that have a mode, in the order of the editor. */
export const MODE_PROCESSES = [
  "gateway",
  "operate",
  "tasklist",
  "admin",
] as const;

export type ModeProcess = (typeof MODE_PROCESSES)[number];

/** modeSteps returns the claim steps of the mode of p: spec.cluster.<p>.mode. */
export function modeSteps(p: ModeProcess): Step[] {
  return [
    { field: "spec" },
    { field: "cluster" },
    { field: p },
    { field: "mode" },
  ];
}

/**
 * ModeRow is the mode of one process before and after a change. host is
 * the process that it runs in after the change when it is embedded. lock
 * is set when the claim cannot change the mode.
 */
export type ModeRow = {
  process: ModeProcess;
  label: string;
  before: ComponentMode;
  after: ComponentMode;
  host: Process;
  lock?: FieldLock;
};

/** BoxMarks name the boxes that only the topology before or after a change has. */
export type BoxMarks = {
  before: Partial<Record<Process, "removed">>;
  after: Partial<Record<Process, "new">>;
};

/**
 * TopologyChange is the topology of a cluster before and after a change,
 * without pods, and the mode row of each process of MODE_PROCESSES, in
 * that order.
 */
export type TopologyChange = {
  before: TopologyBox[];
  after: TopologyBox[];
  marks: BoxMarks;
  modes: ModeRow[];
};

/**
 * topologyChange returns the topology of the merged spec before and after
 * the pending change of input. It covers every pending change of the
 * session, not only the changes of a mode.
 */
export function topologyChange(input: ClusterChangeInput): TopologyChange {
  const { before, after } = mergedSpecs(input);
  const was = topologyOf(before.spec, []);
  const is = topologyOf(after.spec, []);
  const marks: BoxMarks = { before: {}, after: {} };
  for (const box of was)
    if (!is.some((b) => b.process === box.process))
      marks.before[box.process] = "removed";
  for (const box of is)
    if (!was.some((b) => b.process === box.process))
      marks.after[box.process] = "new";
  const gatewayAfter = modeOf(after.spec, "gateway");
  const modes = MODE_PROCESSES.map((p): ModeRow => {
    const lock = lockOf(input.session, modeSteps(p));
    return {
      process: p,
      label: PROCESS_LABEL[p],
      before: modeOf(before.spec, p),
      after: modeOf(after.spec, p),
      host:
        p === "gateway" || gatewayAfter === "Embedded" ? "zeebe" : "gateway",
      ...(lock && { lock }),
    };
  });
  return { before: was, after: is, marks, modes };
}

// lockOf returns the lock of steps or of a field above it, as setValue
// refuses a change below a locked field.
function lockOf(session: Session, steps: Step[]): FieldLock | undefined {
  for (let i = 1; i <= steps.length; i++) {
    const lock = session.locks(pathFrom("", steps.slice(0, i)));
    if (lock) return lock;
  }
  return undefined;
}

/** ModeWrite is the session change of a pick: a reset of steps, or a value at steps. */
export type ModeWrite =
  { reset: Step[] } | { set: Step[]; value: ComponentMode };

/**
 * modeWrite returns the change of a pick of mode for p, where before is
 * the merged spec of the live cluster. A pick of the mode of before resets
 * the claim field when the claim holds no mode, so that no pending change
 * stays. Every other pick sets the mode.
 */
export function modeWrite(
  session: Session,
  before: CamundaClusterSpec,
  p: ModeProcess,
  mode: ComponentMode,
): ModeWrite {
  const steps = modeSteps(p);
  if (mode === modeOf(before, p) && valueAt(session.claim, steps) === undefined)
    return { reset: steps };
  return { set: steps, value: mode };
}

/** othersPending says whether session holds a pending change that is not the mode of a process of MODE_PROCESSES. */
export function othersPending(session: Session): boolean {
  const paths = new Set(MODE_PROCESSES.map((p) => pathFrom("", modeSteps(p))));
  return [...session.pending.keys()].some((path) => !paths.has(path));
}
