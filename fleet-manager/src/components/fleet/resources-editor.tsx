"use client";

import { useId, useState } from "react";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { ValidationError } from "@/lib/schema";
import { useErrorsReport } from "./errors-report";

/** Resources are the requests and limits of a container, as the CRDs define them. */
export type Resources = NonNullable<
  NonNullable<CamundaClusterSpec["gateway"]>["resources"]
>;

/**
 * LimitBelowRequest is a resource whose limit is below its request, such as
 * cpu or ephemeral-storage. request and limit are the quantities as the
 * value writes them.
 */
export type LimitBelowRequest = {
  resource: string;
  request: string;
  limit: string;
};

/**
 * AmountWrite is what an edit of one amount writes to the claim: a value at
 * the key of the amount, or, as undefined, a reset of that key.
 */
export type AmountWrite = { value: string | number } | undefined;

/** EDITED_RESOURCES are the resources that ResourcesEditor has fields for. */
export const EDITED_RESOURCES: readonly string[] = ["cpu", "memory"];

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
  P: 1e15,
  E: 1e18,
  n: 1e-9,
  u: 1e-6,
  Ki: 2 ** 10,
  Mi: 2 ** 20,
  Gi: 2 ** 30,
  Ti: 2 ** 40,
  Pi: 2 ** 50,
  Ei: 2 ** 60,
};
// The signed number part of a Kubernetes quantity.
const NUMBER = /^[+-]?(\d+\.?\d*|\.\d+)$/;
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
 * amount that is not a number, a negative amount, and a limit below its
 * request. describedBy names the hint and error elements of the field for
 * each amount and unit. With limitsChecked, the caller checks each limit
 * against its request in the value that the cluster gets: the editor then
 * neither shows nor reports a limit below its request, and passes such an
 * edit on. onErrors gets the errors that it shows, also at mount.
 *
 * With onAmounts, the editor reports amounts in place of the whole value:
 * after each edit, the quantity of each field that the user typed in since
 * the last edit without errors, by its path such as requests.memory, with
 * the errors. An empty field gives "". The caller writes them only when
 * errors is empty.
 */
