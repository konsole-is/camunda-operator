import type { ReactNode } from "react";
import { Panel, Term, Terms } from "@/components/cluster/claim-tab";
import type { PolicyResult } from "@/lib/policy/types";
import type { ParameterRow } from "./parameters";
import { ValueError } from "./tab-parts";

/**
 * ParametersTab shows each parameter of rows in its own panel: its type,
 * its range or values, its default, whether a claim can change it, and its
 * value in each Environment. While Environment is in unread, it says that
 * the values of the environments are not known.
 */
export function ParametersTab({
  rows,
  unread,
}: {
  rows: PolicyResult<ParameterRow[]>;
  unread: readonly string[];
}): ReactNode {
  if (!("value" in rows))
    return (
      <ValueError text={"error" in rows ? rows.error : rows.refusal.message} />
    );
  if (rows.value.length === 0)
    return (
      <p className="text-sm text-muted">This option takes no parameters.</p>
    );
  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-text-2">
        Default and A claim can change it apply in a namespace without an
        environment. Each environment shows what a new claim in it gets. A claim
        that exists keeps the values that it resolved.
      </p>
      {unread.includes("Environment") && (
        <p className="text-sm text-muted">
          Values of the environments: Not known: Environment is not read.
        </p>
      )}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        {rows.value.map((row) => (
          <Panel key={row.name} label={row.name}>
            {row.error ? (
              <ValueError text={row.error} />
            ) : (
              <>
                {row.description && (
                  <p className="text-sm text-text-2">{row.description}</p>
                )}
                {row.environmentOnly && (
                  <p className="text-sm text-text-2">
                    Only an environment sets this parameter.
                  </p>
                )}
                <Terms>
                  <Term label="Type">{row.type ?? "Any"}</Term>
                  {rangeOf(row) && <Term label="Range">{rangeOf(row)}</Term>}
                  {row.values && (
                    <Term label="Values">{row.values.join(", ")}</Term>
                  )}
                  <Term label="Default">
                    <Value value={row.fallback} none="None" />
                  </Term>
                  <Term label="A claim can change it">
                    {row.claimEditable ? "Yes" : "No"}
                  </Term>
                  {row.environments?.map((e) => (
                    <Term
                      key={e.name}
                      label={e.environment}
                      note={`A claim can change it: ${e.claimEditable ? "Yes" : "No"}`}
                    >
                      <Value value={e.value} none="No value" />
                    </Term>
                  ))}
                </Terms>
              </>
            )}
          </Panel>
        ))}
      </div>
    </div>
  );
}

function rangeOf({ minimum, maximum }: ParameterRow): string | undefined {
  if (minimum !== undefined && maximum !== undefined)
    return `${minimum} to ${maximum}`;
  if (minimum !== undefined) return `At least ${minimum}`;
  if (maximum !== undefined) return `At most ${maximum}`;
  return undefined;
}

function Value({ value, none }: { value: unknown; none: string }) {
  if (value === undefined) return <span className="text-muted">{none}</span>;
  return <span className="font-mono break-all">{JSON.stringify(value)}</span>;
}
