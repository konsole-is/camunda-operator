"use client";

import { useEffect, useId, useRef, useState } from "react";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";

/** SecretKeyRef names one key of a Secret in the namespace of the object. */
export type SecretKeyRef = { name: string; key: string };

/** SecretRefs are the Secrets of a namespace with their key names, never their values. */
export type SecretRefs = { name: string; keys: string[] }[];

/** secretRefsOf returns the name and the key names of each Secret, and nothing else. */
export function secretRefsOf(
  items: readonly { metadata?: { name?: string }; keys: string[] }[],
): SecretRefs {
  return items.map((s) => ({
    name: s.metadata?.name ?? "",
    keys: [...s.keys],
  }));
}

/**
 * SecretRefPicker picks a Secret and one of its keys. It shows names only:
 * it never has a Secret value. onChange gets the reference when both are
 * picked, and undefined otherwise. When secrets no longer hold the picked
 * Secret or key, onChange gets undefined if its last value was a reference.
 * describedBy gives the ids for aria-describedby of both selects. Without
 * secrets it shows the reference of defaultValue, when it has one, and
 * emptyText.
 */
export function SecretRefPicker({
  label,
  secrets,
  defaultValue,
  invalid = false,
  describedBy,
  emptyText = "No Secrets in this namespace",
  onChange,
}: {
  label: string;
  secrets: SecretRefs;
  defaultValue?: SecretKeyRef;
  invalid?: boolean;
  describedBy?: string;
  emptyText?: string;
  onChange?: (ref: SecretKeyRef | undefined) => void;
}) {
  const id = useId();
  const [name, setName] = useState(defaultValue?.name ?? "");
  const [key, setKey] = useState(defaultValue?.key ?? "");
  const activeName = secrets.some((s) => s.name === name) ? name : "";
  const keys = secrets.find((s) => s.name === activeName)?.keys ?? [];
  const activeKey = keys.includes(key) ? key : "";
  const ref =
    activeName && activeKey ? { name: activeName, key: activeKey } : undefined;
  const report = JSON.stringify(ref ?? null);
  const reported = useRef(
    JSON.stringify(
      defaultValue?.name && defaultValue.key
        ? { name: defaultValue.name, key: defaultValue.key }
        : null,
    ),
  );
  useEffect(() => {
    if (reported.current === report) return;
    reported.current = report;
    onChange?.(ref);
  });

  if (secrets.length === 0) {
    return (
      <div className="flex flex-col gap-0.5 text-sm">
        {defaultValue?.name && defaultValue.key && (
          <span className="font-mono break-all text-text-2">
            {defaultValue.name}/{defaultValue.key}
          </span>
        )}
        <p className="text-muted">{emptyText}</p>
      </div>
    );
  }

  return (
    <div role="group" aria-label={label} className="flex min-w-48 flex-1 gap-2">
      <label htmlFor={`${id}-secret`} className="sr-only">
        Secret
      </label>
      <NativeSelect
        id={`${id}-secret`}
        value={activeName}
        aria-invalid={invalid && !activeName ? true : undefined}
        aria-describedby={describedBy}
        onChange={(event) => {
          setName(event.target.value);
          setKey("");
        }}
      >
        <NativeSelectOption value="">Pick a Secret</NativeSelectOption>
        {secrets.map((s) => (
          <NativeSelectOption key={s.name} value={s.name}>
            {s.name}
          </NativeSelectOption>
        ))}
      </NativeSelect>
      <label htmlFor={`${id}-key`} className="sr-only">
        Key
      </label>
      <NativeSelect
        id={`${id}-key`}
        value={activeKey}
        disabled={!activeName}
        aria-invalid={invalid && !activeKey ? true : undefined}
        aria-describedby={describedBy}
        onChange={(event) => {
          setKey(event.target.value);
        }}
      >
        <NativeSelectOption value="">Pick a key</NativeSelectOption>
        {keys.map((k) => (
          <NativeSelectOption key={k} value={k}>
            {k}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    </div>
  );
}
