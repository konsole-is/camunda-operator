// This module runs in the browser and on the server.

/** listOf joins items for a sentence: "a and b", or "a, b, and c" for three or more. */
export function listOf(items: readonly string[]): string {
  if (items.length <= 2) return items.join(" and ");
  return `${items.slice(0, -1).join(", ")}, and ${items.at(-1)}`;
}
