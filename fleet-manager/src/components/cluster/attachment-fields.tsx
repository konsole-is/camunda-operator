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
import {
  type ClusterAttachments,
  DEFAULT_SCHEDULE,
  optimizeBlocked,
  scheduleBlocked,
} from "@/lib/clusters/attachments";
import { type FieldLock, lockLabel, pathFrom, type Step } from "@/lib/fields";
import type { KindName } from "@/lib/k8s/kinds";
import { byCodeUnit } from "@/lib/policy";
import type { FieldError } from "@/lib/session";
import { useEditorGeneration } from "./edit/editor-generation";
import { errorsAt } from "./settings-editor";

const f = (field: string): Step => ({ field });
const OPTIMIZE: Step[] = [f("spec"), f("attachments"), f("optimize")];
const AUTH: Step[] = [...OPTIMIZE, f("managementAuthConfigRef")];
const URL: Step[] = [...OPTIMIZE, f("externalUrl")];
const SCHEDULE: Step[] = [f("spec"), f("attachments"), f("backupSchedule")];
const CRON: Step[] = [...SCHEDULE, f("schedule")];
const COMPLETED: Step[] = [...SCHEDULE, f("retained"), f("completed")];
const FAILED: Step[] = [...SCHEDULE, f("retained"), f("failed")];

/**
 * AttachmentContext is what AttachmentFields reads beside the value.
 * secondaryStorage is the type of the secondary storage, undefined when it
 * is not known. managementAuthConfigs are names, sorted. lock gives the
 * lock of a claim path, as steps from the root of the claim. errors are
 * field errors of the claim. pending is the count of pending changes of
 * the page, 0 on a page without a session. lockedUrl shows in a locked
 * external URL row when the claim sets no URL. optimizeHref links the
 * running Optimize. unread are the kinds that could not be read.
 */
export type AttachmentContext = {
  secondaryStorage?: string;
  backupStorage: boolean;
  managementAuthConfigs: readonly string[];
  lock: (steps: Step[]) => FieldLock | undefined;
  errors: readonly FieldError[];
  pending: number;
  lockedUrl?: string;
  optimizeHref?: string;
  unread?: readonly KindName[];
};

/**
 * AttachmentFields edits spec.attachments of a claim, as value. Attach
 * Optimize sets spec.attachments.optimize to the first ManagementAuthConfig,
 * and turning it off removes the attachment with its settings. Optimize
 * attaches only as optimizeBlocked allows, but an attached Optimize stays
 * removable. While Optimize is on, the fields edit its ManagementAuthConfig
 * and its external URL, which shows locked while lock gives a lock for it.
 * Back up on a schedule sets DEFAULT_SCHEDULE and needs backup storage.
 * While it is on, the fields edit the cron and the counts to keep, where an
 * empty count removes the field. A count with an error is not set: invalid
 * gets its message, and undefined once the error or the field goes. set,
 * reset, and invalid take steps from the root of the claim, and invalid
 * must be stable.
 */
export function AttachmentFields({
  value,
  set,
  reset,
  invalid,
  context,
}: {
  value: ClusterAttachments | undefined;
  set: (steps: Step[], value: unknown) => void;
  reset: (steps: Step[]) => void;
  invalid: (steps: Step[], message: string | undefined) => void;
  context: AttachmentContext;
}) {
  const { generation, own } = useEditorGeneration(context.pending);
  const optimize = value?.optimize;
  const schedule = value?.backupSchedule;
  const auths = context.managementAuthConfigs;
  const optimizeOff = optimizeBlocked(
    context.secondaryStorage,
    auths,
    context.unread?.includes("ManagementAuthConfig"),
  );
  const scheduleOff = scheduleBlocked(context.backupStorage);
  const urlLock = context.lock(URL);
  const at = (steps: Step[]) =>
    errorsAt(context.errors, steps).map((e) => e.message);
  const exactly = (steps: Step[]) => {
    const path = pathFrom("", steps);
    return context.errors.filter((e) => e.path === path).map((e) => e.message);
  };

  return (
    <div className="flex flex-col gap-6">
      <Toggle
        label="Attach Optimize"
        checked={optimize !== undefined}
        off={optimize === undefined ? optimizeOff : undefined}
        lock={context.lock(OPTIMIZE)}
        errors={exactly(OPTIMIZE)}
        onChange={(on) =>
          on
            ? set(OPTIMIZE, { managementAuthConfigRef: auths[0] })
            : reset(OPTIMIZE)
        }
      >
        {optimize && (
          <>
            <Field label="ManagementAuthConfig" errors={at(AUTH)}>
              {(props) => (
                <NativeSelect
                  {...props}
                  value={optimize.managementAuthConfigRef}
                  onChange={(event) => set(AUTH, event.target.value)}
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
                value={optimize.externalUrl ?? context.lockedUrl ?? "Not set"}
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
                        ? reset(URL)
                        : set(URL, event.target.value)
                    }
                  />
                )}
              </Field>
            )}
            {context.optimizeHref && (
              <Link
                href={context.optimizeHref}
                className="relative self-start text-sm font-medium text-text underline underline-offset-4 hit-area hover:text-accent"
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
        lock={context.lock(SCHEDULE)}
        errors={exactly(SCHEDULE)}
        onChange={(on) =>
          on ? set(SCHEDULE, { schedule: DEFAULT_SCHEDULE }) : reset(SCHEDULE)
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
                  onChange={(event) => set(CRON, event.target.value)}
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
                set={own(set)}
                reset={own(reset)}
                invalid={invalid}
              />
              <CountField
                label="Failed backups to keep"
                steps={FAILED}
                minimum={0}
                value={schedule.retained?.failed}
                errors={at(FAILED)}
                set={own(set)}
                reset={own(reset)}
                invalid={invalid}
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
  lock: FieldLock | undefined;
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
  invalid,
}: {
  label: string;
  steps: Step[];
  minimum: number;
  value: number | undefined;
  errors: string[];
  set: (steps: Step[], value: unknown) => void;
  reset: (steps: Step[]) => void;
  invalid: (steps: Step[], message: string | undefined) => void;
}) {
  // The fields remount the counts when the pending changes end.
  const { generation, own } = useEditorGeneration(0, value);
  useEffect(() => () => invalid(steps, undefined), [invalid, steps]);
  return (
    <CountInput
      key={generation}
      label={label}
      minimum={minimum}
      value={value}
      errors={errors}
      onChange={own((next: string) => {
        const nextError = countError(next, minimum);
        invalid(steps, nextError && `${label}: ${nextError}`);
        if (nextError) return;
        if (next === "") reset(steps);
        else set(steps, Number(next));
      })}
    />
  );
}

function CountInput({
  label,
  minimum,
  value,
  errors,
  onChange,
}: {
  label: string;
  minimum: number;
  value: number | undefined;
  errors: string[];
  onChange: (text: string) => void;
}) {
  const id = useId();
  const [text, setText] = useState(value === undefined ? "" : String(value));
  const error = countError(text, minimum) ?? (errors.join(" ") || undefined);
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
          setText(event.target.value);
          onChange(event.target.value);
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
