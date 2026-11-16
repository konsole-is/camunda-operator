"use client";

import { TriangleAlert } from "lucide-react";
import { useId } from "react";
import { Input } from "@/components/ui/input";
import type { FieldError } from "@/lib/session";

/**
 * ClaimTextField is one text input of a panel of the plane Claim tab.
 * errors show under it and mark it invalid. note shows under the label.
 */
export function ClaimTextField({
  label,
  value,
  onChange,
  required = false,
  note,
  placeholder,
  errors = [],
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  required?: boolean;
  note?: string;
  placeholder?: string;
  errors?: readonly FieldError[];
}) {
  const id = useId();
  const described = [note && `${id}-note`, errors.length > 0 && `${id}-error`]
    .filter(Boolean)
    .join(" ");
  return (
    <div className="flex max-w-md min-w-0 flex-col gap-1">
      <label htmlFor={id} className="text-sm font-medium text-text">
        {label}
      </label>
      {note && (
        <p id={`${id}-note`} className="text-sm text-muted">
          {note}
        </p>
      )}
      <Input
        id={id}
        value={value}
        required={required}
        placeholder={placeholder}
        autoComplete="off"
        spellCheck={false}
        aria-invalid={errors.length > 0 || undefined}
        aria-describedby={described || undefined}
        onChange={(event) => onChange(event.target.value)}
      />
      <FieldErrors id={`${id}-error`} errors={errors} />
    </div>
  );
}

/** FieldErrors shows the messages of errors, and nothing without errors. */
export function FieldErrors({
  id,
  errors,
}: {
  id?: string;
  errors: readonly FieldError[];
}) {
  if (errors.length === 0) return null;
  return (
    <ul id={id} className="flex flex-col gap-0.5 text-sm text-bad">
      {errors.map((e, i) => (
        <li key={i}>{e.message}</li>
      ))}
    </ul>
  );
}

/** Notes shows each line of notes as a warning note, and nothing without notes. */
export function Notes({ notes }: { notes: readonly string[] }) {
  if (notes.length === 0) return null;
  return (
    <div className="flex flex-col gap-2">
      {notes.map((note) => (
        <p
          key={note}
          role="note"
          className="flex items-start gap-2 rounded-md border border-warn/40 bg-warn/10 p-3 text-sm text-text"
        >
          <TriangleAlert
            aria-hidden
            className="mt-0.5 size-4 shrink-0 text-warn"
          />
          {note}
        </p>
      ))}
    </div>
  );
}
