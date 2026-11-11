import {
  COMPONENT_LABEL,
  OPTIMIZE_COMPONENTS,
  OPTIMIZE_UID_LABEL,
} from "@/lib/labels";
import type { MetricsComponent } from "./types";

/**
 * componentPodSelector returns the label selector of the pods of one
 * component, within the pods that clusterSelector selects. For "optimize"
 * it selects the webapp and importer pods of the CamundaOptimize with
 * optimizeUid; without optimizeUid it selects no pod.
 */
export function componentPodSelector(
  clusterSelector: string,
  component: MetricsComponent,
  optimizeUid?: string,
): string {
  if (component === "optimize") {
    return [
      clusterSelector,
      `${COMPONENT_LABEL} in (${OPTIMIZE_COMPONENTS.join(",")})`,
      `${OPTIMIZE_UID_LABEL}=${optimizeUid ?? ""}`,
    ].join(",");
  }
  return `${clusterSelector},${COMPONENT_LABEL}=${component}`;
}
