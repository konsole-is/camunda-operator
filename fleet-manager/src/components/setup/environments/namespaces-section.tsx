"use client";

import { useState } from "react";
import { ChangeReview } from "@/components/fleet/change-review";
import { Button } from "@/components/ui/button";
import type { Change, WriteMode, WriteResult } from "@/lib/write/types";
import { namespaceLabelChange } from "./draft";
import type { EnvironmentActions } from "./environment-editor";
import { SelectField } from "./form-parts";
import { labelledNamespaces, namespacesToAdd } from "./rows";
import type { EnvironmentSources } from "./sources";

/**
 * NamespacesSection lists the namespaces of environment with their claims,
 * and adds the label of environment to a namespace that has no environment
 * label. The label goes to the cluster also in Git mode. A new environment
 * offers no namespace yet.
 */
export function NamespacesSection({
  environment,
  isNew,
  sources,
  mode,
  actions,
  show,
}: {
  environment: string;
  isNew: boolean;
  sources: EnvironmentSources;
  mode: WriteMode;
  actions: Pick<EnvironmentActions, "previewLabel" | "applyLabel">;
  show: (result: WriteResult, message: string) => void;
}) {
  const [picked, setPicked] = useState("");
  const [change, setChange] = useState<Change>();
  const members = labelledNamespaces(environment, sources.namespaces);
  const offered = namespacesToAdd(sources.namespaces);
  // A namespace can get an environment label after the admin picked it.
  const selected = offered.includes(picked) ? picked : "";
  const count = (
    claims: readonly { metadata: { namespace?: string } }[],
    ns: string,
  ) => claims.filter((c) => c.metadata.namespace === ns).length;

  function review() {
    const namespace = sources.namespaces.find(
      (n) => n.metadata?.name === selected,
    );
    if (namespace) setChange(namespaceLabelChange(namespace, environment));
  }

  return (
    <div className="flex flex-col gap-4">
      {members.length === 0 ? (
        <p className="text-sm text-muted">
          No namespace carries the label of this environment.
        </p>
      ) : (
        <ul
          aria-label="Namespaces of the environment"
          className="flex flex-col divide-y divide-divider rounded-md border border-border"
        >
          {members.map((ns) => (
            <li
              key={ns}
              className="flex flex-wrap items-baseline justify-between gap-2 px-3 py-2 text-sm"
            >
              <span className="font-mono">{ns}</span>
              <span className="text-muted">
                {claimsText(
                  count(sources.clusterClaims, ns),
                  count(sources.planeClaims, ns),
                )}
              </span>
            </li>
          ))}
        </ul>
      )}
      {isNew ? (
        <p className="text-sm text-muted">
          Save the environment, then add namespaces.
        </p>
      ) : change ? (
        <ChangeReview
          change={change}
          mode={mode}
          diffLabel="Changes to the namespace"
          effects={[
            `New claims in ${picked} follow the policy of ${environment}.`,
            `Claims that exist in ${picked} keep the policy they recorded.`,
            ...(mode === "git"
              ? [
                  "The label goes straight to the cluster, also in Git mode. It is not in the Git repository.",
                ]
              : []),
          ]}
          confirmLabel="Add the namespace"
          gitConfirmLabel="Add the namespace"
          savedMessage={`The namespace ${picked} is in the environment ${environment}.`}
          preview={actions.previewLabel}
          apply={actions.applyLabel}
          show={show}
          onWritten={() => {
            setChange(undefined);
            setPicked("");
          }}
          onCancel={() => setChange(undefined)}
        />
      ) : offered.length === 0 ? (
        <p className="text-sm text-muted">
          Each namespace already carries an environment label.
        </p>
      ) : (
        <div className="flex flex-wrap items-end gap-2">
          <SelectField
            label="Add a namespace"
            value={selected}
            options={[
              ["", "Pick a namespace"],
              ...offered.map((n) => [n, n] as const),
            ]}
            onChange={setPicked}
          />
          <Button variant="outline" disabled={selected === ""} onClick={review}>
            Add
          </Button>
        </div>
      )}
    </div>
  );
}

function claimsText(clusters: number, planes: number): string {
  const parts = [
    clusters === 1 ? "1 cluster" : `${clusters} clusters`,
    ...(planes > 0
      ? [planes === 1 ? "1 management plane" : `${planes} management planes`]
      : []),
  ];
  return parts.join(", ");
}
