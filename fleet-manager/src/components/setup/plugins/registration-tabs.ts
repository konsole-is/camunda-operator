import { tabOf } from "@/lib/tabs";

/** RegistrationTab names one tab of the registration page. */
export type RegistrationTab =
  "about" | "contract" | "parameters" | "grants" | "health" | "used-by";

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
  { value: "used-by", label: "Used by" },
];

/**
 * registrationTabOf returns the tab that the tab query parameter names, as
 * tabOf does. The fallback is about.
 */
export function registrationTabOf(
  value: string | string[] | undefined,
): RegistrationTab {
  return tabOf(REGISTRATION_TABS, value, "about");
}
