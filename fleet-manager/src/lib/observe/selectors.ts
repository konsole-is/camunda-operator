import {
  COMPONENT_LABEL,
  OPTIMIZE_COMPONENTS,
  OPTIMIZE_UID_LABEL,
} from "@/lib/labels";
import type { MetricsComponent } from "./types";

/**
 * componentObjectSelector returns the label selector of the objects of one
 * component, such as its Services and PVCs, within the objects that
 * clusterSelector selects. For "optimize" it selects the objects of each
 * CamundaOptimize of the cluster, since only the pods carry the UID label.
 */
export function componentObjectSelector(
  clusterSelector: string,
  component: MetricsComponent,
): string {
  return component === "optimize"
    ? `${clusterSelector},${COMPONENT_LABEL} in (${OPTIMIZE_COMPONENTS.join(",")})`
    : `${clusterSelector},${COMPONENT_LABEL}=${component}`;
}

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
  const objects = componentObjectSelector(clusterSelector, component);
  return component === "optimize"
    ? `${objects},${OPTIMIZE_UID_LABEL}=${optimizeUid ?? ""}`
    : objects;
}
