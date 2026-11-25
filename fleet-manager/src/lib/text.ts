// This module runs in the browser and on the server.

/** messageOf is the message of error, or error as text when it is not an Error. */
export function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/** listOf joins items for a sentence: "a and b", or "a, b, and c" for three or more. */
export function listOf(items: readonly string[]): string {
  if (items.length <= 2) return items.join(" and ");
  return `${items.slice(0, -1).join(", ")}, and ${items.at(-1)}`;
}

/**
 * breakParts splits text where a line can break before it breaks inside a
 * word: after a / or a . that is not the last character, and between the
 * words of a CamelCase name. The parts joined are text.
 */
export function breakParts(text: string): string[] {
  return text.split(/(?<=[/.])(?=.)|(?<=[a-z])(?=[A-Z])/);
}
