import "server-only";
import fs from "node:fs/promises";
import { parse } from "yaml";
import { get, list } from "@/lib/k8s/read";
import {
  type PodMetrics,
  podUsage as realPodUsage,
  podUsageFrom,
} from "./kube";
import {
  type PodLogRequest,
  followPodLog as realFollowPodLog,
} from "./pod-log";
import { OBSERVE_FILE_VARIABLE } from "./observe-variable";
import type { PodUsage, Unavailable } from "./types";

export { OBSERVE_FILE_VARIABLE };

/**
 * ObserveFile is the observe.yaml of a page scenario: the log lines and the
 * usage that envtest cannot give, since it has no kubelet and no
 * metrics.k8s.io. A log without container is the log of each container.
 */
export type ObserveFile = {
  logs: {
    namespace: string;
    pod: string;
    container?: string;
    lines: string[];
  }[];
  usage: { namespace: string; pod: string; cpu: string; memory: string }[];
};

/** parseObserveFile reads observe.yaml. It throws unless schemaVersion is 1. A missing list is empty. */
export function parseObserveFile(text: string): ObserveFile {
  const file = (parse(text) ?? {}) as Partial<ObserveFile> & {
    schemaVersion?: unknown;
  };
  if (file.schemaVersion !== 1) {
    throw new Error(
      `observe.yaml has schemaVersion ${String(file.schemaVersion)}. The fleet manager reads schemaVersion 1`,
    );
  }
  return { logs: file.logs ?? [], usage: file.usage ?? [] };
}

// readObserveFile reads the file on each call, so a test can change it
// while the app runs. No variable and no file give undefined.
async function readObserveFile(): Promise<ObserveFile | undefined> {
  const file = process.env[OBSERVE_FILE_VARIABLE];
  if (!file) return undefined;
  try {
    return parseObserveFile(await fs.readFile(file, "utf8"));
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") return undefined;
    throw error;
  }
}

/**
 * podUsage is podUsage of kube.ts, with the usage of observe.yaml in place
 * of metrics.k8s.io for the pods that the file names. Without such a pod it
 * returns what the real reader returns.
 */
export async function podUsage(
  namespace: string,
  selector: string,
): Promise<PodUsage[] | Unavailable> {
  const entries = (await readObserveFile())?.usage.filter(
    (u) => u.namespace === namespace,
  );
  if (!entries?.length) return realPodUsage(namespace, selector);
  const pods = await list("Pod", {
    namespace,
    ...(selector && { labelSelector: selector }),
  });
  const selected = new Set(pods.map((p) => p.metadata?.name));
  const items = entries.filter((u) => selected.has(u.pod));
  if (!items.length) return realPodUsage(namespace, selector);
  const metrics: PodMetrics = {
    items: items.map((u) => ({
      metadata: { name: u.pod },
      containers: [{ usage: { cpu: u.cpu, memory: u.memory } }],
    })),
  };
  return podUsageFrom(namespace, selector, metrics);
}

/**
 * followPodLog is followPodLog of pod-log.ts, with the lines of observe.yaml
 * for a pod that exists and that the file names, when the pod has the
 * container and the file names that container or no container. That stream
 * gives the last tailLines lines and then closes. Otherwise it returns what
 * the real reader returns, such as PodLogNotFound for a pod that does not
 * exist.
 */
export async function followPodLog(
  req: PodLogRequest,
  signal: AbortSignal,
): Promise<ReadableStream<string>> {
  const logs = (await readObserveFile())?.logs.filter(
    (l) => l.namespace === req.namespace && l.pod === req.pod,
  );
  if (!logs?.length) return realFollowPodLog(req, signal);
  const pod = await get("Pod", req.namespace, req.pod);
  // As in the real reader, no container means the first container of the pod.
  const container = req.container ?? pod?.spec?.containers?.[0]?.name;
  const names = [
    ...(pod?.spec?.containers ?? []),
    ...(pod?.spec?.initContainers ?? []),
    ...(pod?.spec?.ephemeralContainers ?? []),
  ].map((c) => c.name);
  const log =
    container && names.includes(container)
      ? logs.find((l) => !l.container || l.container === container)
      : undefined;
  if (!log) return realFollowPodLog(req, signal);
  const lines = req.tailLines ? log.lines.slice(-req.tailLines) : log.lines;
  return new ReadableStream<string>({
    start(controller) {
      for (const line of lines) controller.enqueue(line);
      controller.close();
    },
  });
}
