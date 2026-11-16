/** PlaneTab names one tab of the plane page. */
export type PlaneTab =
  "overview" | "clusters" | "optimize" | "contract" | "endpoints" | "claim";

/** PLANE_TABS are the tabs of the plane page, in their order. */
export const PLANE_TABS: readonly { id: PlaneTab; label: string }[] = [
  { id: "overview", label: "Overview" },
];

/**
 * planeTabOf returns the tab that the tab query parameter names. Of an
 * array, the first item counts. A missing value, or one that PLANE_TABS does
 * not list, is overview.
 */
export function planeTabOf(value: string | string[] | undefined): PlaneTab {
  const first = Array.isArray(value) ? value[0] : value;
  return PLANE_TABS.find((t) => t.id === first)?.id ?? "overview";
}
