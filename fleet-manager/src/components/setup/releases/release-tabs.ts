/** ReleaseTab names one tab of the release page. */
export type ReleaseTab = "settings" | "rollout";

/** RELEASE_TABS are the tabs that the release page shows, in order. */
export const RELEASE_TABS: readonly { value: ReleaseTab; label: string }[] = [
  { value: "settings", label: "Settings" },
  { value: "rollout", label: "Rollout" },
];

/**
 * releaseTabOf returns the tab that the tab query parameter names. Of an
 * array, the first item counts. A value that RELEASE_TABS does not list, or
 * no value, is settings.
 */
export function releaseTabOf(value: string | string[] | undefined): ReleaseTab {
  const first = Array.isArray(value) ? value[0] : value;
  return RELEASE_TABS.find((t) => t.value === first)?.value ?? "settings";
}
