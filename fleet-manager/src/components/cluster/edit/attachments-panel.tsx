"use client";

import Link from "next/link";
import { type ReactNode, useEffect, useId, useState } from "react";
import { LockedField } from "@/components/fleet/locked-field";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { lockLabel, pathFrom, type Step } from "@/lib/fields";
import { byCodeUnit } from "@/lib/policy";
import { claimAfter, fieldLock } from "@/lib/session";
import { errorsAt } from "../settings-editor";
import type { ClusterSlotContext } from "../slots";
import { useClusterEdit } from "./cluster-edit";
import { useEditorGeneration } from "./editor-generation";

const f = (field: string): Step => ({ field });
const OPTIMIZE: Step[] = [f("spec"), f("attachments"), f("optimize")];
const AUTH: Step[] = [...OPTIMIZE, f("managementAuthConfigRef")];
const URL: Step[] = [...OPTIMIZE, f("externalUrl")];
const SCHEDULE: Step[] = [f("spec"), f("attachments"), f("backupSchedule")];
const CRON: Step[] = [...SCHEDULE, f("schedule")];
const COMPLETED: Step[] = [...SCHEDULE, f("retained"), f("completed")];
const FAILED: Step[] = [...SCHEDULE, f("retained"), f("failed")];

/** DEFAULT_SCHEDULE is the cron of a new backup schedule: each day at 02:00 UTC. */
export const DEFAULT_SCHEDULE = "0 2 * * *";

/**
 * AttachmentsPanel edits the attachments of the claim. Attach Optimize
 * sets spec.attachments.optimize to the first ManagementAuthConfig by
 * name, and turning it off removes the attachment with its settings. It
 * needs Elasticsearch as secondary storage and a ManagementAuthConfig.
 * While Optimize is on, the panel edits its ManagementAuthConfig and its
 * external URL, which stays locked while a section fills it. Back up on a
 * schedule sets DEFAULT_SCHEDULE and needs a backup storage option; while
 * it is on, the panel edits the cron and the counts to keep, where an
 * empty count removes the field. It renders nothing without a session.
 */
export function AttachmentsPanel({ context }: { context: ClusterSlotContext }) {
  const edit = useClusterEdit();
  const { session } = edit;
  const { generation, own } = useEditorGeneration(edit.pending);
  if (!session) return null;
  const after = claimAfter(session).spec?.attachments;
  const optimize = after?.optimize;
  const schedule = after?.backupSchedule;
  const auths = context.managementAuthConfigs
    .map((a) => a.metadata?.name ?? "")
    .sort(byCodeUnit);
  const optimizeOff =
    context.storage?.spec.type !== "elasticsearch"
      ? "Optimize needs Elasticsearch as secondary storage."
      : auths.length === 0
        ? "No ManagementAuthConfig exists. Your admin creates one first."
        : undefined;
  const scheduleOff =
    context.backupStorage.kind === "none"
      ? "Needs a backup storage option."
      : undefined;
  const urlLock = fieldLock(session, URL);
  const at = (steps: Step[]) =>
    errorsAt(edit.errors, steps).map((e) => e.message);
  const exactly = (steps: Step[]) => {
    const path = pathFrom("", steps);
    return edit.errors.filter((e) => e.path === path).map((e) => e.message);
  };

  return (
    <div className="flex flex-col gap-6">
      <Toggle
        label="Attach Optimize"
        checked={optimize !== undefined}
        off={optimize === undefined ? optimizeOff : undefined}
        lock={fieldLock(session, OPTIMIZE)}
        errors={exactly(OPTIMIZE)}
        onChange={(on) =>
          on
            ? edit.set(OPTIMIZE, { managementAuthConfigRef: auths[0] })
            : edit.reset(OPTIMIZE)
        }
      >
        {optimize && (
          <>
            <Field label="ManagementAuthConfig" errors={at(AUTH)}>
              {(props) => (
                <NativeSelect
                  {...props}
                  value={optimize.managementAuthConfigRef}
                  onChange={(event) => edit.set(AUTH, event.target.value)}
                >
                  {[...new Set([optimize.managementAuthConfigRef, ...auths])]
                    .sort(byCodeUnit)
                    .map((name) => (
                      <NativeSelectOption key={name} value={name}>
                        {name}
                      </NativeSelectOption>
                    ))}
                </NativeSelect>
              )}
            </Field>
            {urlLock ? (
              <LockedField
                label="External URL"
                value={
                  optimize.externalUrl ??
                  context.optimize?.spec.externalUrl ??
                  "Not set"
                }
                reason={lockLabel(urlLock)}
              />
            ) : (
              <Field label="External URL" errors={at(URL)}>
                {(props) => (
                  <Input
                    {...props}
                    type="url"
                    spellCheck={false}
                    placeholder="https://optimize.example.com"
                    value={optimize.externalUrl ?? ""}
                    onChange={(event) =>
                      event.target.value === ""
                        ? edit.reset(URL)
                        : edit.set(URL, event.target.value)
                    }
                  />
                )}
              </Field>
            )}
            {context.optimize && (
              <Link
                href="?tab=overview&component=optimize"
                className="self-start text-sm font-medium text-text underline underline-offset-4 hover:text-accent"
              >
                Open Optimize
              </Link>
            )}
          </>
        )}
      </Toggle>
      <Toggle
        label="Back up on a schedule"
        checked={schedule !== undefined}
        off={schedule === undefined ? scheduleOff : undefined}
        lock={fieldLock(session, SCHEDULE)}
        errors={exactly(SCHEDULE)}
        onChange={(on) =>
          on
            ? edit.set(SCHEDULE, { schedule: DEFAULT_SCHEDULE })
            : edit.reset(SCHEDULE)
        }
      >
        {schedule && (
          <>
            <Field label="Schedule (cron, UTC)" errors={at(CRON)}>
              {(props) => (
                <Input
                  {...props}
                  className="max-w-56 font-mono"
                  spellCheck={false}
                  value={schedule.schedule}
                  onChange={(event) => edit.set(CRON, event.target.value)}
                />
              )}
            </Field>
            <div key={generation} className="flex flex-wrap gap-4">
              <CountField
                label="Completed backups to keep"
                steps={COMPLETED}
                minimum={1}
                value={schedule.retained?.completed}
                errors={at(COMPLETED)}
                set={own(edit.set)}
                reset={own(edit.reset)}
              />
              <CountField
                label="Failed backups to keep"
                steps={FAILED}
                minimum={0}
                value={schedule.retained?.failed}
                errors={at(FAILED)}
                set={own(edit.set)}
                reset={own(edit.reset)}
              />
            </div>
          </>
        )}
      </Toggle>
    </div>
  );
}

