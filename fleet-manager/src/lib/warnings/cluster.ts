import type { PluginsPluginRequestConditionType } from "@/generated/conditions";
import type { SectionComponent } from "@/lib/claims/sections";
import type { Warning } from "./types";

const FAILED = "Failed";
const READY: PluginsPluginRequestConditionType = "Ready";

/** backupWarning returns a warning when newest, the newest backup of a cluster, has phase Failed. */
export function backupWarning(
  newest: { name: string; phase: string } | undefined,
): Warning | undefined {
  if (newest?.phase !== FAILED) return undefined;
  return {
    kind: "backup",
    severity: "Warning",
    label: "Backup failed",
    detail: `The newest backup ${newest.name} failed`,
  };
}

/**
 * requestWarnings returns, for each component in order, a warning with the
 * severity of each status.summary item of its request that has severity
 * Warning or Error, then an error when its Ready condition is False.
 */
export function requestWarnings(
  components: readonly SectionComponent[],
): Warning[] {
  return components.flatMap((component) => {
    const status = component.request?.status;
    const summaryTime = status?.summaryTime;
    const summary = (status?.summary ?? []).flatMap((item): Warning[] =>
      item.severity === "Warning" || item.severity === "Error"
        ? [
            {
              kind: "summary",
              severity: item.severity,
              label: item.label,
              detail: `${item.label}: ${item.value}`,
              ...(summaryTime ? { since: summaryTime } : {}),
            },
          ]
        : [],
    );
    const condition = component.condition;
    if (condition?.status !== "False") return summary;
    const since = status?.conditions?.find(
      (c) => c.type === READY,
    )?.lastTransitionTime;
    return [
      ...summary,
      {
        kind: "request",
        severity: "Error",
        label: `${component.label} not ready`,
        detail: [condition.reason, condition.message]
          .filter(Boolean)
          .join(": "),
        ...(since ? { since } : {}),
      },
    ];
  });
}
