/** unreadText says that a tab can be incomplete because the page could not read kinds. */
export function unreadText(kinds: readonly string[]): string {
  return `The page could not read ${kinds.join(", ")}, so this tab can be incomplete or out of date.`;
}

/** UnreadNote shows unreadText of kinds. Without kinds it renders nothing. */
export function UnreadNote({ kinds }: { kinds: readonly string[] }) {
  if (kinds.length === 0) return null;
  return <p className="text-sm text-muted">{unreadText(kinds)}</p>;
}
