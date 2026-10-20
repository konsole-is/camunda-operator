export type ClusterTab = "overview" | "backups" | "endpoints" | "claim";

export const CLUSTER_TABS: readonly { value: ClusterTab; label: string }[] = [
  { value: "overview", label: "Overview" },
  { value: "backups", label: "Backups" },
  { value: "endpoints", label: "Endpoints" },
  { value: "claim", label: "Claim" },
];

/**
 * clusterTabOf returns the tab that the tab query parameter names. Of an
 * array, the first item counts. An unknown or missing value is overview.
 */
export function clusterTabOf(value: string | string[] | undefined): ClusterTab {
  const first = Array.isArray(value) ? value[0] : value;
  return CLUSTER_TABS.find((t) => t.value === first)?.value ?? "overview";
}