function Toggle({
  label,
  checked,
  off,
  lock,
  errors,
  onChange,
  children,
}: {
  label: string;
  checked: boolean;
  off?: string;
  lock: ReturnType<typeof fieldLock>;
  errors: string[];
  onChange: (on: boolean) => void;
  children: ReactNode;
}) {
  const id = useId();
  return (
    <fieldset className="flex min-w-0 flex-col gap-3">
      <div className="flex items-start gap-3">
        <Checkbox
          id={id}
          className="mt-0.5"
          checked={checked}
          disabled={off !== undefined || lock !== undefined}
          aria-describedby={off || lock ? `${id}-note` : undefined}
          onCheckedChange={(value) => onChange(value === true)}
        />
        <div className="flex flex-col gap-0.5">
          <label htmlFor={id} className="text-sm font-medium text-text">
            {label}
          </label>
          {(off || lock) && (
            <p id={`${id}-note`} className="text-xs text-muted">
              {off ?? (lock && lockLabel(lock))}
            </p>
          )}
          <Errors errors={errors} />
        </div>
      </div>
      {checked && (
        <div className="ml-7 flex flex-col gap-4 border-l border-divider pl-4">
          {children}
        </div>
      )}
    </fieldset>
  );
}

// Field labels the input that children renders with the props it gets,
// and shows errors under it.
function Field({
  label,
  errors,
  children,
}: {
  label: string;
  errors: string[];
  children: (props: {
    id: string;
    "aria-invalid"?: true;
    "aria-describedby"?: string;
  }) => ReactNode;
}) {
  const id = useId();
  const invalid = errors.length > 0;
  return (
    <div className="flex max-w-md flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium text-text">
        {label}
      </label>
      {children({
        id,
        ...(invalid && {
          "aria-invalid": true,
          "aria-describedby": `${id}-error`,
        }),
      })}
      <Errors id={`${id}-error`} errors={errors} />
    </div>
  );
}

function Errors({ id, errors }: { id?: string; errors: string[] }) {
  if (errors.length === 0) return null;
  return (
    <p id={id} className="text-xs text-bad">
      {errors.join(" ")}
    </p>
  );
}

// CountField keeps its own text, so that a count with an error stays in
// the input while the session keeps the last valid count. It clears its
// error when it unmounts.
function CountField({
  label,
  steps,
  minimum,
  value,
  errors,
  set,
  reset,
}: {
  label: string;
  steps: Step[];
  minimum: number;
  value: number | undefined;
  errors: string[];
  set: (steps: Step[], value: unknown) => void;
  reset: (steps: Step[]) => void;
}) {
  const edit = useClusterEdit();
  const id = useId();
  const [text, setText] = useState(value === undefined ? "" : String(value));
  const error = countError(text, minimum) ?? (errors.join(" ") || undefined);
  const { invalid } = edit;
  useEffect(() => () => invalid(steps, undefined), [invalid, steps]);
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium text-text">
        {label}
      </label>
      <Input
        id={id}
        type="number"
        min={minimum}
        step={1}
        className="w-32"
        value={text}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${id}-error` : undefined}
        onChange={(event) => {
          const next = event.target.value;
          setText(next);
          const nextError = countError(next, minimum);
          edit.invalid(steps, nextError && `${label}: ${nextError}`);
          if (nextError) return;
          if (next === "") reset(steps);
          else set(steps, Number(next));
        }}
      />
      {error && (
        <p id={`${id}-error`} className="text-xs text-bad">
          {error}
        </p>
      )}
    </div>
  );
}

function countError(text: string, minimum: number): string | undefined {
  if (text === "") return undefined;
  const n = Number(text);
  if (!Number.isInteger(n)) return "Enter a whole number";
  if (n < minimum) return `Enter ${minimum} or more`;
  return undefined;
}
