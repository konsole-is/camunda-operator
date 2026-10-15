"use client";

import { Plus, X } from "lucide-react";
import type React from "react";
import { useId, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { ValidationError } from "@/lib/schema";

/** Toleration is one toleration of the pods, as the CRDs define it. */
export type Toleration = NonNullable<
  NonNullable<CamundaClusterSpec["scheduling"]>["tolerations"]
>[number];

type Row = {
  id: number;
  key: string;
  operator: "Equal" | "Exists";
  value: string;
  effect: string;
  seconds: string;
};

/**
 * SchedulingEditor edits the tolerations of the pods as rows of key,
 * operator, value, effect, and seconds. The value shows only for Equal, and
 * the seconds only for NoExecute. A field that the row does not use is not
 * written. onChange gets the tolerations and the errors after each edit: an
 * Equal row needs a key, and the seconds must be a whole number.
 */
export function SchedulingEditor({
  label,
  defaultValue = [],
  onChange,
}: {
  label: string;
  defaultValue?: Toleration[];
  onChange?: (value: Toleration[], errors: ValidationError[]) => void;
}) {
  const id = useId();
  const next = useRef(defaultValue.length);
  const [rows, setRows] = useState<Row[]>(() =>
    defaultValue.map((t, index) => ({
      id: index,
      key: t.key ?? "",
      operator: t.operator === "Exists" ? "Exists" : "Equal",
      value: t.value ?? "",
      effect: t.effect ?? "",
      seconds:
        t.tolerationSeconds === undefined ? "" : String(t.tolerationSeconds),
    })),
  );
  const update = (changed: Row[]) => {
    setRows(changed);
    onChange?.(changed.map(toToleration), rowErrors(changed));
  };
  const replace = (row: Row) =>
    update(rows.map((r) => (r.id === row.id ? row : r)));
  const errors = rowErrors(rows);

  return (
    <fieldset className="flex min-w-0 flex-col gap-3">
      <legend className="mb-1 text-sm font-medium text-muted">{label}</legend>
      {rows.map((row, index) => {
        const rowErrorsText = errors
          .filter((e) => e.path.startsWith(`${index}.`))
          .map((e) => e.message)
          .join(". ");
        const keyInvalid =
          errors.some((e) => e.path === `${index}.key`) || undefined;
        const at = (field: string) => `${id}-${row.id}-${field}`;
        const select = (
          field: string,
          name: string,
          value: string,
          options: [string, string][],
          onPick: (v: string) => void,
        ) => (
          <div className="flex flex-col gap-1">
            <label htmlFor={at(field)} className="text-xs text-muted">
              {name}
            </label>
            <NativeSelect
              id={at(field)}
              value={value}
              onChange={(event) => onPick(event.target.value)}
            >
              {options.map(([v, l]) => (
                <NativeSelectOption key={v} value={v}>
                  {l}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>
        );
        const text = (
          field: string,
          name: string,
          value: string,
          onText: (v: string) => void,
          extra: React.ComponentProps<typeof Input> = {},
        ) => (
          <div className="flex flex-col gap-1">
            <label htmlFor={at(field)} className="text-xs text-muted">
              {name}
            </label>
            <Input
              id={at(field)}
              value={value}
              onChange={(event) => onText(event.target.value)}
              {...extra}
            />
          </div>
        );
        return (
          <div
            key={row.id}
            role="group"
            aria-label={`Toleration ${index + 1}`}
            className="flex flex-col gap-1"
          >
            <div className="flex flex-wrap items-end gap-2">
              {text("key", "Key", row.key, (v) => replace({ ...row, key: v }), {
                "aria-invalid": keyInvalid,
              })}
              {select(
                "operator",
                "Operator",
                row.operator,
                [
                  ["Equal", "Equal"],
                  ["Exists", "Exists"],
                ],
                (v) =>
                  replace({
                    ...row,
                    operator: v as Row["operator"],
                    value: "",
                  }),
              )}
              {row.operator === "Equal" &&
                text("value", "Value", row.value, (v) =>
                  replace({ ...row, value: v }),
                )}
              {select(
                "effect",
                "Effect",
                row.effect,
                [
                  ["", "Any effect"],
                  ["NoSchedule", "NoSchedule"],
                  ["PreferNoSchedule", "PreferNoSchedule"],
                  ["NoExecute", "NoExecute"],
                ],
                (v) =>
                  replace({
                    ...row,
                    effect: v,
                    seconds: v === "NoExecute" ? row.seconds : "",
                  }),
              )}
              {row.effect === "NoExecute" &&
                text(
                  "seconds",
                  "Seconds",
                  row.seconds,
                  (v) => replace({ ...row, seconds: v }),
                  { type: "number", min: 0, step: 1, className: "w-28" },
                )}
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={`Remove toleration ${index + 1}`}
                onClick={() => update(rows.filter((r) => r.id !== row.id))}
              >
                <X />
              </Button>
            </div>
            {rowErrorsText && (
              <p className="text-xs text-bad">{rowErrorsText}</p>
            )}
          </div>
        );
      })}
      <Button
        variant="outline"
        size="sm"
        className="self-start"
        onClick={() =>
          update([
            ...rows,
            {
              id: next.current++,
              key: "",
              operator: "Equal",
              value: "",
              effect: "",
              seconds: "",
            },
          ])
        }
      >
        <Plus aria-hidden />
        Add a toleration
      </Button>
    </fieldset>
  );
}

function toToleration(row: Row): Toleration {
  return {
    ...(row.key && { key: row.key }),
    operator: row.operator,
    ...(row.operator === "Equal" && row.value && { value: row.value }),
    ...(row.effect && { effect: row.effect }),
    ...(row.effect === "NoExecute" &&
      /^\d+$/.test(row.seconds) && { tolerationSeconds: Number(row.seconds) }),
  };
}

function rowErrors(rows: Row[]): ValidationError[] {
  const errors: ValidationError[] = [];
  rows.forEach((row, index) => {
    if (row.operator === "Equal" && row.key === "")
      errors.push({
        path: `${index}.key`,
        message: "Enter a key, or use the operator Exists",
      });
    if (
      row.effect === "NoExecute" &&
      row.seconds !== "" &&
      !/^\d+$/.test(row.seconds)
    )
      errors.push({
        path: `${index}.tolerationSeconds`,
        message: "Enter whole seconds",
      });
  });
  return errors;
}
