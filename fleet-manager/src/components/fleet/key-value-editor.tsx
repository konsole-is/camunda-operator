"use client";

import { Plus, X } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { ValidationError } from "@/lib/schema";
import { isLabelKey, isLabelValue } from "@/lib/labels";

type Entry = { id: number; key: string; value: string };

/**
 * EmptyMap says what an editor with no rows means, where the user can also
 * set the map to {}: "inherited" when an inherited value then applies, and
 * "unset" when the map then has no value.
 */
export type EmptyMap = "inherited" | "unset";

/**
 * KeyValueEditor edits a map of strings, such as pod labels or annotations,
 * as rows of key and value. A row with an empty key and an empty value is
 * ignored. With format "label", keys and values must be valid Kubernetes
 * labels. onChange gets the map and the errors after each edit, and
 * undefined in place of the map while no row has a key. With emptyMap,
 * the user can also set the map to {}, which onChange gets as {}, and a
 * defaultValue of {} starts as that empty map. An error path is
 * "<row>.key" or "<row>.value", counting rows from 0. describedBy names
 * the hint and error elements of the field for each input.
 */
export function KeyValueEditor({
  label,
  defaultValue,
  keyLabel = "Key",
  valueLabel = "Value",
  format,
  emptyMap,
  describedBy,
  onChange,
}: {
  label: string;
  defaultValue?: Record<string, string>;
  keyLabel?: string;
  valueLabel?: string;
  format?: "label";
  emptyMap?: EmptyMap;
  describedBy?: string;
  onChange?: (
    value: Record<string, string> | undefined,
    errors: ValidationError[],
  ) => void;
}) {
  const id = useId();
  const start = defaultValue ?? {};
  const next = useRef(Object.keys(start).length);
  const [entries, setEntries] = useState<Entry[]>(() =>
    Object.entries(start).map(([key, value], index) => ({
      id: index,
      key,
      value,
    })),
  );
  const [empty, setEmpty] = useState(
    () =>
      !!emptyMap && !!defaultValue && Object.keys(defaultValue).length === 0,
  );
  const setButton = useRef<HTMLButtonElement>(null);
  const undoButton = useRef<HTMLButtonElement>(null);
  // The button that the user clicked goes away, so focus moves to the one
  // that takes its place.
  const moveFocus = useRef(false);
  useEffect(() => {
    if (!moveFocus.current) return;
    moveFocus.current = false;
    (empty ? undoButton : setButton).current?.focus();
  }, [empty]);
  const errors = keyValueErrors(entries, format);

  const update = (changed: Entry[], changedEmpty = false) => {
    setEntries(changed);
    setEmpty(changedEmpty);
    const map = Object.fromEntries(
      changed.filter((e) => e.key !== "").map((e) => [e.key, e.value]),
    );
    onChange?.(
      changedEmpty || Object.keys(map).length > 0 ? map : undefined,
      keyValueErrors(changed, format),
    );
  };
  const setEmptyMap = (on: boolean) => {
    moveFocus.current = true;
    update([], on);
  };
  const errorAt = (index: number, field: "key" | "value") =>
    errors.find((e) => e.path === `${index}.${field}`)?.message;
  const describe = (...ids: (string | false | undefined)[]) =>
    ids.filter(Boolean).join(" ") || undefined;

  return (
    <fieldset className="flex min-w-0 flex-col gap-2">
      <legend className="mb-1 text-sm font-medium text-muted">{label}</legend>
      {entries.map((entry, index) => {
        const keyError = errorAt(index, "key");
        const valueError = errorAt(index, "value");
        const errorId = `${id}-${entry.id}-error`;
        const described = describe(
          (keyError || valueError) && errorId,
          describedBy,
        );
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
                aria-describedby={described}
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
                aria-describedby={described}
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
      {empty && (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-md border border-dashed border-border px-3 py-2 text-sm">
          <code aria-hidden className="font-mono text-text">
            {"{}"}
          </code>
          <span id={`${id}-empty`} className="text-text-2">
            {emptyMap === "inherited"
              ? "Empty map, overrides the inherited value"
              : "Empty map"}
          </span>
          <Button
            ref={undoButton}
            variant="link"
            size="xs"
            className="h-auto px-0"
            aria-describedby={`${id}-empty`}
            onClick={() => setEmptyMap(false)}
          >
            {emptyMap === "inherited" ? "Use the inherited value" : "Clear"}
          </Button>
        </div>
      )}
      {emptyMap === "inherited" && !empty && entries.length === 0 && (
        <p className="text-xs text-muted">
          No entries. The inherited value applies.
        </p>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          onClick={() =>
            update([...entries, { id: next.current++, key: "", value: "" }])
          }
        >
          <Plus aria-hidden />
          Add an entry
        </Button>
        {emptyMap && !empty && entries.length === 0 && (
          <Button
            ref={setButton}
            variant="ghost"
            size="sm"
            onClick={() => setEmptyMap(true)}
          >
            Set to an empty map
          </Button>
        )}
      </div>
    </fieldset>
  );
}

/**
 * keyValueErrors returns the errors that KeyValueEditor shows for entries,
 * with the paths "<row>.key" and "<row>.value". A row with an empty key and
 * an empty value has none.
 */
export function keyValueErrors(
  entries: readonly { key: string; value: string }[],
  format?: "label",
): ValidationError[] {
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
