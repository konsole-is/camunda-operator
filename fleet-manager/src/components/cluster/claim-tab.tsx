"use client";

import { Lock } from "lucide-react";
import { type ReactNode, useState } from "react";
import { ConditionsTable } from "@/components/fleet/conditions-table";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  type ClusterField,
  lockLabel,
  stateLabel,
  stateNote,
} from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import {
  type ClaimRef,
  type ClaimSection,
  type ClaimView,
  storedYaml,
} from "./claim";

const PANEL =
  "flex flex-col gap-3 rounded-lg border border-border bg-panel p-4";
const HEADING = "font-heading text-base font-semibold";
const NOTE = "block text-xs text-muted";

/**
 * ClaimTab shows the claim of a cluster read only. view is claimView of
 * stored, and stored is the claim as the API server returns it, for Show
 * YAML. now is the time that the condition ages count to.
 */
export function ClaimTab({
  view,
  stored,
  now,
}: {
  view: ClaimView;
  stored: Resource<"CamundaClusterClaim">;
  now: string;
}) {
  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 lg:grid-cols-2">
        <Panel label="Basics">
          <Terms>
            <Term label="Name" note="Set at creation">
              {view.name}
            </Term>
            <Term label="Namespace" note="Set at creation">
              {view.namespace}
            </Term>
            <Term label="Environment" note="Set at creation">
              {view.environment ?? (
                <span className="text-muted">No environment</span>
              )}
            </Term>
          </Terms>
        </Panel>
        <Panel label="Release and preset">
          <Terms>
            <RefTerm label="Release" value={view.release} />
            <Term label="Preset">{view.preset ?? "-"}</Term>
            <RefTerm label="Platform config" value={view.platformConfig} />
          </Terms>
        </Panel>
      </div>
      {view.sections.map((s) => (
        <SectionPanel key={`${s.section}/${s.registration}`} section={s} />
      ))}
      <div className="grid gap-4 lg:grid-cols-2">
        <Panel label="Attachments">
          <Terms>
            <Term label="Optimize">
              {view.optimize ? (
                <>
                  {view.optimize.externalUrl ?? "Attached"}
                  <span className={NOTE}>
                    Auth config {view.optimize.managementAuthConfigRef}
                  </span>
                </>
              ) : (
                <span className="text-muted">Not attached</span>
              )}
            </Term>
            <Term label="Backup schedule">
              {view.backupSchedule ? (
                <>
                  <code className="rounded bg-panel-2 px-1 text-xs">
                    {view.backupSchedule.schedule}
                  </code>
                  {retained(view.backupSchedule) && (
                    <span className={NOTE}>
                      {retained(view.backupSchedule)}
                    </span>
                  )}
                </>
              ) : (
                <span className="text-muted">Not attached</span>
              )}
            </Term>
          </Terms>
        </Panel>
        <Panel label="Deletion policy">
          <Terms>
            <RefTerm
              label="When the claim is deleted"
              value={view.deletionPolicy}
            />
          </Terms>
        </Panel>
      </div>
      <Panel label="Conditions">
        <ConditionsTable
          conditions={view.conditions}
          now={now}
          label="Claim conditions"
        />
      </Panel>
      <Panel label="Requests">
        <RequestsTable view={view} />
      </Panel>
      <StoredYaml claim={stored} />
    </div>
  );
}

function Panel({ label, children }: { label: string; children: ReactNode }) {
  return (
    <section aria-label={label} className={PANEL}>
      <h2 className={HEADING}>{label}</h2>
      {children}
    </section>
  );
}

function Terms({ children }: { children: ReactNode }) {
  return (
    <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-2 text-sm">
      {children}
    </dl>
  );
}

function Term({
  label,
  note,
  children,
}: {
  label: string;
  note?: string;
  children: ReactNode;
}) {
  return (
    <>
      <dt className="text-muted">{label}</dt>
      <dd className="min-w-0 break-words">
        {children}
        {note && <span className={NOTE}>{note}</span>}
      </dd>
    </>
  );
}

