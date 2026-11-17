/**
 * tabOf returns the tab of tabs that value, a tab query parameter, names.
 * Of an array, the first item counts. A value that tabs does not list, or
 * no value, gives fallback.
 */
export function tabOf<T extends string>(
  tabs: readonly { value: T }[],
  value: string | string[] | undefined,
  fallback: T,
): T {
  const first = Array.isArray(value) ? value[0] : value;
  return tabs.find((t) => t.value === first)?.value ?? fallback;
}
