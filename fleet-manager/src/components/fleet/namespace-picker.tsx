"use client";

import { useId, useState } from "react";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import type { ValidationError } from "@/lib/schema";
import { isLabelValue } from "./labels";

/** ENVIRONMENT_LABEL is the label that puts a namespace in an environment (contract 3.1). */
export const ENVIRONMENT_LABEL = "fleet.camunda.io/environment";

export type NamespaceChoice =
  | { kind: "existing"; name: string }
  | { kind: "new"; name: string; labels: Record<string, string> };

/** PickerEnvironment is the part of an Environment that the picker reads. */
export type PickerEnvironment = {
  name: string;
  displayName?: string;
  namespaceCreation?: {
    allowed?: boolean;
    labels?: Record<string, string>;
    requiredLabelKeys?: string[];
  };
};

const DNS_LABEL = /^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$/;

/**
 * NamespacePicker picks a namespace of the environment, or, when the
 * environment allows it, a new one. A new namespace gets the fixed labels
 * of the environment, a value for each required label key, and the
 * environment label, which always wins. Without an environment, it lists
 * every namespace and cannot create one. onChange gets the choice and no
 * errors, or undefined and the errors: a name that is taken or not a DNS
 * label, and a required key without a valid label value.
 */
export function NamespacePicker({
  namespaces,
  environment,
  defaultValue,
  onChange,
}: {
  namespaces: { name: string; environment?: string }[];
  environment?: PickerEnvironment;
  defaultValue?: NamespaceChoice;
  onChange?: (
    choice: NamespaceChoice | undefined,
    errors: ValidationError[],
  ) => void;
}) {
  const id = useId();
  const creation = environment?.namespaceCreation;
  const canCreate = !!creation?.allowed;
  const own = namespaces.filter(
    (ns) => !environment || ns.environment === environment.name,
  );
  const [mode, setMode] = useState<"existing" | "new">(
    defaultValue?.kind === "new" && canCreate ? "new" : "existing",
  );
  const [existing, setExisting] = useState(
    defaultValue?.kind === "existing" ? defaultValue.name : "",
  );
  const [name, setName] = useState(
    defaultValue?.kind === "new" ? defaultValue.name : "",
  );
  const [values, setValues] = useState<Record<string, string>>(
    defaultValue?.kind === "new" ? defaultValue.labels : {},
  );
  const [touched, setTouched] = useState<ReadonlySet<string>>(new Set());

  const evaluate = (
    m: typeof mode,
    ex: string,
    n: string,
    v: Record<string, string>,
  ): [NamespaceChoice | undefined, ValidationError[]] => {
    if (m === "existing")
      return ex
        ? [{ kind: "existing", name: ex }, []]
        : [undefined, [{ path: "name", message: "Pick a namespace" }]];
    const errors: ValidationError[] = [];
    if (!DNS_LABEL.test(n))
      errors.push({
        path: "name",
        message:
          "Use lowercase letters, digits, and '-', at most 63 characters",
      });
    else if (namespaces.some((ns) => ns.name === n))
      errors.push({ path: "name", message: `${n} already exists` });
    for (const key of creation?.requiredLabelKeys ?? []) {
      const value = v[key] ?? "";
      if (value === "")
        errors.push({
          path: `labels.${key}`,
          message: `Enter a value for ${key}`,
        });
      else if (!isLabelValue(value))
        errors.push({
          path: `labels.${key}`,
          message:
            "Use at most 63 letters, digits, '-', '_', or '.', starting and ending with a letter or digit",
        });
    }
    if (errors.length) return [undefined, errors];
    const required = Object.fromEntries(
      (creation?.requiredLabelKeys ?? []).map((k) => [k, v[k]]),
    );
    return [
      {
        kind: "new",
        name: n,
        labels: {
          ...creation?.labels,
          ...required,
          [ENVIRONMENT_LABEL]: environment!.name,
        },
      },
      [],
    ];
  };
  const emit = (
    m: typeof mode,
    ex: string,
    n: string,
    v: Record<string, string>,
  ) => onChange?.(...evaluate(m, ex, n, v));
  const [, errors] = evaluate(mode, existing, name, values);
  const errorAt = (path: string) =>
    touched.has(path)
      ? errors.find((e) => e.path === path)?.message
      : undefined;
  const touch = (path: string) => setTouched(new Set(touched).add(path));

  return (
    <fieldset className="flex min-w-0 flex-col gap-4">
      <legend className="mb-1 font-heading text-base font-semibold">
        Namespace
      </legend>
      <label className="flex items-center gap-2 text-sm">
        <input
          type="radio"
          name={`${id}-mode`}
          checked={mode === "existing"}
          className="accent-[var(--accent)]"
          onChange={() => {
            setMode("existing");
            emit("existing", existing, name, values);
          }}
        />
        Use an existing namespace
      </label>
      {mode === "existing" && (
        <div className="flex flex-col gap-1 pl-6">
          <label htmlFor={`${id}-existing`} className="sr-only">
            Namespace
          </label>
          {own.length === 0 ? (
            <p className="text-sm text-muted">{`No namespace is in ${environment?.displayName ?? environment?.name ?? "the cluster"} yet.`}</p>
          ) : (
            <NativeSelect
              id={`${id}-existing`}
              value={existing}
              onChange={(e) => {
                setExisting(e.target.value);
                emit("existing", e.target.value, name, values);
              }}
            >
              <NativeSelectOption value="">Pick a namespace</NativeSelectOption>
              {own.map((ns) => (
                <NativeSelectOption key={ns.name} value={ns.name}>
                  {ns.name}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          )}
        </div>
      )}
      {canCreate && (
        <label className="flex items-center gap-2 text-sm">
          <input
            type="radio"
            name={`${id}-mode`}
            checked={mode === "new"}
            className="accent-[var(--accent)]"
            onChange={() => {
              setMode("new");
              emit("new", existing, name, values);
            }}
          />
          Create a namespace
        </label>
      )}
      {canCreate && mode === "new" && (
        <div className="flex flex-col gap-3 pl-6">
          <Field
            id={`${id}-name`}
            label="Name"
            value={name}
            error={errorAt("name")}
            onValue={(v) => {
              setName(v);
              touch("name");
              emit("new", existing, v, values);
            }}
          />
          {(creation?.requiredLabelKeys ?? []).map((key) => (
            <Field
              key={key}
              id={`${id}-label-${key}`}
              label={key}
              value={values[key] ?? ""}
              error={errorAt(`labels.${key}`)}
              onValue={(v) => {
                const next = { ...values, [key]: v };
                setValues(next);
                touch(`labels.${key}`);
                emit("new", existing, name, next);
              }}
            />
          ))}
          <div className="text-xs text-muted">
            <p className="mb-1">The new namespace also gets these labels:</p>
            <ul className="font-mono">
              {Object.entries({
                ...creation?.labels,
                [ENVIRONMENT_LABEL]: environment!.name,
              }).map(([k, v]) => (
                <li key={k}>{`${k}=${v}`}</li>
              ))}
            </ul>
          </div>
        </div>
      )}
    </fieldset>
  );
}

function Field({
  id,
  label,
  value,
  error,
  onValue,
}: {
  id: string;
  label: string;
  value: string;
  error?: string;
  onValue: (value: string) => void;
}) {
  return (
    <div className="flex max-w-sm flex-col gap-1">
      <label htmlFor={id} className="text-sm text-muted">
        {label}
      </label>
      <Input
        id={id}
        value={value}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${id}-error` : undefined}
        onChange={(e) => onValue(e.target.value)}
      />
      {error && (
        <p id={`${id}-error`} className="text-xs text-bad">
          {error}
        </p>
      )}
    </div>
  );
}
