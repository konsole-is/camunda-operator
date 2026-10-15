"use client";

import { useId, useState } from "react";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { ValidationError } from "@/lib/schema";

/** Resources are the requests and limits of a container, as the CRDs define them. */
export type Resources = NonNullable<
  NonNullable<CamundaClusterSpec["gateway"]>["resources"]
>;

type Side = "requests" | "limits";
type Resource = "cpu" | "memory";
type Amount = { amount: string; unit: string };

const UNITS: Record<Resource, { value: string; label: string }[]> = {
  cpu: [
    { value: "", label: "cores" },
    { value: "m", label: "millicores" },
  ],
  memory: [
    { value: "Mi", label: "MiB" },
    { value: "Gi", label: "GiB" },
  ],
};
const DEFAULT_UNIT: Record<Resource, string> = { cpu: "", memory: "Gi" };
const SCALE: Record<string, number> = {
  "": 1,
  m: 1e-3,
  k: 1e3,
  M: 1e6,
  G: 1e9,
  T: 1e12,
  Ki: 2 ** 10,
  Mi: 2 ** 20,
  Gi: 2 ** 30,
  Ti: 2 ** 40,
};
// The number part of a Kubernetes quantity.
const NUMBER = /^(\d+\.?\d*|\.\d+)$/;
const FIELDS: [Side, Resource, string][] = [
  ["requests", "cpu", "CPU request"],
  ["limits", "cpu", "CPU limit"],
  ["requests", "memory", "Memory request"],
  ["limits", "memory", "Memory limit"],
];

/**
 * ResourcesEditor edits the CPU and memory requests and limits of a
 * container, each as an amount and a unit. A field left empty is not set.
 * Every other resource, such as ephemeral-storage, and every other key stay
 * unchanged, and a value in a unit that the list does not hold keeps its
 * unit. onChange gets the resources and the errors after each edit: an
 * amount that is not a number, and a limit below its request.
 */
export function ResourcesEditor({
  label,
  defaultValue = {},
  onChange,
}: {
  label: string;
  defaultValue?: Resources;
  onChange?: (value: Resources, errors: ValidationError[]) => void;
}) {
  const id = useId();
  const [amounts, setAmounts] = useState<Record<string, Amount>>(() =>
    Object.fromEntries(
      FIELDS.map(([side, resource]) => [
        `${side}.${resource}`,
        parse(defaultValue[side]?.[resource], resource),
      ]),
    ),
  );
  const errors = amountErrors(amounts);

  const update = (path: string, amount: Amount) => {
    const next = { ...amounts, [path]: amount };
    setAmounts(next);
    onChange?.(toResources(defaultValue, next), amountErrors(next));
  };

  return (
    <fieldset className="grid min-w-0 gap-3 sm:grid-cols-2">
      <legend className="mb-1 text-sm font-medium text-muted">{label}</legend>
      {FIELDS.map(([side, resource, fieldLabel]) => {
        const path = `${side}.${resource}`;
        const value = amounts[path];
        const error = errors.find((e) => e.path === path)?.message;
        const units = UNITS[resource].some((u) => u.value === value.unit)
          ? UNITS[resource]
          : [...UNITS[resource], { value: value.unit, label: value.unit }];
        return (
          <div
            key={path}
            role="group"
            aria-label={fieldLabel}
            className="flex flex-col gap-1"
          >
            <span aria-hidden className="text-sm text-text-2">
              {fieldLabel}
            </span>
            <div className="flex gap-2">
              <Input
                aria-label="Amount"
                inputMode="decimal"
                value={value.amount}
                aria-invalid={error ? true : undefined}
                aria-describedby={error ? `${id}-${path}-error` : undefined}
                onChange={(event) =>
                  update(path, { ...value, amount: event.target.value })
                }
              />
              <label className="sr-only" htmlFor={`${id}-${path}-unit`}>
                Unit
              </label>
              <NativeSelect
                id={`${id}-${path}-unit`}
                className="w-32"
                value={value.unit}
                onChange={(event) =>
                  update(path, { ...value, unit: event.target.value })
                }
              >
                {units.map((u) => (
                  <NativeSelectOption key={u.value} value={u.value}>
                    {u.label}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </div>
            {error && (
              <p id={`${id}-${path}-error`} className="text-xs text-bad">
                {error}
              </p>
            )}
          </div>
        );
      })}
    </fieldset>
  );
}

function parse(
  quantity: string | number | undefined,
  resource: Resource,
): Amount {
  if (quantity === undefined)
    return { amount: "", unit: DEFAULT_UNIT[resource] };
  const match = /^(\d+\.?\d*|\.\d+)([eE][-+]?\d+|[A-Za-z]*)$/.exec(
    String(quantity),
  );
  return match
    ? { amount: match[1], unit: match[2] }
    : { amount: String(quantity), unit: "" };
}

function toResources(
  original: Resources,
  amounts: Record<string, Amount>,
): Resources {
  const result: Resources = { ...original };
  for (const side of ["requests", "limits"] as const) {
    const map: Record<string, string | number> = { ...original[side] };
    for (const resource of ["cpu", "memory"] as const) {
      const value = amounts[`${side}.${resource}`];
      if (value.amount === "") delete map[resource];
      else map[resource] = `${value.amount}${value.unit}`;
    }
    if (Object.keys(map).length) result[side] = map;
    else delete result[side];
  }
  return result;
}

function amountErrors(amounts: Record<string, Amount>): ValidationError[] {
  const errors: ValidationError[] = [];
  for (const [path, value] of Object.entries(amounts)) {
    if (value.amount !== "" && !NUMBER.test(value.amount))
      errors.push({ path, message: "Enter a number" });
  }
  for (const resource of ["cpu", "memory"] as const) {
    const request = base(amounts[`requests.${resource}`]);
    const limit = base(amounts[`limits.${resource}`]);
    if (request !== undefined && limit !== undefined && limit < request)
      errors.push({
        path: `limits.${resource}`,
        message: "The limit is below the request",
      });
  }
  return errors;
}

function base(value: Amount): number | undefined {
  if (!NUMBER.test(value.amount)) return undefined;
  const exponent = /^[eE]([-+]?\d+)$/.exec(value.unit);
  const scale = exponent
    ? 10 ** Number(exponent[1])
    : Object.hasOwn(SCALE, value.unit)
      ? SCALE[value.unit]
      : undefined;
  return scale === undefined ? undefined : Number(value.amount) * scale;
}