export function ResourcesEditor({
  label,
  defaultValue = {},
  describedBy,
  limitsChecked = false,
  onChange,
  onAmounts,
  onErrors,
}: {
  label: string;
  defaultValue?: Resources;
  describedBy?: string;
  limitsChecked?: boolean;
  onChange?: (value: Resources, errors: ValidationError[]) => void;
  onAmounts?: (
    amounts: Readonly<Record<string, string>>,
    errors: ValidationError[],
  ) => void;
  onErrors?: (errors: ValidationError[]) => void;
}) {
  const id = useId();
  const [amounts, setAmounts] = useState(() => amountsOf(defaultValue));
  const [unwritten, setUnwritten] = useState<ReadonlySet<string>>(new Set());
  const errors = amountErrors(amounts, limitsChecked);
  useErrorsReport(errors, onErrors);

  const update = (path: string, amount: Amount) => {
    const next = { ...amounts, [path]: amount };
    const nextErrors = amountErrors(next, limitsChecked);
    setAmounts(next);
    if (onAmounts) {
      const typed = new Set(unwritten).add(path);
      setUnwritten(nextErrors.length === 0 ? new Set() : typed);
      onAmounts(
        Object.fromEntries([...typed].map((p) => [p, quantityOf(next[p])])),
        nextErrors,
      );
    } else onChange?.(toResources(defaultValue, next), nextErrors);
  };

  return (
    <fieldset className="grid min-w-0 gap-3 sm:grid-cols-2">
      <legend className="mb-1 text-sm font-medium text-muted">{label}</legend>
      {FIELDS.map(([side, resource, fieldLabel]) => {
        const path = `${side}.${resource}`;
        const value = amounts[path];
        const error = errors.find((e) => e.path === path)?.message;
        const described =
          [error && `${id}-${path}-error`, describedBy]
            .filter(Boolean)
            .join(" ") || undefined;
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
                aria-describedby={described}
                onChange={(event) =>
                  update(path, { ...value, amount: event.target.value })
                }
              />
              <label className="sr-only" htmlFor={`${id}-${path}-unit`}>
                Unit
              </label>
              <NativeSelect
                id={`${id}-${path}-unit`}
                className="w-32 shrink-0"
                value={value.unit}
                aria-describedby={described}
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

function amountsOf(value: Resources): Record<string, Amount> {
  return Object.fromEntries(
    FIELDS.map(([side, resource]) => [
      `${side}.${resource}`,
      parse(value[side]?.[resource], DEFAULT_UNIT[resource]),
    ]),
  );
}

// An undefined quantity is an empty amount in emptyUnit.
function parse(quantity: string | number | undefined, emptyUnit = ""): Amount {
  if (quantity === undefined) return { amount: "", unit: emptyUnit };
  const match = /^([+-]?(?:\d+\.?\d*|\.\d+))([eE][-+]?\d+|[A-Za-z]*)$/.exec(
    String(quantity),
  );
  return match
    ? { amount: match[1], unit: match[2] }
    : { amount: String(quantity), unit: "" };
}

function quantityOf(value: Amount): string {
  return value.amount === "" ? "" : `${value.amount}${value.unit}`;
}

function toResources(
  original: Resources,
  amounts: Record<string, Amount>,
): Resources {
  const result: Resources = { ...original };
  for (const side of ["requests", "limits"] as const) {
    const map: Record<string, string | number> = { ...original[side] };
    for (const resource of ["cpu", "memory"] as const) {
      const quantity = quantityOf(amounts[`${side}.${resource}`]);
      if (quantity === "") delete map[resource];
      else map[resource] = quantity;
    }
    if (Object.keys(map).length) result[side] = map;
    else delete result[side];
  }
  return result;
}

/**
 * amountWrite returns what an edit of one amount writes. quantity is what
 * the field shows, "" when it is empty. claim is the value of the key in
 * the claim without a pending change of the key itself, below is its
 * merged value in the layers under the claim, and saved its value in the
 * claim before the pending changes. A quantity equal in value to claim, or
 * to below when claim is undefined (1, "1" and 1000m are equal), writes
 * claim as it is, or a reset when claim is undefined, so that it leaves no
 * change. An empty quantity writes a reset. A quantity equal in value to
 * saved writes saved as it is. Any other quantity writes itself.
 */
export function amountWrite(
  quantity: string,
  claim: string | number | undefined,
  below: string | number | undefined,
  saved: string | number | undefined,
): AmountWrite {
  if (sameAmount(parse(quantity), parse(claim ?? below)))
    return claim === undefined ? undefined : { value: claim };
  if (quantity === "") return undefined;
  if (saved !== undefined && sameAmount(parse(quantity), parse(saved)))
    return { value: saved };
  return { value: quantity };
}

// sameAmount says whether a and b are one quantity. Two empty amounts are
// one, and an amount that is not a number equals only the same text.
function sameAmount(a: Amount, b: Amount): boolean {
  if (a.amount === "" || b.amount === "") return a.amount === b.amount;
  const x = base(a);
  const y = base(b);
  if (x === undefined || y === undefined)
    return a.amount + a.unit === b.amount + b.unit;
  return close(x, y);
}

function amountErrors(
  amounts: Record<string, Amount>,
  limitsChecked: boolean,
): ValidationError[] {
  const errors: ValidationError[] = [];
  for (const [path, value] of Object.entries(amounts)) {
    if (value.amount === "") continue;
    if (!NUMBER.test(value.amount))
      errors.push({ path, message: "Enter a number" });
    // Kubernetes refuses a negative request or limit.
    else if (value.amount.startsWith("-"))
      errors.push({ path, message: "Enter 0 or more" });
  }
  if (limitsChecked) return errors;
  for (const resource of ["cpu", "memory"] as const)
    if (isBelow(amounts[`requests.${resource}`], amounts[`limits.${resource}`]))
      errors.push({
        path: `limits.${resource}`,
        message: "The limit is below the request",
      });
  return errors;
}

/**
 * limitsBelowRequests returns each resource of value whose limit is below
 * its request, with the rule that the editor shows: the CPU first, then the
 * memory, then the others in the order of the requests. A quantity that is
 * not a number in a unit of Kubernetes is below nothing.
 */
export function limitsBelowRequests(value: Resources): LimitBelowRequest[] {
  const requests = value.requests ?? {};
  const limits = value.limits ?? {};
  const keys = Object.keys(requests).sort((a, b) => rank(a) - rank(b));
  return keys.flatMap((resource) => {
    const request = requests[resource];
    const limit = limits[resource];
    if (limit === undefined) return [];
    return isBelow(parse(request), parse(limit))
      ? [{ resource, request: String(request), limit: String(limit) }]
      : [];
  });
}

function rank(resource: string): number {
  const at = EDITED_RESOURCES.indexOf(resource);
  return at < 0 ? 2 : at;
}

function isBelow(request: Amount, limit: Amount): boolean {
  const r = base(request);
  const l = base(limit);
  return r !== undefined && l !== undefined && l < r && !close(l, r);
}

// close says whether a and b, in base units, are one quantity. Float scales
// make equal quantities differ in the last digit, such as 700m and 0.7.
function close(a: number, b: number): boolean {
  return Math.abs(a - b) <= Math.abs(b) * 1e-12;
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
