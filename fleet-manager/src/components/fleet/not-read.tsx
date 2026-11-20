/**
 * notReadText says that the fleet manager has no current list of kinds: a
 * read failed, or the list is out of date. It starts with "Not known", for
 * a box or a value that then shows nothing from those lists. With tab, it
 * says instead that the tab can be incomplete or out of date, for a tab that
 * still shows what it has.
 */
export function notReadText(kinds: readonly string[], tab = false): string {
  const names = kinds.join(", ");
  return tab
    ? `The fleet manager has no current list of ${names}. This tab can be incomplete or out of date.`
    : `Not known: the fleet manager has no current list of ${names}.`;
}

/** NotRead shows notReadText of kinds and tab. Without kinds it renders nothing. */
export function NotRead({
  kinds,
  tab = false,
}: {
  kinds: readonly string[];
  tab?: boolean;
}) {
  if (kinds.length === 0) return null;
  return <p className="text-sm text-muted">{notReadText(kinds, tab)}</p>;
}
