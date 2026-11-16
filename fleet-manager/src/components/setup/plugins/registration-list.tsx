"use client";

import { type ReactNode, useEffect, useState } from "react";
import { Panel } from "@/components/cluster/claim-tab";
import { age } from "@/components/fleet/age";
import { DataTable } from "@/components/fleet/data-table";
import type { ReadError } from "@/components/planes/planes-table";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { CatalogChart } from "@/startup/bundled-data";
import { type CatalogRead, missingCharts } from "./catalog";
import { CatalogPanel } from "./catalog-panel";
import { ConditionBadge } from "./condition-badge";
import {
  type RegistrationGroup,
  type RegistrationRow,
  type SectionHeader,
  fillsText,
  registrationGroups,
  registrationSummary,
  unreadKinds,
} from "./groups";
import { type RegistrationSources, useRegistrationSources } from "./sources";

const MUTED = "block text-xs text-muted";
const NOW_INTERVAL_MS = 30_000;

/**
 * RegistrationList is the Plugins page: the registrations in the groups of
 * their sections, the summary line, and the panel of the charts. While
 * PluginRegistration is read, charts are the charts that are not installed.
 * While it is not read, charts must not be empty when the catalog has a
 * chart. catalogError is the error of the read of the catalog. Each of
 * errors shows above the groups, and the groups stay. stale holds the kinds
 * whose watch has no snapshot on its current connection, as
 * useRegistrationSources gives them. While PluginRegistration is not read,
 * the page shows no health count and no chart as not installed. now is the
 * time that "Healthy since" counts to.
 */
export function RegistrationList({
  groups,
  summary,
  charts,
  catalogError,
  sections,
  errors,
  stale = [],
  now,
}: {
  groups: RegistrationGroup[];
  summary: { healthy: number; notHealthy: number };
  charts: readonly CatalogChart[];
  catalogError?: string;
  sections: readonly PluginSection[];
  errors: ReadError[];
  stale?: readonly string[];
  now: string;
}): ReactNode {
  const unread = unreadKinds({ errors, stale });
  const registrationsUnread = unread.includes("PluginRegistration");
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
          Plugins
        </h1>
        <p className="text-sm text-muted">
          {registrationsUnread
            ? "Health not known"
            : `${summary.healthy} healthy, ${summary.notHealthy} not healthy`}
        </p>
      </div>
      {errors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      {unread.length > 0 && (
        <p className="text-sm text-muted">
          {`Not read: ${unread.join(", ")}. What needs them shows as not known.`}
        </p>
      )}
      {groups.map((group) => (
        <Group key={group.section?.name ?? ""} group={group} now={now} />
      ))}
      <CatalogPanel
        charts={charts}
        sections={sections}
        error={catalogError}
        installedUnknown={registrationsUnread}
      />
    </div>
  );
}

/**
 * RegistrationListLive is RegistrationList on the live sources, starting
 * from initial. catalog is the whole catalog: the panel offers the charts
 * that no registration names as its controller. now is the time of the
 * render.
 */
export function RegistrationListLive({
  initial,
  catalog,
  now,
}: {
  initial: RegistrationSources;
  catalog: CatalogRead;
  now: string;
}): ReactNode {
  const sources = useRegistrationSources(initial);
  const clock = useClock(now);
  const registrationsUnread =
    unreadKinds(sources).includes("PluginRegistration");
  return (
    <RegistrationList
      groups={registrationGroups(sources)}
      summary={registrationSummary(sources.registrations)}
      charts={
        !("charts" in catalog)
          ? []
          : registrationsUnread
            ? catalog.charts
            : missingCharts(catalog.charts, sources.registrations)
      }
      catalogError={"error" in catalog ? catalog.error : undefined}
      sections={sources.sections}
      errors={sources.errors}
      stale={sources.stale}
      now={clock}
    />
  );
}

// useClock starts at the time of the server, so that the first render in
// the browser matches the render of the server.
function useClock(initial: string): string {
  const [clock, setClock] = useState(initial);
  useEffect(() => {
    const timer = setInterval(
      () => setClock(new Date().toISOString()),
      NOW_INTERVAL_MS,
    );
    return () => clearInterval(timer);
  }, []);
  return clock;
}

function Group({ group, now }: { group: RegistrationGroup; now: string }) {
  const label =
    group.section?.label ??
    (group.unread ? "Section not known" : "Not in a section");
  return (
    <Panel label={label}>
      {group.section && <SectionFacts section={group.section} />}
      {group.rows.length === 0 ? (
        <p className="text-sm text-muted">
          {group.unread
            ? "Options not known"
            : "No option is installed for this section."}
        </p>
      ) : (
        <DataTable
          label={`Options of ${label}`}
          toolbar={false}
          columns={COLUMNS}
          rows={group.rows.map((row) => ({
            id: row.name,
            label: row.label,
            href: `/setup/plugins/${encodeURIComponent(row.name)}`,
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
    accepted: (
      <ConditionBadge
        type="Accepted"
        condition={row.accepted}
        unknown={row.healthUnknown}
      />
    ),
    ready: (
      <>
        <ConditionBadge
          type="Ready"
          condition={row.ready}
          unknown={row.healthUnknown}
        />
        {row.healthySince && (
          <time dateTime={row.healthySince} className={`${MUTED} mt-1`}>
            {`Healthy since ${age(row.healthySince, now)} ago`}
          </time>
        )}
      </>
    ),
    requests: row.requests ? (
      <>
        <span>{row.requests.total}</span>
        {row.requests.byEnvironment
          ? row.requests.byEnvironment.map((e) => (
              <span key={e.environment} className={MUTED}>
                {`${e.environment} ${e.count}`}
              </span>
            ))
          : row.requests.total > 0 && (
              <span className={MUTED}>Environments not known</span>
            )}
      </>
    ) : (
      "Not known"
    ),
    operatorVersions: row.operatorVersions ?? "Any",
    orphan: row.orphan ? "Supported" : "Not supported",
  };
}
