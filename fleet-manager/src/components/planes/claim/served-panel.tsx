"use client";

import { useEditorGeneration } from "@/components/cluster/edit/editor-generation";
import { useEffect, useId } from "react";
import { errorsAt } from "@/components/cluster/settings-editor";
import { KeyValueEditor } from "@/components/fleet/key-value-editor";
import { PreviewTable } from "@/components/planes/create/served-step";
import { Checkbox } from "@/components/ui/checkbox";
import type { Step } from "@/lib/fields";
import {
  PREVIEW_NEEDS_ISSUER,
  servedChangeLines,
  servedChanges,
} from "@/lib/planes/impacts";
import { appliedSession, claimAfter } from "@/lib/session";
import { FieldErrors } from "./fields";
import { selectorText } from "./plane-claim-tab";
import {
  changesWaitText,
  planeChangeInput,
  usePlaneClaimEdit,
} from "./plane-claim-edit";

const ENVIRONMENTS: Step[] = [{ field: "spec" }, { field: "environments" }];
const SELECTOR: Step[] = [{ field: "spec" }, { field: "clusterSelector" }];

/** EXPRESSIONS_READ_ONLY is the note under the match expressions of the selector, which the panel does not edit. */
export const EXPRESSIONS_READ_ONLY =
  "Change match expressions in the claim YAML.";

/** NO_SELECTOR is the hint under the cluster labels of a claim without a cluster selector. */
export const NO_SELECTOR =
  "The claim has no cluster selector, so the plane serves no cluster. Add a label to serve the clusters that carry it.";
/** EMPTY_LABELS is the hint under the cluster labels of a claim with a cluster selector. */
export const EMPTY_LABELS =
  "Leave empty to serve every cluster in these environments.";

type Selector = {
  matchLabels?: Record<string, string> | null;
  matchExpressions?: unknown[] | null;
};

/**
 * ServedPanel edits the environments that the plane serves and the labels
 * of its cluster selector. A change of the labels keeps the match
 * expressions, which it shows read only. Below, it lists the clusters that
 * the change attaches and detaches, and the expected clusters after it.
 * While a list that the preview reads is unread, it says that the preview
 * is not known. It renders nothing without a session.
 */
export function ServedPanel() {
  const plane = usePlaneClaimEdit();
  const id = useId();
  const edit = plane.edit;
  const session = edit?.session;
  // An editor that keeps its own text starts again after a discard, a
  // write, or a new selector of the live claim.
  const { generation, own } = useEditorGeneration(
    edit?.pending ?? 0,
    session?.claim.spec?.clusterSelector,
  );
  if (!edit || !session) return null;
  const after = claimAfter(session);
  const chosen = Array.isArray(after.spec?.environments)
    ? after.spec.environments.filter((e): e is string => typeof e === "string")
    : [];
  const names = [
    ...new Set([
      ...plane.environments.map((e) => e.metadata?.name ?? ""),
      ...chosen,
    ]),
  ]
    .filter(Boolean)
    .sort();
  const selector = (after.spec?.clusterSelector ?? undefined) as
    Selector | undefined;
  const expressions = selector?.matchExpressions ?? [];
  const toggle = (name: string, on: boolean) =>
    edit.set(
      ENVIRONMENTS,
      on ? [...chosen, name] : chosen.filter((n) => n !== name),
    );
  // The preview reads the claim that the save leaves.
  const input = planeChangeInput(plane, appliedSession(session));
  const changes = plane.unread.length ? undefined : servedChanges(input);
  const lines = changes ? servedChangeLines(changes, input) : [];

  return (
    <div className="flex min-w-0 flex-col gap-6">
      <fieldset className="flex min-w-0 flex-col gap-2">
        <legend className="mb-1 text-sm font-medium text-text">
          Environments
        </legend>
        {names.map((name) => (
          <div key={name} className="flex items-center gap-2">
            <Checkbox
              id={`${id}-${name}`}
              checked={chosen.includes(name)}
              onCheckedChange={(state) => toggle(name, state === true)}
            />
            <label htmlFor={`${id}-${name}`} className="text-sm text-text">
              {name}
            </label>
          </div>
        ))}
        <FieldErrors errors={errorsAt(edit.errors, ENVIRONMENTS)} />
      </fieldset>
      <div className="flex max-w-xl min-w-0 flex-col gap-2">
        <LabelsEditor
          key={generation}
          defaultValue={selector?.matchLabels ?? undefined}
          describedBy={`${id}-labels-hint`}
          invalid={edit.invalid}
          onChange={own((labels: Record<string, string> | undefined) => {
            const rest: Selector = { ...selector };
            delete rest.matchLabels;
            edit.set(
              SELECTOR,
              labels ? { ...rest, matchLabels: labels } : rest,
            );
          })}
        />
        <p id={`${id}-labels-hint`} className="text-sm text-muted">
          {selector == null ? NO_SELECTOR : EMPTY_LABELS}
        </p>
        {expressions.length > 0 && (
          <p className="text-sm text-text-2">
            Match expressions: {selectorText({ matchExpressions: expressions })}
            <span className="block text-xs text-muted">
              {EXPRESSIONS_READ_ONLY}
            </span>
          </p>
        )}
        <FieldErrors errors={errorsAt(edit.errors, SELECTOR)} />
      </div>
      {plane.unread.length > 0 ? (
        <p className="text-sm text-muted">
          {changesWaitText(plane.unread, plane.waiting)}
        </p>
      ) : changes ? (
        <>
          {lines.length > 0 && (
            <ul
              aria-label="Clusters that attach or detach"
              className="flex list-disc flex-col gap-1 pl-5 text-sm text-text"
            >
              {lines.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          )}
          <PreviewTable rows={changes.after} />
        </>
      ) : (
        <p className="text-sm text-muted">{PREVIEW_NEEDS_ISSUER}</p>
      )}
    </div>
  );
}

// LabelsEditor edits the labels of the cluster selector. It reports the
// error that it shows to invalid, and clears it when it unmounts.
function LabelsEditor({
  defaultValue,
  describedBy,
  invalid,
  onChange,
}: {
  defaultValue: Record<string, string> | undefined;
  describedBy: string;
  invalid: (steps: Step[], message: string | undefined) => void;
  onChange: (labels: Record<string, string> | undefined) => void;
}) {
  useEffect(() => () => invalid(SELECTOR, undefined), [invalid]);
  return (
    <KeyValueEditor
      label="Cluster labels"
      keyLabel="Label"
      format="label"
      defaultValue={defaultValue}
      describedBy={describedBy}
      onChange={(labels, errors) => {
        invalid(SELECTOR, errors.length > 0 ? errors[0].message : undefined);
        if (errors.length === 0) onChange(labels);
      }}
    />
  );
}
