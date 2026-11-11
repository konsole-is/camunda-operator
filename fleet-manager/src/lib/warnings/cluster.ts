import type { SectionComponent } from "@/lib/claims/sections";
import type { Warning } from "./types";

const FAILED = "Failed";

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
    const summary = (component.request?.status?.summary ?? []).flatMap(
      (item): Warning[] =>
        item.severity === "Warning" || item.severity === "Error"
          ? [
              {
                kind: "summary",
                severity: item.severity,
                label: item.label,
                detail: `${item.label}: ${item.value}`,
              },
            ]
          : [],
    );
    const condition = component.condition;
    if (condition?.status !== "False") return summary;
    return [
      ...summary,
      {
        kind: "request",
        severity: "Error",
        label: `${component.label} not ready`,
        detail: [condition.reason, condition.message]
          .filter(Boolean)
          .join(": "),
      },
    ];
  });
}
