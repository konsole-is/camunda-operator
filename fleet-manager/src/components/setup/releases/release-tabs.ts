import { tabOf } from "@/lib/tabs";

/** ReleaseTab names one tab of the release page. */
export type ReleaseTab = "settings" | "rollout";

/** RELEASE_TABS are the tabs that the release page shows, in order. */
export const RELEASE_TABS: readonly { value: ReleaseTab; label: string }[] = [
  { value: "settings", label: "Settings" },
  { value: "rollout", label: "Rollout" },
];

/**
 * releaseTabOf returns the tab that the tab query parameter names, as tabOf
 * does. The fallback is settings.
 */
export function releaseTabOf(value: string | string[] | undefined): ReleaseTab {
  return tabOf(RELEASE_TABS, value, "settings");
}
