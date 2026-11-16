/**
 * NotRead names kinds that the page could not read or whose list is out of
 * date, in a box that then shows nothing from them.
 */
export function NotRead({ kinds }: { kinds: readonly string[] }) {
  return (
    <p className="text-sm text-muted">
      Not known: the page has no current list of {kinds.join(", ")}.
    </p>
  );
}
