"use client";

import { Fragment, type ReactNode } from "react";
import { Panel, StoredYaml, Term, Terms } from "@/components/cluster/claim-tab";
import {
  DELETION_LABELS,
  deletionQuestion,
} from "@/components/fleet/deletion-choice";
import { FieldValue } from "@/components/fleet/field-state";
import { lockLabel, type PlaneField, type PlaneFieldGroup } from "@/lib/fields";
import {
  IDENTITY_MODE_LABEL,
  type IdentityModeId,
} from "@/lib/planes/identity";
import type { PlaneUrlId } from "@/lib/planes/urls";
import type { PlaneClaim } from "@/lib/session";

/** PlaneClaimPanel names one panel of the Claim tab of a plane. */
export type PlaneClaimPanel = PlaneFieldGroup;

/** PLANE_CLAIM_PANELS are the panels of the Claim tab of a plane, in their order, with their headings. */
export const PLANE_CLAIM_PANELS: readonly {
  id: PlaneClaimPanel;
  label: string;
}[] = [
  { id: "basics", label: "Basics" },
  { id: "identity", label: "Identity" },
  { id: "environments", label: "Environments served" },
  { id: "components", label: "Components" },
  { id: "versions", label: "Versions" },
  { id: "databases", label: "Databases" },
  { id: "access", label: "Access" },
  { id: "deletion", label: "When the plane is deleted" },
];

/**
 * PlaneClaimTabEdit puts the tab in edit. panels holds the edit panel that
 * shows in place of the read panel of each name, and actions the control,
 * such as an Edit button, in the header of each read panel.
 */
export type PlaneClaimTabEdit = {
  panels?: Partial<Record<PlaneClaimPanel, ReactNode>>;
  actions?: Partial<Record<PlaneClaimPanel, ReactNode>>;
};

/**
 * PlaneClaimTab shows the claim of a plane: one panel for each group of
 * fields, then Show YAML of stored, the claim as the API server returns it.
 * fields are planeFields of the claim to show. urls are the external URLs
 * that the plane has: the Access panel shows them and their host where the
 * claim sets no URL. Without edit, the tab is read only.
 */
export function PlaneClaimTab({
  fields,
  stored,
  urls,
  edit,
}: {
  fields: readonly PlaneField[];
  stored: PlaneClaim;
  urls: Partial<Record<PlaneUrlId, string>>;
  edit?: PlaneClaimTabEdit;
}) {
  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 lg:grid-cols-2 [&>*]:min-w-0">
        {PLANE_CLAIM_PANELS.map(({ id, label }) => (
          <Fragment key={id}>
            {edit?.panels?.[id] ?? (
              <Panel label={label} action={edit?.actions?.[id]}>
                <GroupTerms
                  group={id}
                  fields={fields.filter((f) => f.group === id)}
                  urls={urls}
                />
              </Panel>
            )}
          </Fragment>
        ))}
      </div>
      <StoredYaml claim={stored} />
    </div>
  );
}

function GroupTerms({
  group,
  fields,
  urls,
}: {
  group: PlaneClaimPanel;
  fields: readonly PlaneField[];
  urls: Partial<Record<PlaneUrlId, string>>;
}) {
  const host = hostOf(urls);
  if (fields.length === 0 && !(group === "access" && host))
    return <p className="text-sm text-muted">{EMPTY[group]}</p>;
  return (
    <Terms>
      {fields.map((field) => {
        const shown =
          field.value === undefined && field.key.startsWith("url.")
            ? urls[field.key.slice(4) as PlaneUrlId]
            : field.value;
        return (
          <Term
            key={field.key}
            label={labelOf(field)}
            note={field.lock ? lockLabel(field.lock) : field.note}
          >
            {shownValue(field, shown)}
          </Term>
        );
      })}
      {group === "access" && host && <Term label="Host">{host}</Term>}
    </Terms>
  );
}

const EMPTY: Record<PlaneClaimPanel, string> = {
  basics: "Not set",
  identity: "Not set",
  environments: "Not set",
  components: "Not set",
  versions: "Not set",
  databases: "No database option.",
  access: "No access from outside.",
  deletion: "Not set",
};

// A parameter row has the parameter name as its label: the Option row
// names its selection.
function labelOf(field: PlaneField): string {
  if (field.group === "deletion") return deletionQuestion("management plane");
  return field.label;
}

function shownValue(field: PlaneField, value: unknown): ReactNode {
  switch (field.key) {
    case "identityProvider":
      return typeof value === "string" && value in IDENTITY_MODE_LABEL ? (
        IDENTITY_MODE_LABEL[value as IdentityModeId]
      ) : (
        <FieldValue value={value} />
      );
    case "console":
    case "webModeler":
      return value ? "Runs" : "Off";
    case "environments":
      return Array.isArray(value) && value.length > 0 ? (
        value.join(", ")
      ) : (
        <span className="text-muted">None</span>
      );
    case "clusterSelector":
      return selectorText(value);
    case "webModeler.mail":
      return mailText(value);
    case "deletionPolicy":
      return typeof value === "string" && value in DELETION_LABELS ? (
        DELETION_LABELS[value as keyof typeof DELETION_LABELS]
      ) : (
        <FieldValue value={value} />
      );
    default:
      return <FieldValue value={value ?? undefined} />;
  }
}

/** selectorText returns a cluster selector in words: no selector serves no cluster, an empty one every cluster. */
export function selectorText(value: unknown): string {
  if (typeof value !== "object" || value === null) return "No cluster";
  const selector = value as {
    matchLabels?: Record<string, string> | null;
    matchExpressions?:
      { key: string; operator: string; values?: string[] | null }[] | null;
  };
  const parts = [
    ...Object.entries(selector.matchLabels ?? {}).map(([k, v]) => `${k}=${v}`),
    ...(selector.matchExpressions ?? []).map((e) =>
      e.values && e.values.length > 0
        ? `${e.key} ${e.operator} (${e.values.join(", ")})`
        : `${e.key} ${e.operator}`,
    ),
  ];
  return parts.length === 0 ? "Every cluster" : parts.join(", ");
}

function mailText(value: unknown): ReactNode {
  if (typeof value !== "object" || value === null)
    return <span className="text-muted">Not set</span>;
  const mail = value as { smtpHost?: unknown; smtpPort?: unknown };
  return typeof mail.smtpHost === "string" ? (
    `${mail.smtpHost}${typeof mail.smtpPort === "number" ? `:${mail.smtpPort}` : ""}`
  ) : (
    <FieldValue value={value} />
  );
}

// hostOf returns the host of the Management Identity URL, else of the
// first URL that parses.
function hostOf(urls: Partial<Record<PlaneUrlId, string>>): string | undefined {
  const ordered = [urls.identity, ...Object.values(urls)];
  for (const url of ordered)
    if (url && URL.canParse(url)) return new URL(url).host;
  return undefined;
}
