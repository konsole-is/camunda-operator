import type { ReactNode } from "react";
import type { SecretView } from "./facts";

/**
 * SecretRefView shows one Secret reference: the Secret, with its namespace
 * when it has one, and each key that the contract reads. It shows names
 * and keys only, never a value.
 */
export function SecretRefView({ secret }: { secret: SecretView }): ReactNode {
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <span className="font-mono text-sm [overflow-wrap:anywhere]">
        {secret.namespace ? `${secret.namespace}/${secret.name}` : secret.name}
      </span>
      <dl className="grid grid-cols-[max-content_1fr] gap-x-3 gap-y-0.5 text-xs">
        {secret.keys.map(([label, key]) => (
          <div key={label} className="contents">
            <dt className="text-muted">{label}</dt>
            <dd className="font-mono text-text-2 [overflow-wrap:anywhere]">
              {key}
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
