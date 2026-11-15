import type { PodUsage, Unavailable } from "@/lib/observe/types";
import { highestUseOf, podsReady } from "./cards";
import { type PodLike, podRestarts } from "./pod-table";
import { StatTile } from "./stat-tile";
import { podDot } from "./topology";

/**
 * ComponentStats shows the numbers of the pods of a component: the pods
 * ready, the highest CPU and the highest memory of one pod against its
 * limit, and the restarts of all containers. An undefined usage is not
 * loaded yet.
 */
export function ComponentStats({
  pods,
  usage,
}: {
  pods: PodLike[];
  usage: PodUsage[] | Unavailable | undefined;
}) {
  const ready = podsReady(pods);
  const names = pods.map((p) => p.metadata?.name ?? "");
  const restarts = pods.reduce((n, p) => n + podRestarts(p), 0);
  return (
    <dl className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <StatTile
        label={ready.label}
        value={ready.value}
        note={
          pods.some((p) => podDot(p).tone !== "ok")
            ? "Not all pods are ready"
            : undefined
        }
      />
      <UseTile
        label="Highest CPU"
        highest={highestUseOf(usage, names, ["cpu"])}
      />
      <UseTile
        label="Highest memory"
        highest={highestUseOf(usage, names, ["memory"])}
      />
      <StatTile
        label="Restarts"
        value={String(restarts)}
        note="Since each pod started"
      />
    </dl>
  );
}

function UseTile({
  label,
  highest,
}: {
  label: string;
  highest: ReturnType<typeof highestUseOf>;
}) {
  return typeof highest === "string" ? (
    <StatTile label={label} value={highest} />
  ) : (
    <StatTile
      label={label}
      value={`${Math.round(highest.share * 100)} %`}
      note={`${highest.pod}, of its limit`}
    />
  );
}
