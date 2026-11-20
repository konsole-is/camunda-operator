import type { ReactNode } from "react";
import type { Fact } from "./rows";

/** FactsList shows the key facts of a preset as label and value pairs. It shows nothing without a fact. */
export function FactsList({ facts }: { facts: readonly Fact[] }): ReactNode {
  if (facts.length === 0) return null;
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
      {facts.map((fact) => (
        <div key={fact.label} className="contents">
          <dt className="text-muted">{fact.label}</dt>
          <dd className="font-mono tabular-nums text-text">{fact.value}</dd>
        </div>
      ))}
    </dl>
  );
}
