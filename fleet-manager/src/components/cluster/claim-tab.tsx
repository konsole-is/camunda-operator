"use client";

import { Fragment, type ReactNode, useState } from "react";
import { ConditionsTable } from "@/components/fleet/conditions-table";
import { FieldState, FieldValue } from "@/components/fleet/field-state";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { lockLabel } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import {
  type ClaimRef,
  type ClaimSection,
  type ClaimView,
  storedYaml,
} from "./claim";
import type { ClaimPanel } from "./tabs";

const PANEL =
  "flex flex-col gap-3 rounded-lg border border-border bg-panel p-4";
const HEADING = "font-heading text-base font-semibold";
const NOTE = "block text-xs text-muted";

/**
 * ClaimTabEdit puts the Claim tab in edit. panels holds the edit panel that
 * shows in place of the read panel of each name. actions holds the control,
 * such as an Edit button, in the header of the read panel of each name. A
 * section panel has the name of the id of its ClaimSection.
 */
export type ClaimTabEdit = {
  panels?: Partial<Record<ClaimPanel, ReactNode>>;
  actions?: Partial<Record<ClaimPanel, ReactNode>>;
};

/**
 * ClaimTab shows the claim of a cluster. view is claimView of stored, and
 * stored is the claim as the API server returns it, for Show YAML. now is
 * the time that the condition ages count to. Without edit, the tab is read
 * only. Conditions, Requests, and the YAML have no edit panel.
 */
export function ClaimTab({
  view,
  stored,
  now,
  edit,
}: {
  view: ClaimView;
  stored: Resource<"CamundaClusterClaim">;
  now: string;
  edit?: ClaimTabEdit;
}) {
  const panel = (name: ClaimPanel, read: (action: ReactNode) => ReactNode) =>
    edit?.panels?.[name] ?? read(edit?.actions?.[name]);
  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 lg:grid-cols-2">
        {panel("basics", (action) => (
          <Panel label="Basics" action={action}>
            <Terms>
              <Term label="Name" note={lockLabel({ reason: "atCreation" })}>
                {view.name}
              </Term>
              <Term
                label="Namespace"
                note={lockLabel({ reason: "atCreation" })}
              >
                {view.namespace}
              </Term>
              <Term
                label="Environment"
                note={lockLabel({ reason: "atCreation" })}
              >
                {view.environment ?? (
                  <span className="text-muted">No environment</span>
                )}
              </Term>
              <Term label="Suspended">{view.suspend ? "Yes" : "No"}</Term>
            </Terms>
          </Panel>
        ))}
        {panel("release", (action) => (
          <Panel label="Release and preset" action={action}>
            <Terms>
              <RefTerm label="Release" value={view.release} />
              <Term label="Preset">{view.preset ?? "-"}</Term>
              <RefTerm label="Platform config" value={view.platformConfig} />
            </Terms>
          </Panel>
        ))}
      </div>
      {view.sections.map((s) => (
        <Fragment key={s.id}>
          {panel(s.id, (action) => (
            <SectionPanel section={s} action={action} />
          ))}
        </Fragment>
      ))}
      <div className="grid gap-4 lg:grid-cols-2">
        {panel("attachments", (action) => (
          <Panel label="Attachments" action={action}>
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
        ))}
        {panel("deletion", (action) => (
          <Panel label="Deletion policy" action={action}>
            <Terms>
              <RefTerm
                label="When the claim is deleted"
                value={view.deletionPolicy}
              />
            </Terms>
          </Panel>
        ))}
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

/**
 * Panel is one panel of the Claim tab. name is its accessible name, label
 * by default. action shows at the end of its header.
 */
export function Panel({
  label,
  name = label,
  action,
  children,
}: {
  label: string;
  name?: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section aria-label={name} className={PANEL}>
      <div className="flex min-h-8 items-center justify-between gap-3">
        <h2 className={HEADING}>{label}</h2>
        {action}
      </div>
      {children}
    </section>
  );
}

/** Terms holds the Term rows of a panel. */
export function Terms({ children }: { children: ReactNode }) {
  return (
    <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-2 text-sm">
      {children}
    </dl>
  );
}

/** Term is one labelled value of a panel. note shows under the value. */
export function Term({
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

function SectionPanel({
  section,
  action,
}: {
  section: ClaimSection;
  action: ReactNode;
}) {
  const name = `${section.sectionLabel}: ${section.optionLabel}`;
  return (
    <Panel label={section.sectionLabel} name={name} action={action}>
      <p className="text-sm">
        <span className="font-medium">{section.optionLabel}</span>
        <span className={NOTE}>
          {lockLabel({ reason: "chosenAtCreation" })}
        </span>
      </p>
      {section.parameters.length === 0 ? (
        <p className="text-sm text-muted">No parameters.</p>
      ) : (
        <Table
          label={`${name} parameters table`}
          aria-label={`${name} parameters`}
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
                  <FieldValue value={field.value} />
                </TableCell>
                <TableCell className="align-top whitespace-normal">
                  <FieldState field={field} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </Panel>
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
