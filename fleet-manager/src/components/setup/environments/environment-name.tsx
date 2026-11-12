import { isHexColor } from "./draft";

/**
 * EnvironmentName shows the swatch of color, when it is a hex color, the
 * display name, and the name in mono when the two differ.
 */
export function EnvironmentName({
  name,
  displayName,
  color,
}: {
  name: string;
  displayName?: string;
  color?: string;
}) {
  const title = displayName || name;
  return (
    <span className="flex items-center gap-2.5">
      <Swatch color={color} />
      <span className="flex min-w-0 flex-col">
        <span className="font-medium">{title}</span>
        {title !== name && (
          <span className="font-mono text-xs text-muted">{name}</span>
        )}
      </span>
    </span>
  );
}

/** Swatch is a dot of color, or an empty ring when color is not a hex color. */
export function Swatch({ color }: { color?: string }) {
  const valid = color !== undefined && isHexColor(color);
  return (
    <span
      aria-hidden
      data-color={valid ? color : undefined}
      className="size-3 shrink-0 rounded-full border border-border"
      style={valid ? { backgroundColor: color, borderColor: color } : undefined}
    />
  );
}
