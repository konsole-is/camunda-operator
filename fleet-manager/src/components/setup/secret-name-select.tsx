"use client";

import { useId } from "react";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";

/**
 * SecretNameSelect picks a Secret of namespace by name, or None (""). A
 * value that is not in names stays picked, marked as not found, so a save
 * does not drop it. Without names, the list is not known: the value stays
 * picked and is not marked.
 */
export function SecretNameSelect({
  label,
  value,
  names,
  namespace,
  onChange,
}: {
  label: string;
  value: string;
  names?: string[];
  namespace: string;
  onChange: (name: string) => void;
}) {
  const id = useId();
  const missing = names !== undefined && value !== "" && !names.includes(value);
  const unlisted = value !== "" && !(names ?? []).includes(value);
  return (
    <div className="space-y-1">
      <label htmlFor={id} className="text-sm font-medium text-text">
        {label}
      </label>
      <NativeSelect
        id={id}
        value={value}
        aria-invalid={missing || undefined}
        aria-describedby={missing ? `${id}-missing` : undefined}
        onChange={(event) => onChange(event.target.value)}
      >
        <NativeSelectOption value="">None</NativeSelectOption>
        {unlisted && (
          <NativeSelectOption value={value}>
            {missing ? `${value} (not found)` : value}
          </NativeSelectOption>
        )}
        {(names ?? []).map((name) => (
          <NativeSelectOption key={name} value={name}>
            {name}
          </NativeSelectOption>
        ))}
      </NativeSelect>
      {missing && (
        <p id={`${id}-missing`} className="text-sm text-warn">
          The Secret {value} does not exist in namespace {namespace}.
        </p>
      )}
    </div>
  );
}
