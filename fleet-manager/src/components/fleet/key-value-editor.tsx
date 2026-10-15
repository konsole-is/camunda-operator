"use client";

import { Plus, X } from "lucide-react";
import { useId, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { ValidationError } from "@/lib/schema";
import { isLabelKey, isLabelValue } from "./labels";

type Entry = { id: number; key: string; value: string };

/**
 * KeyValueEditor edits a map of strings, such as pod labels or annotations,
 * as rows of key and value. A row with an empty key and an empty value is
 * ignored. With format "label", keys and values must be valid Kubernetes
 * labels. onChange gets the map and the errors after each edit. An error
 * path is "<row>.key" or "<row>.value", counting rows from 0.
 */
export function KeyValueEditor({
  label,
  defaultValue = {},
  keyLabel = "Key",
  valueLabel = "Value",
  format,
  onChange,
}: {
  label: string;
  defaultValue?: Record<string, string>;
  keyLabel?: string;
  valueLabel?: string;
  format?: "label";
  onChange?: (value: Record<string, string>, errors: ValidationError[]) => void;
}) {
  const id = useId();
  const next = useRef(Object.keys(defaultValue).length);
  const [entries, setEntries] = useState<Entry[]>(() =>
    Object.entries(defaultValue).map(([key, value], index) => ({
      id: index,
      key,
      value,
    })),
  );
  const errors = entryErrors(entries, format);

  const update = (changed: Entry[]) => {
    setEntries(changed);
    onChange?.(
      Object.fromEntries(
        changed.filter((e) => e.key !== "").map((e) => [e.key, e.value]),
      ),
      entryErrors(changed, format),
    );
  };
  const errorAt = (index: number, field: "key" | "value") =>
    errors.find((e) => e.path === `${index}.${field}`)?.message;

  return (
    <fieldset className="flex min-w-0 flex-col gap-2">
      <legend className="mb-1 text-sm font-medium text-muted">{label}</legend>
      {entries.map((entry, index) => {
        const keyError = errorAt(index, "key");
        const valueError = errorAt(index, "value");
        const errorId = `${id}-${entry.id}-error`;
        return (
          <div
            key={entry.id}
            role="group"
            aria-label={`Entry ${index + 1}`}
            className="flex flex-col gap-1"
          >
            <div className="flex items-center gap-2">
              <Input
                aria-label={keyLabel}
                placeholder={keyLabel}
                value={entry.key}
                aria-invalid={keyError ? true : undefined}
                aria-describedby={keyError || valueError ? errorId : undefined}
                onChange={(event) =>
                  update(
                    entries.map((e) =>
                      e.id === entry.id ? { ...e, key: event.target.value } : e,
                    ),
                  )
                }
              />
              <Input
                aria-label={valueLabel}
                placeholder={valueLabel}
                value={entry.value}
                aria-invalid={valueError ? true : undefined}
                aria-describedby={keyError || valueError ? errorId : undefined}
                onChange={(event) =>
                  update(
                    entries.map((e) =>
                      e.id === entry.id
                        ? { ...e, value: event.target.value }
                        : e,
                    ),
                  )
                }
              />
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={`Remove entry ${index + 1}`}
                onClick={() => update(entries.filter((e) => e.id !== entry.id))}
              >
                <X />
              </Button>
            </div>
            {(keyError || valueError) && (
              <p id={errorId} className="text-xs text-bad">
                {[keyError, valueError].filter(Boolean).join(". ")}
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
          update([...entries, { id: next.current++, key: "", value: "" }])
        }
      >
        <Plus aria-hidden />
        Add an entry
      </Button>
    </fieldset>
  );
}

function entryErrors(entries: Entry[], format?: "label"): ValidationError[] {
  const errors: ValidationError[] = [];
  const seen = new Set<string>();
  entries.forEach((entry, index) => {
    if (entry.key === "" && entry.value === "") return;
    if (entry.key === "")
      errors.push({ path: `${index}.key`, message: "Enter a key" });
    else if (seen.has(entry.key))
      errors.push({
        path: `${index}.key`,
        message: `${entry.key} is set twice`,
      });
    else if (format === "label" && !isLabelKey(entry.key))
      errors.push({
        path: `${index}.key`,
        message: "Use a valid label key, for example team or example.com/team",
      });
    if (format === "label" && !isLabelValue(entry.value))
      errors.push({
        path: `${index}.value`,
        message:
          "Use at most 63 letters, digits, '-', '_', or '.', starting and ending with a letter or digit",
      });
    seen.add(entry.key);
  });
  return errors;
}
