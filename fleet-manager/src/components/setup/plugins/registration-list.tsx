"use client";

import type { ReactNode } from "react";
import { Panel } from "@/components/cluster/claim-tab";
import { age } from "@/components/fleet/age";
import { DataTable } from "@/components/fleet/data-table";
import type { ReadError } from "@/components/planes/planes-table";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { SlotField } from "@/lib/fields/slots";
import type { CatalogChart } from "@/startup/bundled-data";
import { missingCharts } from "./catalog";
import { CatalogPanel } from "./catalog-panel";
import { ConditionBadge } from "./condition-badge";
import {
  type RegistrationGroup,
  type RegistrationRow,
  type SectionHeader,
  registrationGroups,
  registrationSummary,
} from "./groups";
import { type RegistrationSources, useRegistrationSources } from "./sources";

const MUTED = "block text-xs text-muted";

/**
 * RegistrationList is the Plugins page: the registrations in the groups of
 * their sections, the summary line, and the panel of the charts. charts are
 * the charts that the panel offers. Each of errors shows above the groups,
 * and the groups stay. now is the time that "Healthy since" counts to.
 */
export function RegistrationList({
  groups,
  summary,
  charts,
  sections,
  errors,
  now,
}: {
  groups: RegistrationGroup[];
  summary: { healthy: number; notHealthy: number };
  charts: readonly CatalogChart[];
  sections: readonly PluginSection[];
  errors: ReadError[];
  now: string;
}): ReactNode {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
          Plugins
        </h1>
        <p className="text-sm text-muted">
          {`${summary.healthy} healthy, ${summary.notHealthy} not healthy`}
        </p>
      </div>
      {errors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      {groups.map((group) => (
        <Group key={group.section?.name ?? ""} group={group} now={now} />
      ))}
      <CatalogPanel charts={charts} sections={sections} />
    </div>
  );
}

/**
 * RegistrationListLive is RegistrationList on the live sources, starting
 * from initial. charts is the whole catalog: the panel offers the charts
 * that no registration names as its controller.
 */
export function RegistrationListLive({
  initial,
  charts,
  now,
}: {
  initial: RegistrationSources;
  charts: readonly CatalogChart[];
  now: string;
}): ReactNode {
  const sources = useRegistrationSources(initial);
  return (
    <RegistrationList
      groups={registrationGroups(sources)}
      summary={registrationSummary(sources.registrations)}
      charts={missingCharts(charts, sources.registrations)}
      sections={sources.sections}
      errors={sources.errors}
      now={now}
    />
  );
}

function Group({ group, now }: { group: RegistrationGroup; now: string }) {
  const label = group.section?.label ?? "Not in a section";
  return (
    <Panel label={label}>
      {group.section && <SectionFacts section={group.section} />}
      {group.rows.length === 0 ? (
        <p className="text-sm text-muted">
          No option is installed for this section.
        </p>
      ) : (
        <DataTable
          label={`Options of ${label}`}
          toolbar={false}
          columns={COLUMNS}
          rows={group.rows.map((row) => ({
            id: row.name,
            label: row.label,
            cells: cellsOf(row, now),
          }))}
        />
      )}
    </Panel>
  );
}

const COLUMNS = [
  { key: "option", label: "Option" },
  { key: "controller", label: "Controller" },
  { key: "mode", label: "Mode" },
  { key: "accepted", label: "Accepted" },
  { key: "ready", label: "Ready" },
  { key: "requests", label: "Requests" },
  { key: "operatorVersions", label: "Operator versions" },
  { key: "orphan", label: "Orphan" },
];

function SectionFacts({ section }: { section: SectionHeader }) {
  return (
    <div className="flex flex-col gap-1 text-sm">
      {section.fields.length === 0 ? (
        <p>Fills no field</p>
      ) : (
        <ul>
          {section.fields.map((field) => (
            <li key={`${field.claimKind} ${field.kind} ${field.path}`}>
              {fillsText(field)}
            </li>
          ))}
        </ul>
      )}
      <p className="text-muted">
        {section.selection === "One" ? "Pick one" : "Pick many"}
      </p>
      <p className="text-muted">
        {section.emptyOptionLabel
          ? `Empty option: ${section.emptyOptionLabel}`
          : "No empty option"}
      </p>
    </div>
  );
}

function fillsText(field: SlotField): string {
  const plane =
    field.claimKind === "CamundaManagementClusterClaim"
      ? " for a management plane"
      : "";
  return `Fills ${field.kind} ${field.path}${plane}`;
}

function cellsOf(row: RegistrationRow, now: string): Record<string, ReactNode> {
  return {
    option: (
      <>
        <span className="font-medium">{row.label}</span>
        <span className={`${MUTED} font-mono`}>{row.name}</span>
      </>
    ),
    controller: row.controller
      ? [row.controller.name, row.controller.version].filter(Boolean).join(" ")
      : "None",
    mode: row.mode,
    accepted: <ConditionBadge type="Accepted" condition={row.accepted} />,
    ready: (
      <>
        <ConditionBadge type="Ready" condition={row.ready} />
        {row.healthySince && (
          <time dateTime={row.healthySince} className={`${MUTED} mt-1`}>
            {`Healthy since ${age(row.healthySince, now)} ago`}
          </time>
        )}
      </>
    ),
    requests: (
      <>
        <span>{row.requests.total}</span>
        {row.requests.byEnvironment.map((e) => (
          <span key={e.environment} className={MUTED}>
            {`${e.environment} ${e.count}`}
          </span>
        ))}
      </>
    ),
    operatorVersions: row.operatorVersions ?? "Any",
    orphan: row.orphan ? "Supported" : "Not supported",
  };
}
