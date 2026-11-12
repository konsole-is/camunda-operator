"use client";

import Link from "next/link";
import { type ReactNode, useId, useState } from "react";
import { LockedField } from "@/components/fleet/locked-field";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { lockLabel, type Step } from "@/lib/fields";
import { byCodeUnit } from "@/lib/policy";
import { claimAfter, fieldLock } from "@/lib/session";
import type { ClusterSlotContext } from "../slots";
import { useClusterEdit } from "./cluster-edit";

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

  return (
    <div className="flex flex-col gap-6">
      <Toggle
        label="Attach Optimize"
        checked={optimize !== undefined}
        off={optimize === undefined ? optimizeOff : undefined}
        lock={fieldLock(session, OPTIMIZE)}
        onChange={(on) =>
          on
            ? edit.set(OPTIMIZE, { managementAuthConfigRef: auths[0] })
            : edit.reset(OPTIMIZE)
        }
      >
        {optimize && (
          <>
            <Field label="ManagementAuthConfig">
              {(id) => (
                <NativeSelect
                  id={id}
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
                value={optimize.externalUrl ?? "Not set"}
                reason={lockLabel(urlLock)}
              />
            ) : (
              <Field label="External URL">
                {(id) => (
                  <Input
                    id={id}
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
                Optimize settings
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
        onChange={(on) =>
          on
            ? edit.set(SCHEDULE, { schedule: DEFAULT_SCHEDULE })
            : edit.reset(SCHEDULE)
        }
      >
        {schedule && (
          <>
            <Field label="Schedule (cron, UTC)">
              {(id) => (
                <Input
                  id={id}
                  className="max-w-56 font-mono"
                  spellCheck={false}
                  value={schedule.schedule}
                  onChange={(event) => edit.set(CRON, event.target.value)}
                />
              )}
            </Field>
            <div className="flex flex-wrap gap-4">
              <CountField
                label="Completed backups to keep"
                steps={COMPLETED}
                value={schedule.retained?.completed}
              />
              <CountField
                label="Failed backups to keep"
                steps={FAILED}
                value={schedule.retained?.failed}
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
  onChange,
  children,
}: {
  label: string;
  checked: boolean;
  off?: string;
  lock: ReturnType<typeof fieldLock>;
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

function Field({
  label,
  children,
}: {
  label: string;
  children: (id: string) => ReactNode;
}) {
  const id = useId();
  return (
    <div className="flex max-w-md flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium text-text">
        {label}
      </label>
      {children(id)}
    </div>
  );
}

// CountField keeps its own text, so that a count with an error stays in
// the input while the session keeps the last valid count.
function CountField({
  label,
  steps,
  value,
}: {
  label: string;
  steps: Step[];
  value: number | undefined;
}) {
  const edit = useClusterEdit();
  const id = useId();
  const [text, setText] = useState(value === undefined ? "" : String(value));
  const error = countError(text);
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium text-text">
        {label}
      </label>
      <Input
        id={id}
        type="number"
        min={0}
        step={1}
        className="w-32"
        value={text}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${id}-error` : undefined}
        onChange={(event) => {
          const next = event.target.value;
          setText(next);
          const nextError = countError(next);
          edit.invalid(steps, nextError && `${label}: ${nextError}`);
          if (nextError) return;
          if (next === "") edit.reset(steps);
          else edit.set(steps, Number(next));
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

function countError(text: string): string | undefined {
  if (text === "") return undefined;
  const n = Number(text);
  if (!Number.isInteger(n)) return "Enter a whole number";
  if (n < 0) return "Enter 0 or more";
  return undefined;
}
