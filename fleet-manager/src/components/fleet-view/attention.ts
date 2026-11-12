import type { BackupObject } from "@/components/cluster/backups";
import type { Plane } from "@/components/planes/plane";
import type { PluginsPluginRegistrationConditionType } from "@/generated/conditions";
import type { Resource } from "@/lib/k8s/kinds";
import { byCodeUnit } from "@/lib/policy";
import type { ClusterRow } from "./model";

/** FAILED_BACKUP_MS is how long a failed backup stays in Needs attention. */
export const FAILED_BACKUP_MS = 86_400_000;

const ACCEPTED: PluginsPluginRegistrationConditionType = "Accepted";
const READY: PluginsPluginRegistrationConditionType = "Ready";

/**
 * AttentionItem is one entry of the Needs attention panel. what is
 * "Cluster", "Backup", "Option", or "Management plane". href is the page of
 * the object, and an object without a page has none.
 */
export type AttentionItem = {
  id: string;
  what: string;
  name: string;
  reason?: string;
  message?: string;
  href?: string;
};

/**
 * attentionItems returns, in this order: each cluster row with Ready False,
 * each backup with phase Failed that finished (or, without completionTime,
 * was created) in the FAILED_BACKUP_MS before now, each registration with
 * Accepted or Ready False, and each plane with Ready False. A namespaced
 * object is named "<namespace>/<name>", and a registration by its label or
 * its name. A registration and a plane have no href yet.
 */
export function attentionItems({
  rows,
  backups,
  registrations,
  planes,
  now,
}: {
  rows: readonly ClusterRow[];
  backups: readonly BackupObject[];
  registrations: readonly Resource<"PluginRegistration">[];
  planes: readonly Plane[];
  now: string;
}): AttentionItem[] {
  const clusters = rows.flatMap((row): AttentionItem[] =>
    row.ready?.status === "False"
      ? [
          {
            id: row.id,
            what: "Cluster",
            name: `${row.namespace}/${row.name}`,
            ...defined(row.ready.reason, row.ready.message),
            ...(row.href ? { href: row.href } : {}),
          },
        ]
      : [],
  );

  const since = Date.parse(now) - FAILED_BACKUP_MS;
  const failed = backups
    .filter((b) => {
      if (b.status?.phase !== "Failed") return false;
      const at = Date.parse(
        b.status.completionTime ?? b.metadata?.creationTimestamp ?? "",
      );
      return at >= since;
    })
    .map((b): AttentionItem => {
      const namespace = b.metadata?.namespace ?? "";
      return {
        id: `backup/${b.kind}/${namespace}/${b.metadata?.name}`,
        what: "Backup",
        name: `${namespace}/${b.metadata?.name ?? ""}`,
        ...defined(undefined, b.status?.failureMessage),
        href: `/fleet/clusters/${namespace}/${b.spec.clusterRef.name}?tab=backups`,
      };
    })
    .sort((a, b) => byCodeUnit(a.name, b.name));

  const options = [...registrations]
    .sort((a, b) => byCodeUnit(a.metadata?.name ?? "", b.metadata?.name ?? ""))
    .flatMap((r): AttentionItem[] => {
      const conditions = r.status?.conditions ?? [];
      const down =
        conditions.find((c) => c.type === ACCEPTED && c.status === "False") ??
        conditions.find((c) => c.type === READY && c.status === "False");
      if (!down) return [];
      const name = r.metadata?.name ?? "";
      return [
        {
          id: `registration/${name}`,
          what: "Option",
          name: r.spec.label || name,
          ...defined(down.reason, down.message),
        },
      ];
    });

  const downPlanes = planes.flatMap((plane): AttentionItem[] =>
    plane.ready?.status === "False"
      ? [
          {
            // A plane id has the same form as the id of a cluster row.
            id: `plane/${plane.id}`,
            what: "Management plane",
            name: `${plane.namespace}/${plane.name}`,
            ...defined(plane.ready.reason, plane.ready.message),
          },
        ]
      : [],
  );

  return [...clusters, ...failed, ...options, ...downPlanes];
}

function defined(
  reason: string | undefined,
  message: string | undefined,
): { reason?: string; message?: string } {
  return { ...(reason ? { reason } : {}), ...(message ? { message } : {}) };
}
