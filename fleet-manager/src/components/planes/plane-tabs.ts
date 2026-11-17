import { tabOf } from "@/lib/tabs";

/** PlaneTab names one tab of the plane page. */
export type PlaneTab =
  "overview" | "clusters" | "optimize" | "contract" | "endpoints" | "claim";

/** PLANE_TABS are the tabs of the plane page, in their order. */
export const PLANE_TABS: readonly { value: PlaneTab; label: string }[] = [
  { value: "overview", label: "Overview" },
  { value: "clusters", label: "Clusters served" },
  { value: "optimize", label: "Optimize" },
  { value: "contract", label: "Contract" },
  { value: "endpoints", label: "Endpoints" },
  { value: "claim", label: "Claim" },
];

/**
 * planeTabOf returns the tab that the tab query parameter names, as tabOf
 * does. The fallback is overview.
 */
export function planeTabOf(value: string | string[] | undefined): PlaneTab {
  return tabOf(PLANE_TABS, value, "overview");
}