function RefTerm({ label, value }: { label: string; value: ClaimRef }) {
  return (
    <Term
      label={label}
      note={value.fromEnvironment ? "Default of the environment" : undefined}
    >
      {value.value ?? <span className="text-muted">Not set</span>}
    </Term>
  );
}

function retained(schedule: { completed?: number; failed?: number }) {
  const parts = [
    schedule.completed !== undefined && `${schedule.completed} completed`,
    schedule.failed !== undefined && `${schedule.failed} failed`,
  ].filter(Boolean);
  return parts.length > 0 ? `Keeps ${parts.join(" and ")}` : undefined;
}

function SectionPanel({ section }: { section: ClaimSection }) {
  return (
    <Panel label={section.sectionLabel}>
      <p className="text-sm">
        <span className="font-medium">{section.optionLabel}</span>
        <span className={NOTE}>Chosen at creation</span>
      </p>
      {section.parameters.length === 0 ? (
        <p className="text-sm text-muted">No parameters.</p>
      ) : (
        <Table
          label={`${section.sectionLabel} parameters table`}
          aria-label={`${section.sectionLabel} parameters`}
        >
          <TableHeader>
            <TableRow>
              <TableHead>Parameter</TableHead>
              <TableHead>Value</TableHead>
              <TableHead>State</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {section.parameters.map((field) => (
              <TableRow key={field.key}>
                <th
                  scope="row"
                  className="px-2 py-2 text-left align-top font-normal"
                >
                  <code className="text-text">{field.label}</code>
                </th>
                <TableCell className="max-w-80 align-top break-words whitespace-normal">
                  <Value value={field.value} />
                </TableCell>
                <TableCell className="align-top whitespace-normal">
                  <State field={field} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </Panel>
  );
}

function Value({ value }: { value: unknown }) {
  if (value === undefined) return <span className="text-muted">Not set</span>;
  if (typeof value === "string") return <>{value}</>;
  if (typeof value === "number" || typeof value === "boolean")
    return <>{String(value)}</>;
  return (
    <code className="text-xs text-text-2 [overflow-wrap:anywhere]">
      {JSON.stringify(value)}
    </code>
  );
}

function State({ field }: { field: ClusterField }) {
  const note = stateNote(field.state);
  return (
    <div className="flex flex-col gap-1">
      <span className="text-text-2">{stateLabel(field.state)}</span>
      {note && <span className="text-xs text-muted">{note}</span>}
      {field.lock && (
        <span className="flex items-center gap-1.5 text-xs text-muted">
          <Lock aria-hidden className="size-3.5 shrink-0" />
          {lockLabel(field.lock)}
        </span>
      )}
    </div>
  );
}

function RequestsTable({ view }: { view: ClaimView }) {
  if (view.requests.length === 0) {
    return <p className="text-sm text-muted">No requests</p>;
  }
  return (
    <Table label="Requests table" aria-label="Requests">
      <TableHeader>
        <TableRow>
          <TableHead>Request</TableHead>
          <TableHead>Section</TableHead>
          <TableHead>Option</TableHead>
          <TableHead>Outputs ready</TableHead>
          <TableHead>Ready</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {view.requests.map((r) => (
          <TableRow key={r.name}>
            <th
              scope="row"
              className="px-2 py-2 text-left font-medium whitespace-nowrap text-text"
            >
              {r.name}
            </th>
            <TableCell className="text-text-2">{r.sectionLabel}</TableCell>
            <TableCell className="text-text-2">{r.optionLabel}</TableCell>
            <TableCell className="text-text-2">
              {r.outputsReady ?? "-"}
            </TableCell>
            <TableCell className="text-text-2">{r.ready ?? "-"}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

function StoredYaml({ claim }: { claim: Resource<"CamundaClusterClaim"> }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="flex flex-col items-start gap-2">
      <Button
        variant="outline"
        size="sm"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        {open ? "Hide YAML" : "Show YAML"}
      </Button>
      {open && (
        <pre
          role="region"
          aria-label="Claim YAML"
          tabIndex={0}
          className="w-full overflow-x-auto rounded-md border border-border bg-panel-2 p-3 text-xs text-text outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {storedYaml(claim)}
        </pre>
      )}
    </div>
  );
}
