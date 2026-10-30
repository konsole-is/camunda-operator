"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import { useEffect, useRef } from "react";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import { EnvironmentCards } from "@/components/create/environment-cards";
import type { DraftPath } from "@/components/create/use-create-flow";
import { DataTable } from "@/components/fleet/data-table";
import {
  KeyValueEditor,
  keyValueErrors,
} from "@/components/fleet/key-value-editor";
import { type StatusTone, TEXT_TONE } from "@/components/fleet/status";
import { type Plane, servesEnvironment } from "@/components/planes/plane";
import {
  type PlaneContext,
  type PlaneDraft,
  planeClaimOf,
  platformConfigOf,
  servedOf,
} from "@/lib/planes/draft";
import { planeIssuer } from "@/lib/planes/identity";
import {
  type PreviewResult,
  type PreviewRow,
  servedPreview,
} from "@/lib/planes/preview";
import type { ValidationError } from "@/lib/schema";
import { environmentCounts } from "@/lib/usage";
import { cn } from "@/lib/utils";

const RESULT_TONE: Record<PreviewResult, StatusTone> = {
  Attach: "ok",
  NotReady: "warn",
  ClaimedElsewhere: "bad",
  InvalidReference: "bad",
};

/**
 * ServedStep is step 2 of the plane create flow: the environments that the
 * plane serves, the cluster labels that narrow them, and the expected
 * result of each cluster. Each environment card names the planes that
 * serve it now. onSelectorErrors gets the errors of the cluster labels
 * when the step mounts and after each edit.
 */
export function ServedStep({
  draft,
  set,
  ctx,
  namespaces,
  clusters,
  planes,
  onSelectorErrors,
}: {
  draft: PlaneDraft;
  set: (path: DraftPath, value: unknown) => void;
  ctx: PlaneContext;
  namespaces: readonly V1Namespace[];
  clusters: readonly CamundaCluster[];
  planes: readonly Plane[];
  onSelectorErrors: (errors: ValidationError[]) => void;
}) {
  // The editor restores only the rows of the draft, so errors of rows that
  // the draft dropped would block the step with nothing left to fix.
  const restored = useRef(draft.clusterLabels);
  const report = useRef(onSelectorErrors);
  useEffect(() => {
    report.current(
      keyValueErrors(
        Object.entries(restored.current).map(([key, value]) => ({
          key,
          value,
        })),
        "label",
      ),
    );
  }, []);

  const served = servedOf(draft);
  const config = ctx.platformConfigs.find(
    (c) => c.metadata.name === platformConfigOf(draft, ctx),
  );
  const issuer = planeIssuer(draft.mode, draft.externalKeycloak, config);
  const rows =
    issuer.kind === "unknown"
      ? undefined
      : servedPreview({
          plane: { namespace: draft.namespace?.name ?? "", name: draft.name },
          environments: served,
          clusterSelector: planeClaimOf(draft, ctx).spec.clusterSelector,
          issuer,
          clusters,
          namespaces,
          platformConfigs: ctx.platformConfigs,
        });

  return (
    <div className="flex max-w-3xl min-w-0 flex-col gap-8">
      <EnvironmentCards
        multiple
        environments={ctx.environments}
        counts={environmentCounts(ctx.environments, namespaces, clusters)}
        value={served}
        onChange={(names) => set(["served"], names)}
        extra={(environment) => {
          const servers = planes.filter((p) =>
            servesEnvironment(p, environment.metadata.name ?? ""),
          );
          return servers.length === 0 ? (
            <span className="text-xs text-muted">No plane serves it yet.</span>
          ) : (
            <ul className="text-xs text-text-2">
              {servers.map((p) => (
                <li key={p.id}>{`Served by ${p.namespace}/${p.name}`}</li>
              ))}
            </ul>
          );
        }}
      />
      <div className="flex max-w-xl flex-col gap-1">
        <KeyValueEditor
          label="Cluster labels"
          format="label"
          defaultValue={draft.clusterLabels}
          onChange={(labels, errors) => {
            set(["clusterLabels"], labels);
            onSelectorErrors(errors);
          }}
        />
        <p className="text-sm text-muted">
          Leave empty to serve every cluster in these environments.
        </p>
      </div>
      {rows && <PreviewTable rows={rows} />}
    </div>
  );
}

/** PreviewTable shows the expected result of each cluster that a new plane serves. */
export function PreviewTable({ rows }: { rows: PreviewRow[] }) {
  return (
    <section className="flex min-w-0 flex-col gap-2">
      <h2 className="font-heading text-base font-semibold">
        Expected clusters
      </h2>
      <p className="text-sm text-muted">
        Expected results. The operator decides when the plane runs, and the
        plane then reports the result for each cluster.
      </p>
      <DataTable
        label="Expected clusters"
        emptyText={rows.length === 0 ? "No cluster matches yet." : undefined}
        columns={[
          { key: "cluster", label: "Cluster" },
          { key: "result", label: "Result" },
          { key: "why", label: "Why" },
        ]}
        rows={rows.map((row) => ({
          id: `${row.namespace}/${row.name}`,
          search: `${row.namespace}/${row.name} ${row.label}`,
          cells: {
            cluster: `${row.namespace}/${row.name}`,
            result: (
              <span
                className={cn(
                  "text-sm font-medium",
                  TEXT_TONE[RESULT_TONE[row.result]],
                )}
              >
                {row.label}
              </span>
            ),
            why: row.message ?? "—",
          },
        }))}
      />
    </section>
  );
}
