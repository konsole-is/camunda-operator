import type { StatusTone } from "@/components/fleet/status";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { Resource } from "@/lib/k8s/kinds";
import { COMPONENT_LABEL } from "@/lib/labels";
import type { CoreComponent } from "@/lib/observe/types";

/** Process is the CoreComponent of a pod, as the topology names it. */
export type Process = CoreComponent;

export const PROCESS_LABEL: Record<Process, string> = {
  zeebe: "Brokers",
  gateway: "Gateway",
  operate: "Operate",
  tasklist: "Tasklist",
  admin: "Admin",
  connectors: "Connectors",
};

/** PodDot is the state of one pod as a dot of the topology. */
export type PodDot = { name: string; tone: StatusTone; state: string };

/**
 * TopologyBox is one workload of a cluster. embedded lists the processes
 * that run inside it, and pods are sorted by name.
 */
export type TopologyBox = {
  process: Process;
  label: string;
  workload: "StatefulSet" | "Deployment";
  embedded: { process: Process; label: string }[];
  pods: PodDot[];
};

const WEB_APPS = ["operate", "tasklist", "admin"] as const;

/**
 * topologyOf returns the boxes of a cluster from its merged spec, in the
 * order Brokers, Gateway, Operate, Tasklist, Admin, Connectors. Brokers
 * always run. The gateway is standalone unless its mode is Embedded, and a
 * web app is embedded unless its mode is Standalone. An embedded process is
 * a chip of its host: the brokers for the gateway, and for a web app the
 * gateway when it is standalone, else the brokers. Connectors have a box
 * only when connectors.enabled is true. A pod goes into the box of its
 * COMPONENT_LABEL value. Other pods, such as those of Optimize, are left out.
 */
export function topologyOf(
  spec: CamundaClusterSpec,
  pods: readonly Resource<"Pod">[],
): TopologyBox[] {
  const standalone: Process[] = ["zeebe"];
  const embeddedIn: Record<string, Process[]> = { zeebe: [], gateway: [] };

  if (spec.gateway?.mode === "Embedded") embeddedIn.zeebe.push("gateway");
  else standalone.push("gateway");

  const host = spec.gateway?.mode === "Embedded" ? "zeebe" : "gateway";
  for (const app of WEB_APPS) {
    if (spec[app]?.mode === "Standalone") standalone.push(app);
    else embeddedIn[host].push(app);
  }

  if (spec.connectors?.enabled === true) standalone.push("connectors");

  const sorted = [...pods].sort((a, b) =>
    (a.metadata?.name ?? "").localeCompare(b.metadata?.name ?? ""),
  );
  return standalone.map((process) => ({
    process,
    label: PROCESS_LABEL[process],
    workload: process === "zeebe" ? "StatefulSet" : "Deployment",
    embedded: (embeddedIn[process] ?? []).map((p) => ({
      process: p,
      label: PROCESS_LABEL[p],
    })),
    pods: sorted
      .filter((pod) => pod.metadata?.labels?.[COMPONENT_LABEL] === process)
      .map(podDot),
  }));
}

// The waiting reasons that do not clear without a change to the pod or its
// image.
const BAD_REASONS = new Set([
  "CrashLoopBackOff",
  "ImagePullBackOff",
  "ErrImagePull",
  "CreateContainerConfigError",
  "CreateContainerError",
]);

/**
 * podDot returns the state of a pod, by the first rule that holds: a
 * terminating pod is off, a waiting container shows its reason (bad for a
 * reason that does not clear by itself, else warn), a Failed pod is bad, a
 * Ready pod is ok, and any other pod is warn with its phase, or Unknown.
 */
export function podDot(pod: Resource<"Pod">): PodDot {
  const name = pod.metadata?.name ?? "";
  if (pod.metadata?.deletionTimestamp) {
    return { name, tone: "off", state: "Terminating" };
  }

  const reason = [
    ...(pod.status?.initContainerStatuses ?? []),
    ...(pod.status?.containerStatuses ?? []),
  ].find((c) => c.state?.waiting?.reason)?.state?.waiting?.reason;
  if (reason) {
    return {
      name,
      tone: BAD_REASONS.has(reason) ? "bad" : "warn",
      state: reason,
    };
  }

  if (pod.status?.phase === "Failed") {
    return { name, tone: "bad", state: "Failed" };
  }
  const ready = pod.status?.conditions?.some(
    (c) => c.type === "Ready" && c.status === "True",
  );
  if (ready) return { name, tone: "ok", state: "Ready" };
  return { name, tone: "warn", state: pod.status?.phase ?? "Unknown" };
}
