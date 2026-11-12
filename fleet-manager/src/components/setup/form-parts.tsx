"use client";

import { type ReactNode, useId } from "react";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { byCodeUnit } from "@/lib/policy";

/** EditorSection is one open part of the editor with its heading. */
export function EditorSection({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <details
      open
      className="group rounded-md border border-border bg-panel open:pb-5"
    >
      <summary className="cursor-pointer px-5 py-3.5 font-heading text-base font-semibold select-none">
        {title}
      </summary>
      <div className="flex flex-col gap-4 px-5">{children}</div>
    </details>
  );
}

/** ErrorText is the error of one field, or nothing. */
export function ErrorText({ id, error }: { id?: string; error?: string }) {
  if (!error) return null;
  return (
    <p id={id} className="text-xs text-bad">
      {error}
    </p>
  );
}

/** TextField is a labelled input with its hint and its error. */
export function TextField({
  label,
  value,
  hint,
  error,
  placeholder,
  mono = false,
  onChange,
  after,
}: {
  label: string;
  value: string;
  hint?: string;
  error?: string;
  placeholder?: string;
  mono?: boolean;
  onChange: (value: string) => void;
  after?: ReactNode;
}) {
  const id = useId();
  const described = [hint && `${id}-hint`, error && `${id}-error`]
    .filter(Boolean)
    .join(" ");
  return (
    <div className="flex max-w-md flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium text-text">
        {label}
      </label>
      <div className="flex items-center gap-2">
        <Input
          id={id}
          value={value}
          placeholder={placeholder}
          spellCheck={false}
          autoComplete="off"
          className={mono ? "font-mono" : undefined}
          aria-invalid={error ? true : undefined}
          aria-describedby={described || undefined}
          onChange={(event) => onChange(event.target.value)}
        />
        {after}
      </div>
      {hint && (
        <p id={`${id}-hint`} className="text-xs text-muted">
          {hint}
        </p>
      )}
      <ErrorText id={`${id}-error`} error={error} />
    </div>
  );
}

/** SelectField is a labelled select of options, each a value and its text. */
export function SelectField({
  label,
  value,
  options,
  error,
  onChange,
}: {
  label: string;
  value: string;
  options: readonly (readonly [string, string])[];
  error?: string;
  onChange: (value: string) => void;
}) {
  const id = useId();
  return (
    <div className="flex max-w-md flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium text-text">
        {label}
      </label>
      <NativeSelect
        id={id}
        value={value}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${id}-error` : undefined}
        onChange={(event) => onChange(event.target.value)}
      >
        {options.map(([v, text]) => (
          <NativeSelectOption key={v} value={v}>
            {text}
          </NativeSelectOption>
        ))}
      </NativeSelect>
      <ErrorText id={`${id}-error`} error={error} />
    </div>
  );
}

/**
 * NameChecklist is a list of checkboxes, one for each name of available and
 * of selected. A selected name that no object has shows "Not found", so the
 * admin can clear it. onChange gets the selected names in code-unit order.
 * noneText, when given, shows while no name is selected.
 */
export function NameChecklist({
  legend,
  available,
  selected,
  empty,
  noneText,
  error,
  onChange,
}: {
  legend: string;
  available: readonly string[];
  selected: readonly string[];
  empty: string;
  noneText?: string;
  error?: string;
  onChange: (selected: string[]) => void;
}) {
  const names = [...new Set([...available, ...selected])].sort(byCodeUnit);
  const toggle = (name: string, on: boolean) =>
    onChange(
      (on ? [...selected, name] : selected.filter((n) => n !== name)).sort(
        byCodeUnit,
      ),
    );
  return (
    <fieldset className="flex min-w-0 flex-col gap-2">
      <legend className="mb-1 text-sm font-medium text-text">{legend}</legend>
      {names.length === 0 && <p className="text-sm text-muted">{empty}</p>}
      <ul className="flex flex-wrap gap-x-5 gap-y-2">
        {names.map((name) => {
          const missing = !available.includes(name);
          return (
            <li key={name}>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="size-4 accent-accent"
                  checked={selected.includes(name)}
                  onChange={(event) => toggle(name, event.target.checked)}
                />
                <span className="font-mono">{name}</span>
                {missing && (
                  <span className="rounded-full bg-bad/15 px-2 py-0.5 text-xs text-bad">
                    Not found
                  </span>
                )}
              </label>
            </li>
          );
        })}
      </ul>
      {noneText && selected.length === 0 && (
        <p className="text-sm text-muted">{noneText}</p>
      )}
      <ErrorText error={error} />
    </fieldset>
  );
}
