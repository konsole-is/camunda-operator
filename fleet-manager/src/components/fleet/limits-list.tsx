import type { ReactNode } from "react";

/**
 * LimitsList shows each limit of a registration, read only, as "<label>
 * (limit set at install)", its value, and its description. It renders
 * nothing for no limit.
 */
export function LimitsList({
  limits,
}: {
  limits: readonly { label: string; value: string; description?: string }[];
}): ReactNode {
  if (limits.length === 0) return null;
  return (
    <dl className="flex flex-col gap-2 text-sm">
      {limits.map((limit) => (
        <div key={limit.label} className="flex min-w-0 flex-col gap-0.5">
          <dt className="text-muted">{`${limit.label} (limit set at install)`}</dt>
          <dd className="font-mono break-all text-text">{limit.value}</dd>
          {limit.description && (
            <dd className="text-xs text-muted">{limit.description}</dd>
          )}
        </div>
      ))}
    </dl>
  );
}
