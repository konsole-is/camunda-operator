/** RegistrationTab names one tab of the registration page. */
export type RegistrationTab =
  "about" | "contract" | "parameters" | "grants" | "health" | "usedBy";

/** REGISTRATION_TABS are the tabs that the registration page shows, in order. */
export const REGISTRATION_TABS: readonly {
  value: RegistrationTab;
  label: string;
}[] = [
  { value: "about", label: "About" },
  { value: "contract", label: "Contract" },
  { value: "parameters", label: "Parameters" },
  { value: "grants", label: "Grants" },
  { value: "health", label: "Health" },
  { value: "usedBy", label: "Used by" },
];

/**
 * registrationTabOf returns the tab that the tab query parameter names. Of
 * an array, the first item counts. A value that REGISTRATION_TABS does not
 * list, or no value, is about.
 */
export function registrationTabOf(
  value: string | string[] | undefined,
): RegistrationTab {
  const first = Array.isArray(value) ? value[0] : value;
  return REGISTRATION_TABS.find((t) => t.value === first)?.value ?? "about";
}
