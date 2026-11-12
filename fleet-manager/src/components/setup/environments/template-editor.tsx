"use client";

import { useId, useLayoutEffect, useRef, useState } from "react";
import { stringify } from "yaml";
import { parseYaml } from "@/components/fleet/yaml-editor";
import { Textarea } from "@/components/ui/textarea";
import type { OpenAPISchema, ValidationError } from "@/lib/schema";
import { templateErrors } from "./preview";

const PLACEHOLDERS = [
  "{{ .namespace }}",
  "{{ .claim }}",
  "{{ .request }}",
  "{{ .serviceAccount }}",
];

/**
 * TemplateEditor edits a template parameter as text, as YAML with object,
 * with a chip for each placeholder of a template parameter that inserts it
 * at the cursor. parameters are the names of the other parameters of the
 * registration. It shows the error of each string that is not a valid
 * template parameter at the field. onChange gets the value and those
 * errors, or undefined and the error of YAML that does not parse. Empty
 * text is no value. value is read at mount only.
 */
export function TemplateEditor({
  label,
  value,
  object,
  parameters,
  onChange,
}: {
  label: string;
  value: unknown;
  object: boolean;
  parameters: string[];
  onChange: (value: unknown, errors: ValidationError[]) => void;
}) {
  const id = useId();
  const field = useRef<HTMLTextAreaElement>(null);
  const cursor = useRef<number | undefined>(undefined);
  const [text, setText] = useState(() => textOf(value, object));
  const [, errors] = check(text, object);
  useLayoutEffect(() => {
    if (cursor.current === undefined) return;
    field.current?.setSelectionRange(cursor.current, cursor.current);
    cursor.current = undefined;
  });
  const update = (next: string) => {
    setText(next);
    onChange(...check(next, object));
  };
  const chips = [
    ...PLACEHOLDERS,
    ...parameters.map((p) => `{{ .parameters.${p} }}`),
  ];
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium text-muted">
        {label}
      </label>
      <div className="flex flex-wrap gap-1.5">
        {chips.map((chip) => (
          <button
            key={chip}
            type="button"
            className="rounded-full border border-border bg-panel-2 px-2 py-0.5 font-mono text-xs text-text-2 hover:border-accent hover:text-text"
            onClick={() => {
              const el = field.current;
              const start = el?.selectionStart ?? text.length;
              const end = el?.selectionEnd ?? start;
              cursor.current = start + chip.length;
              update(text.slice(0, start) + chip + text.slice(end));
              el?.focus();
            }}
          >
            {chip}
          </button>
        ))}
      </div>
      <Textarea
        ref={field}
        id={id}
        rows={object ? 8 : 2}
        spellCheck={false}
        value={text}
        className="font-mono text-xs"
        aria-invalid={errors.length > 0 ? true : undefined}
        aria-describedby={errors.length > 0 ? `${id}-errors` : undefined}
        onChange={(event) => update(event.target.value)}
      />
      {errors.length > 0 && (
        <ul
          id={`${id}-errors`}
          className="flex flex-col gap-0.5 text-xs text-bad"
        >
          {errors.map((e, i) => (
            <li key={i}>{e.path ? `${e.path}: ${e.message}` : e.message}</li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** isStructured says whether a template parameter of schema holds an object or a list, which TemplateEditor edits as YAML. */
export function isStructured(schema: OpenAPISchema | undefined): boolean {
  return schema?.type === "object" || schema?.type === "array";
}

function textOf(value: unknown, object: boolean): string {
  if (value === undefined) return "";
  if (object || typeof value !== "string") return stringify(value);
  return value;
}

function check(text: string, object: boolean): [unknown, ValidationError[]] {
  if (text.trim() === "") return [undefined, []];
  if (!object) return [text, templateErrors(text)];
  const [value, errors] = parseYaml(text);
  return value === undefined ? [value, errors] : [value, templateErrors(value)];
}
