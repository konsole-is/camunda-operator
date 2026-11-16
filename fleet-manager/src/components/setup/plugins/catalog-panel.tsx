"use client";

import { type ReactNode, useState } from "react";
import { Panel } from "@/components/cluster/claim-tab";
import { Button } from "@/components/ui/button";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { CatalogChart } from "@/startup/bundled-data";

/**
 * CatalogPanel lists charts that are not installed, with the sections they
 * bring and the command that installs each. It renders nothing for no
 * chart. A section that sections does not hold shows by its name.
 */
export function CatalogPanel({
  charts,
  sections,
}: {
  charts: readonly CatalogChart[];
  sections: readonly PluginSection[];
}): ReactNode {
  if (charts.length === 0) return null;
  const labelOf = (name: string) =>
    sections.find((s) => s.metadata.name === name)?.spec.label ?? name;
  return (
    <Panel label="Charts you can install">
      <ul className="flex flex-col divide-y divide-border">
        {charts.map((chart) => (
          <li
            key={chart.name}
            className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0"
          >
            <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
              <h3 className="font-mono text-sm font-semibold">{chart.name}</h3>
              <ul
                aria-label={`Sections of ${chart.name}`}
                className="flex flex-wrap gap-1.5"
              >
                {chart.sections.map((section) => (
                  <li
                    key={section}
                    className="rounded-full bg-panel-2 px-2 py-0.5 text-xs text-muted"
                  >
                    {labelOf(section)}
                  </li>
                ))}
              </ul>
            </div>
            <p className="text-sm text-muted">{chart.description}</p>
            <InstallCommand command={chart.installCommand} />
          </li>
        ))}
      </ul>
    </Panel>
  );
}

function InstallCommand({ command }: { command: string }) {
  const [copy, setCopy] = useState<"idle" | "copied" | "failed">("idle");
  const onCopy = () => {
    // navigator.clipboard is undefined outside a secure context, so the
    // call runs inside the promise and its error rejects it.
    Promise.resolve()
      .then(() => navigator.clipboard.writeText(command))
      .then(
        () => setCopy("copied"),
        () => setCopy("failed"),
      );
  };
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex flex-col items-start gap-2 sm:flex-row">
        <pre className="w-full min-w-0 rounded-md sm:flex-1 bg-panel-2 px-3 py-2 whitespace-pre-wrap [overflow-wrap:anywhere]">
          <code className="font-mono text-xs text-text">{command}</code>
        </pre>
        <Button variant="outline" size="sm" onClick={onCopy}>
          {copy === "copied" ? "Copied" : "Copy the command"}
        </Button>
      </div>
      {copy === "failed" && (
        <p role="alert" className="text-xs text-bad">
          Select the command and copy it.
        </p>
      )}
    </div>
  );
}
