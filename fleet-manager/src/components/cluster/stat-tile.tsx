/**
 * statValueClass is the style of the value of a stat: large for a number,
 * small and muted for words such as Not available.
 */
export function statValueClass(value: string): string {
  return /^\d/.test(value)
    ? "font-heading text-lg font-semibold text-text tabular-nums"
    : "text-sm text-muted";
}

/**
 * StatTile is one stat in a dl: its label, its value, and a note under the
 * value.
 */
export function StatTile({
  label,
  value,
  note,
}: {
  label: string;
  value: string;
  note?: string;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-0.5 rounded-md border border-border bg-panel-2 p-3">
      <dt className="text-xs text-muted">{label}</dt>
      <dd className={statValueClass(value)}>{value}</dd>
      {note && <dd className="text-xs break-words text-muted">{note}</dd>}
    </div>
  );
}
