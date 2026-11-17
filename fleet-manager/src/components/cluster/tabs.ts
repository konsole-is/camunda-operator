import { tabOf } from "@/lib/tabs";

export type ClusterTab = "overview" | "backups" | "endpoints" | "claim";

export const CLUSTER_TABS: readonly { value: ClusterTab; label: string }[] = [
  { value: "overview", label: "Overview" },
  { value: "backups", label: "Backups" },
  { value: "endpoints", label: "Endpoints" },
  { value: "claim", label: "Claim" },
];

/**
 * clusterTabOf returns the tab that the tab query parameter names, as tabOf
 * does. The fallback is overview.
 */
export function clusterTabOf(value: string | string[] | undefined): ClusterTab {
  return tabOf(CLUSTER_TABS, value, "overview");
}

/** ClaimPanel names one panel of the Claim tab. A section panel is section:<section name>. */
export type ClaimPanel =
  "basics" | "release" | "attachments" | "deletion" | `section:${string}`;

const CLAIM_PANELS: readonly string[] = [
  "basics",
  "release",
  "attachments",
  "deletion",
];

/** claimPanelHref is the URL of the Claim tab with panel open for editing: ?tab=claim&section=<panel>. */
export function claimPanelHref(panel: ClaimPanel): string {
  return `?tab=claim&section=${encodeURIComponent(panel)}`;
}

/** claimPanelOf returns the panel that the section query parameter names, or undefined for a missing or unknown value. */
export function claimPanelOf(value: string | null): ClaimPanel | undefined {
  if (value === null) return undefined;
  if (CLAIM_PANELS.includes(value)) return value as ClaimPanel;
  return /^section:.+/.test(value) ? (value as ClaimPanel) : undefined;
}
