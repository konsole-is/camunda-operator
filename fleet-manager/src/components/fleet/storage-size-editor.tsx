"use client";

import { useId, useState } from "react";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { parseQuantity } from "@/lib/observe/quantity";
import type { ValidationError } from "@/lib/schema";

const UNITS = [
  { value: "Mi", label: "MiB" },
  { value: "Gi", label: "GiB" },
  { value: "Ti", label: "TiB" },
];
// The number part of a size: a positive decimal number.
const AMOUNT = /^(\d+\.?\d*|\.\d+)$/;
const SHRINK = "Volumes cannot shrink. The operator keeps the current size.";
const NOT_POSITIVE = "Enter a size above 0";

/**
 * StorageSizeEditor edits a volume size as an amount and a unit (Mi, Gi,
 * Ti). A value in another unit keeps its unit. Empty means not set. A size
 * below current shows that volumes cannot shrink. onChange gets the size,
 * or undefined when empty, and the errors: an amount that is not a
 * positive number, at the path "".
 */
export function StorageSizeEditor({
  label,
  defaultValue,
  current,
  onChange,
}: {
  label: string;
  defaultValue?: string;
  current?: string;
  onChange?: (value: string | undefined, errors: ValidationError[]) => void;
}) {
  const id = useId();
  const [size, setSize] = useState(() => parse(defaultValue));
  const error = errorOf(size.amount);
  const shrinks = !error && size.amount !== "" && below(size, current);
  const units = UNITS.some((u) => u.value === size.unit)
    ? UNITS
    : [...UNITS, { value: size.unit, label: size.unit }];

  const update = (next: { amount: string; unit: string }) => {
    setSize(next);
    const nextError = errorOf(next.amount);
    onChange?.(
      next.amount === "" ? undefined : `${next.amount}${next.unit}`,
      nextError ? [{ path: "", message: nextError }] : [],
    );
  };

  return (
    <fieldset className="flex min-w-0 flex-col gap-1">
      <legend className="mb-1 text-sm font-medium text-muted">{label}</legend>
      <div className="flex gap-2">
        <Input
          aria-label="Amount"
          inputMode="decimal"
          className="w-32"
          value={size.amount}
          aria-invalid={error ? true : undefined}
          aria-describedby={
            error ? `${id}-error` : shrinks ? `${id}-shrink` : undefined
          }
          onChange={(event) => update({ ...size, amount: event.target.value })}
        />
        <label className="sr-only" htmlFor={`${id}-unit`}>
          Unit
        </label>
        <NativeSelect
          id={`${id}-unit`}
          className="w-28"
          value={size.unit}
          onChange={(event) => update({ ...size, unit: event.target.value })}
        >
          {units.map((u) => (
            <NativeSelectOption key={u.value} value={u.value}>
              {u.label}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>
      {error && (
        <p id={`${id}-error`} className="text-xs text-bad">
          {error}
        </p>
      )}
      {shrinks && (
        <p id={`${id}-shrink`} className="text-xs text-warn">
          {SHRINK}
        </p>
      )}
    </fieldset>
  );
}

function parse(size: string | undefined): { amount: string; unit: string } {
  if (size === undefined || size === "") return { amount: "", unit: "Gi" };
  const match = /^(\d+\.?\d*|\.\d+)([A-Za-z]*)$/.exec(size);
  return match
    ? { amount: match[1], unit: match[2] }
    : { amount: size, unit: "" };
}

function errorOf(amount: string): string | undefined {
  if (amount === "") return undefined;
  return AMOUNT.test(amount) && Number(amount) > 0 ? undefined : NOT_POSITIVE;
}

function below(
  size: { amount: string; unit: string },
  current: string | undefined,
): boolean {
  if (current === undefined) return false;
  try {
    return parseQuantity(`${size.amount}${size.unit}`) < parseQuantity(current);
  } catch {
    return false;
  }
}
