"use client";

import { useId } from "react";
import { Badge } from "@/components/ui/badge";
import type { DeletionChoice, DeletionPolicy } from "@/lib/policy";
import { cn } from "@/lib/utils";

/**
 * DeletionChoiceField asks "When the <subject> is deleted": "Keep the
 * resources" or "Delete the resources", with "Default" on the default of
 * choice and choice.policy checked. Keep is disabled while
 * choice.keepAllowed is false, with the options that always delete what
 * they made. When the default keeps and Keep is disabled, a line says why
 * Delete is chosen. Each item of choice.alwaysDeleted shows with "Always
 * deleted".
 */
export function DeletionChoiceField({
  subject,
  choice,
  onChange,
}: {
  subject: "cluster" | "management plane";
  choice: DeletionChoice;
  onChange: (policy: DeletionPolicy) => void;
}) {
  const id = useId();
  return (
    <fieldset className="flex min-w-0 flex-col gap-3">
      <legend className="mb-1 font-heading text-base font-semibold text-text">
        {`When the ${subject} is deleted`}
      </legend>
      <p className="-mt-1 text-sm text-muted">
        This applies to the resources of the storage and access options.
      </p>
      <div className="grid gap-3 md:grid-cols-2">
        <PolicyCard
          name={id}
          label="Keep the resources"
          description={`They stay after the ${subject} is gone.`}
          checked={choice.policy === "Orphan"}
          isDefault={choice.defaultPolicy === "Orphan"}
          disabledReason={
            choice.keepAllowed
              ? undefined
              : "Some options always delete what they made."
          }
          onPick={() => onChange("Orphan")}
        />
        <PolicyCard
          name={id}
          label="Delete the resources"
          description={`They are deleted with the ${subject}.`}
          checked={choice.policy === "Delete"}
          isDefault={choice.defaultPolicy === "Delete"}
          onPick={() => onChange("Delete")}
        />
      </div>
      {choice.defaultPolicy === "Orphan" && !choice.keepAllowed && (
        <p className="text-sm text-text-2">
          The default keeps the resources, but the options below cannot keep
          them.
        </p>
      )}
      {choice.alwaysDeleted.length > 0 && (
        <ul className="flex flex-col gap-1.5">
          {choice.alwaysDeleted.map((item) => (
            <li
              key={`${item.section}/${item.label}`}
              className="flex flex-wrap items-center gap-2 text-sm text-text-2"
            >
              {item.label}
              <Badge variant="outline">Always deleted</Badge>
            </li>
          ))}
        </ul>
      )}
    </fieldset>
  );
}

// The card follows the option cards of SectionCards, so the choice reads as
// one more section of the step.
function PolicyCard({
  name,
  label,
  description,
  checked,
  isDefault,
  disabledReason,
  onPick,
}: {
  name: string;
  label: string;
  description: string;
  checked: boolean;
  isDefault: boolean;
  disabledReason?: string;
  onPick: () => void;
}) {
  return (
    <label
      className={cn(
        "flex min-w-0 gap-3 rounded-md border bg-panel p-3",
        checked ? "border-accent" : "border-border",
        disabledReason ? "cursor-not-allowed" : "cursor-pointer",
      )}
    >
      <input
        type="radio"
        name={name}
        checked={checked}
        disabled={!!disabledReason}
        onChange={onPick}
        className="mt-1 size-4 shrink-0 accent-[var(--accent)]"
      />
      <span className="flex min-w-0 flex-col gap-0.5">
        <span
          className={cn(
            "flex flex-wrap items-center gap-2 text-sm font-medium",
            disabledReason ? "text-muted" : "text-text",
          )}
        >
          {label}
          {isDefault && <Badge variant="secondary">Default</Badge>}
        </span>
        <span className="text-sm text-muted">{description}</span>
        {disabledReason && (
          <span className="text-sm text-warn">{disabledReason}</span>
        )}
      </span>
    </label>
  );
}
