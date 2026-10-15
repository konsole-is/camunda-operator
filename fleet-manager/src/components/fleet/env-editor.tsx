"use client";

import { Plus, X } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { ValidationError } from "@/lib/schema";
import { cn } from "@/lib/utils";
import { LockedField } from "./locked-field";
import {
  type SecretKeyRef,
  type SecretRefs,
  SecretRefPicker,
} from "./secret-ref-picker";

/** EnvVar is one entry of extraEnv, as the CRDs define it. */
export type EnvVar = NonNullable<CamundaClusterSpec["extraEnv"]>[number];

type Row =
  | { id: number; source: "value"; name: string; value: string }
  | { id: number; source: "secret"; name: string; ref?: SecretKeyRef }
  | { id: number; source: "other"; entry: EnvVar };

/**
 * EnvEditor edits environment variables as rows of a name with a value or a
 * Secret key. A Secret entry holds only a reference to a key: the editor has
 * no field for a Secret value, and a value typed before the switch to a
 * Secret is dropped. An entry from another source, such as a ConfigMap key
 * or a field of the pod, stays unchanged and read only. onChange gets the
 * entries and the errors after each edit. An error path is
 * "<row>.name" or "<row>.secret", counting rows from 0.
 */
export function EnvEditor({
  label,
  secrets,
  defaultValue = [],
  onChange,
}: {
  label: string;
  secrets: SecretRefs;
  defaultValue?: EnvVar[];
  onChange?: (env: EnvVar[], errors: ValidationError[]) => void;
}) {
  const id = useId();
  const next = useRef(defaultValue.length);
  const [rows, setRows] = useState<Row[]>(() =>
    defaultValue.map((entry, index) => toRow(entry, index)),
  );
  const errors = rowErrors(rows);
  const env = rows.map(toEnv);
  const report = JSON.stringify([env, errors]);
  const reported = useRef(report);
  useEffect(() => {
    if (reported.current === report) return;
    reported.current = report;
    onChange?.(env, errors);
  });

  const update = setRows;
  // Pickers can report in the same commit, so each one changes the latest rows.
  const replace = (row: Row) =>
    setRows((current) => current.map((r) => (r.id === row.id ? row : r)));

  return (
    <fieldset className="flex min-w-0 flex-col gap-3">
      <legend className="mb-1 text-sm font-medium text-muted">{label}</legend>
      {rows.map((row, index) => {
        const nameError = errors.find(
          (e) => e.path === `${index}.name`,
        )?.message;
        const secretError = errors.find(
          (e) => e.path === `${index}.secret`,
        )?.message;
        const errorId = `${id}-${row.id}-error`;
        return (
          <div
            key={row.id}
            role="group"
            aria-label={`Variable ${index + 1}`}
            className={cn(
              "flex flex-col gap-1",
              index > 0 && "border-t border-divider pt-3",
            )}
          >
            <div className="flex flex-wrap items-center gap-2">
              {row.source === "other" ? (
                <LockedField
                  label={row.entry.name}
                  value={describeOther(row.entry)}
                  reason="Edit this variable in the YAML of the claim"
                />
              ) : (
                <>
                  <Input
                    aria-label="Name"
                    placeholder="Name"
                    className="w-full sm:w-48"
                    value={row.name}
                    aria-invalid={nameError ? true : undefined}
                    aria-describedby={
                      nameError || secretError ? errorId : undefined
                    }
                    onChange={(event) =>
                      replace({ ...row, name: event.target.value })
                    }
                  />
                  <label htmlFor={`${id}-${row.id}-source`} className="sr-only">
                    Source
                  </label>
                  <NativeSelect
                    id={`${id}-${row.id}-source`}
                    className="w-40"
                    value={row.source}
                    onChange={(event) =>
                      replace(
                        event.target.value === "secret"
                          ? { id: row.id, source: "secret", name: row.name }
                          : {
                              id: row.id,
                              source: "value",
                              name: row.name,
                              value: "",
                            },
                      )
                    }
                  >
                    <NativeSelectOption value="value">Value</NativeSelectOption>
                    <NativeSelectOption value="secret">
                      From a Secret
                    </NativeSelectOption>
                  </NativeSelect>
                  {row.source === "value" ? (
                    <Input
                      aria-label="Value"
                      placeholder="Value"
                      className="min-w-32 flex-1"
                      value={row.value}
                      onChange={(event) =>
                        replace({ ...row, value: event.target.value })
                      }
                    />
                  ) : (
                    <SecretRefPicker
                      label={`Secret of variable ${index + 1}`}
                      secrets={secrets}
                      defaultValue={row.ref}
                      invalid={!!secretError}
                      onChange={(ref) => replace({ ...row, ref })}
                    />
                  )}
                </>
              )}
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={`Remove variable ${index + 1}`}
                onClick={() => update(rows.filter((r) => r.id !== row.id))}
              >
                <X />
              </Button>
            </div>
            {(nameError || secretError) && (
              <p id={errorId} className="text-xs text-bad">
                {[nameError, secretError].filter(Boolean).join(". ")}
              </p>
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
            { id: next.current++, source: "value", name: "", value: "" },
          ])
        }
      >
        <Plus aria-hidden />
        Add a variable
      </Button>
    </fieldset>
  );
}

function toRow(entry: EnvVar, id: number): Row {
  const from = entry.valueFrom;
  if (!from)
    return { id, source: "value", name: entry.name, value: entry.value ?? "" };
  const ref = from.secretKeyRef;
  if (ref && Object.keys(from).length === 1 && ref.name && !ref.optional) {
    return {
      id,
      source: "secret",
      name: entry.name,
      ref: { name: ref.name, key: ref.key },
    };
  }
  return { id, source: "other", entry };
}

function toEnv(row: Row): EnvVar {
  if (row.source === "other") return row.entry;
  if (row.source === "value") return { name: row.name, value: row.value };
  return row.ref
    ? {
        name: row.name,
        valueFrom: { secretKeyRef: { name: row.ref.name, key: row.ref.key } },
      }
    : { name: row.name };
}

function rowErrors(rows: Row[]): ValidationError[] {
  const errors: ValidationError[] = [];
  const seen = new Set<string>();
  rows.forEach((row, index) => {
    const name = row.source === "other" ? row.entry.name : row.name;
    if (name === "")
      errors.push({ path: `${index}.name`, message: "Enter a name" });
    else if (seen.has(name))
      errors.push({ path: `${index}.name`, message: `${name} is set twice` });
    seen.add(name);
    if (row.source === "secret" && !row.ref)
      errors.push({
        path: `${index}.secret`,
        message: "Pick a Secret and a key",
      });
  });
  return errors;
}

function describeOther(entry: EnvVar): string {
  const from = entry.valueFrom ?? {};
  if (from.configMapKeyRef) return "From a ConfigMap key";
  if (from.secretKeyRef) return "From a Secret key";
  if (from.fieldRef) return "From a field of the pod";
  if (from.resourceFieldRef) return "From a resource of the container";
  return "From another source";
}
