"use client";

import { useId } from "react";
import { SelectField } from "@/components/setup/form-parts";
import type { ValidationError } from "@/lib/schema";
import type { SectionGroup, SectionPolicyForm, SectionRow } from "./sections";

/**
 * SectionsTable edits the policy of each section, in one group for each
 * claim kind: Required, the default option, and the allowed options. A
 * section of both kinds shows in both groups and edits one policy. An
 * allowed name that no accepted registration of the section has shows
 * checked, as "Not offered", so the admin can clear it.
 */
export function SectionsTable({
  groups,
  errors,
  onPolicy,
}: {
  groups: readonly SectionGroup[];
  errors: readonly ValidationError[];
  onPolicy: (section: string, policy: SectionPolicyForm) => void;
}) {
  if (groups.length === 0)
    return (
      <p className="text-sm text-muted">
        No section exists. The operator writes the core sections when it
        starts.
      </p>
    );
  return (
    <div className="flex flex-col gap-6">
      <p className="max-w-2xl text-sm text-muted">
        A claim of the environment can pick only an allowed option. A section
        without an allowed option offers only its empty option.
      </p>
      {groups.map((group) => (
        <section
          key={group.label}
          aria-label={group.label}
          className="flex flex-col gap-3"
        >
          <h3 className="font-heading text-sm font-semibold">{group.label}</h3>
          {group.rows.map((row) => (
            <SectionPolicyRow
              key={row.name}
              row={row}
              error={
                errors.find(
                  (e) => e.path === `spec.sections.${row.name}.default`,
                )?.message
              }
              onPolicy={(policy) => onPolicy(row.name, policy)}
            />
          ))}
        </section>
      ))}
    </div>
  );
}

function SectionPolicyRow({
  row,
  error,
  onPolicy,
}: {
  row: SectionRow;
  error?: string;
  onPolicy: (policy: SectionPolicyForm) => void;
}) {
  const id = useId();
  const { policy } = row;
  const labelOf = (name: string) =>
    row.options.find((o) => o.registration === name)?.label ?? name;
  const toggle = (name: string, on: boolean) =>
    onPolicy({
      ...policy,
      allowed: on
        ? [...policy.allowed, name]
        : policy.allowed.filter((n) => n !== name),
    });
  const defaults = [
    ...new Set([
      ...policy.allowed,
      ...(policy.default ? [policy.default] : []),
    ]),
  ];
  return (
    <fieldset className="grid min-w-0 gap-4 rounded-md border border-border bg-panel-2 p-4 md:grid-cols-[minmax(0,14rem)_minmax(0,1fr)]">
      <legend className="sr-only">{row.label}</legend>
      <div className="flex min-w-0 flex-col gap-3">
        <div className="flex flex-col gap-1">
          <p aria-hidden className="text-sm font-semibold text-text">
            {row.label}
          </p>
          {row.description && (
            <p className="text-xs text-muted">{row.description}</p>
          )}
          {row.selection === "Many" && (
            <p className="text-xs text-muted">
              A claim can pick more than one option.
            </p>
          )}
        </div>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            className="size-4 accent-accent"
            checked={policy.required}
            onChange={(e) => onPolicy({ ...policy, required: e.target.checked })}
          />
          Required
        </label>
        <SelectField
          label="Default option"
          value={policy.default ?? ""}
          error={error}
          options={[
            ["", "No default"],
            ...defaults.map((name) => [name, labelOf(name)] as const),
          ]}
          onChange={(value) => onPolicy({ ...policy, default: value })}
        />
      </div>
      <div className="flex min-w-0 flex-col gap-2">
        <p id={`${id}-allowed`} className="text-sm font-medium text-text">
          Allowed options
        </p>
        {row.options.length === 0 && row.missing.length === 0 && (
          <p className="text-sm text-muted">
            No accepted option serves this section.
          </p>
        )}
        <ul aria-labelledby={`${id}-allowed`} className="flex flex-col gap-2">
          {row.options.map((option) => (
            <li key={option.registration} className="flex flex-col gap-0.5">
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="size-4 accent-accent"
                  checked={policy.allowed.includes(option.registration)}
                  onChange={(e) =>
                    toggle(option.registration, e.target.checked)
                  }
                />
                {option.label}
              </label>
              {[option.readyNote, ...option.inputNotes]
                .filter(Boolean)
                .map((note) => (
                  <p key={note} className="pl-6 text-xs text-warn">
                    {note}
                  </p>
                ))}
            </li>
          ))}
          {row.missing.map((name) => (
            <li key={name} className="flex flex-wrap items-center gap-2">
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="size-4 accent-accent"
                  checked
                  onChange={() => toggle(name, false)}
                />
                <span className="font-mono">{name}</span>
              </label>
              <span className="rounded-full bg-bad/15 px-2 py-0.5 text-xs text-bad">
                Not offered
              </span>
            </li>
          ))}
        </ul>
        {row.emptyOptionLabel ? (
          <p className="text-xs text-muted">
            A claim can also pick {row.emptyOptionLabel}.
          </p>
        ) : (
          policy.required && (
            <p className="text-xs text-muted">
              A claim must pick an allowed option.
            </p>
          )
        )}
      </div>
    </fieldset>
  );
}
