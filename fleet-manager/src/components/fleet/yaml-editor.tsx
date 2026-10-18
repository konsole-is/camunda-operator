"use client";

import { useEffect, useId, useRef, useState } from "react";
import { YAMLParseError, parse, stringify } from "yaml";
import { Textarea } from "@/components/ui/textarea";
import {
  type OpenAPISchema,
  type ValidationError,
  validate,
} from "@/lib/schema";

/**
 * YamlEditor edits a value as YAML text. It is for fields without a
 * structured editor: affinity, extraEnvFrom, and a parameter of SectionCards
 * that no other field of SectionCards fits. Empty text is no value. A YAML
 * error names its line. With a schema, the parsed value is validated against
 * it. onChange gets the value and its schema errors, or undefined and the
 * errors while the text has an error that is not of the schema. It is called
 * when the value or the errors change, not at mount. error is an error of
 * the parent, such as a missing value, and shows while the text has no
 * error of its own.
 */
export function YamlEditor({
  label,
  defaultValue,
  schema,
  rows = 8,
  error,
  onChange,
}: {
  label: string;
  defaultValue?: unknown;
  schema?: OpenAPISchema;
  rows?: number;
  error?: string;
  onChange?: (value: unknown, errors: ValidationError[]) => void;
}) {
  const id = useId();
  const [text, setText] = useState(() =>
    defaultValue === undefined ? "" : stringify(defaultValue),
  );
  const [value, errors] = check(text, schema);
  const report = JSON.stringify([value === undefined, value ?? null, errors]);
  const reported = useRef(report);
  useEffect(() => {
    if (reported.current === report) return;
    reported.current = report;
    onChange?.(value, errors);
  });

  const shown = errors.length
    ? errors.map((e) => (e.path ? `${e.path}: ${e.message}` : e.message))
    : error
      ? [error]
      : [];

  return (
    <div className="flex min-w-0 flex-col gap-1">
      <label htmlFor={id} className="text-sm font-medium text-muted">
        {label}
      </label>
      <Textarea
        id={id}
        rows={rows}
        spellCheck={false}
        value={text}
        className="font-mono text-xs"
        aria-invalid={shown.length ? true : undefined}
        aria-describedby={shown.length ? `${id}-errors` : undefined}
        onChange={(event) => {
          setText(event.target.value);
        }}
      />
      {shown.length > 0 && (
        <ul
          id={`${id}-errors`}
          className="flex flex-col gap-0.5 text-xs text-bad"
        >
          {shown.map((message, i) => (
            <li key={i}>{message}</li>
          ))}
        </ul>
      )}
    </div>
  );
}

function check(
  text: string,
  schema?: OpenAPISchema,
): [unknown, ValidationError[]] {
  if (text.trim() === "") return [undefined, []];
  let value: unknown;
  try {
    value = parse(text);
  } catch (error) {
    const line =
      error instanceof YAMLParseError ? error.linePos?.[0]?.line : undefined;
    const message =
      error instanceof Error ? error.message.split("\n")[0] : String(error);
    return [
      undefined,
      [{ path: "", message: line ? `Line ${line}: ${message}` : message }],
    ];
  }
  // The change report serializes the value, which fails on a cyclic alias.
  try {
    JSON.stringify(value);
  } catch {
    return [
      undefined,
      [{ path: "", message: "An alias refers to the value that holds it" }],
    ];
  }
  // A claim travels as JSON, which has no NaN or Infinity.
  if (!finite(value))
    return [
      undefined,
      [{ path: "", message: "Use a finite number, not .nan or .inf" }],
    ];
  if (!schema) return [value, []];
  const result = validate(schema, value);
  return [value, result.ok ? [] : result.errors];
}

function finite(value: unknown): boolean {
  if (typeof value === "number") return Number.isFinite(value);
  if (value === null || typeof value !== "object") return true;
  return Object.values(value).every(finite);
}
