"use client";

import { useState } from "react";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import {
  claimValueOf,
  editorValueOf,
  parameterEditorOf,
  type SchemaEditor,
} from "@/lib/fields";
import type { ParameterRule } from "@/lib/policy/types";
import {
  type SchemaRule,
  type ValidationError,
  schemaRules,
} from "@/lib/schema";
import { cn } from "@/lib/utils";
import { EnvEditor, type EnvVar } from "./env-editor";
import { KeyValueEditor } from "./key-value-editor";
import { LockedField } from "./locked-field";
import { ResourcesEditor, type Resources } from "./resources-editor";
import { SchedulingEditor, type Toleration } from "./scheduling-editor";
import type { SecretRefs } from "./secret-ref-picker";
import { StorageSizeEditor } from "./storage-size-editor";
import { YamlEditor } from "./yaml-editor";

/**
 * ParameterField is the field of one parameter rule with value. A locked
 * rule shows its summary as set by the admin. An editable rule gets the
 * editor that parameterEditorOf picks, with the allowed names shown by
 * their labels. onValue gets undefined for a cleared field, but "" for a
 * cleared text field and [] for choices with none checked. It also gets the
 * errors of an editor that shows its own, but from YamlEditor only those of
 * text that does not parse. error shows while the editor shows no error of
 * its own.
 */
export function ParameterField({
  id,
  rule,
  value,
  error,
  onValue,
}: {
  id: string;
  rule: ParameterRule;
  value: unknown;
  error?: string;
  onValue: (value: unknown, parse?: ValidationError[]) => void;
}) {
  const [rowErrors, setRowErrors] = useState(false);
  // YAML can give a value that rows read, and a swap to rows would drop the
  // text that the user types.
  const [mounted] = useState(value);
  const schema = rule.schema ?? {};
  const rules = rule.claimEditable ? schemaRules(schema) : [];
  const hints = <RuleHints id={`${id}-rules`} rules={rules} />;
  const describedBy =
    [error && `${id}-error`, rules.length > 0 && `${id}-rules`]
      .filter(Boolean)
      .join(" ") || undefined;
  const invalid = error ? true : undefined;
  const message = error ? (
    <p id={`${id}-error`} className="text-xs text-bad">
      {error}
    </p>
  ) : null;
  const label = (
    <label htmlFor={id} className="text-sm text-muted">
      {rule.label}
    </label>
  );

  if (!rule.claimEditable) {
    return (
      <div className="flex flex-col gap-1">
        <LockedField
          label={rule.label}
          value={rule.summary}
          reason="Set by your admin"
        />
        {rule.note && <p className="text-xs text-muted">{rule.note}</p>}
      </div>
    );
  }
  // A claim stores one name for a list whose choices are each a whole list.
  // A schema always gives an editor.
  const editor = parameterEditorOf(
    rule.name,
    rule.allowedLists ? { type: "string" } : schema,
    { choices: rule.allowed?.map((a) => a.name), value: mounted },
  )!;
  const labelOf = (name: string, index: number) =>
    rule.allowed?.[index]?.label ?? (name === "" ? "Empty" : name);

  switch (editor.kind) {
    case "choices": {
      const names = Array.isArray(value) ? (value as string[]) : [];
      return (
        <fieldset
          aria-label={rule.label}
          aria-describedby={describedBy}
          className="flex flex-col gap-2"
        >
          <legend className="mb-1 text-sm text-muted">{rule.label}</legend>
          {editor.options.map((name, index) => (
            <div
              key={name}
              className="flex items-center gap-2 text-sm text-text-2"
            >
              <Checkbox
                id={`${id}-choice-${index}`}
                aria-label={labelOf(name, index)}
                checked={names.includes(name)}
                onCheckedChange={(on) =>
                  onValue(
                    editor.options.filter((n) =>
                      n === name ? on === true : names.includes(n),
                    ),
                  )
                }
              />
              <label htmlFor={`${id}-choice-${index}`}>
                {labelOf(name, index)}
              </label>
            </div>
          ))}
          {message}
          {hints}
        </fieldset>
      );
    }
    case "select": {
      // The options use their position as the value, so an enum value of ""
      // does not collide with the placeholder.
      const picked = editor.options.indexOf(value as string);
      return (
        <div className="flex max-w-sm flex-col gap-1">
          {label}
          <NativeSelect
            id={id}
            value={picked === -1 ? "" : String(picked)}
            aria-invalid={invalid}
            aria-describedby={describedBy}
            onChange={(e) =>
              onValue(
                e.target.value === ""
                  ? undefined
                  : editor.options[Number(e.target.value)],
              )
            }
          >
            <NativeSelectOption value="">Choose one</NativeSelectOption>
            {editor.options.map((name, index) => (
              <NativeSelectOption key={name} value={String(index)}>
                {labelOf(name, index)}
              </NativeSelectOption>
            ))}
          </NativeSelect>
          {message}
          {hints}
        </div>
      );
    }
    case "toggle":
      return (
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <Checkbox
              id={id}
              checked={value === true}
              aria-invalid={invalid}
              aria-describedby={describedBy}
              onCheckedChange={(on) => onValue(on === true)}
            />
            {label}
            {message}
          </div>
          {hints}
        </div>
      );
    case "number": {
      const { minimum: min, maximum: max } = editor;
      return (
        <div className="flex max-w-48 flex-col gap-1">
          {label}
          <Input
            id={id}
            type="number"
            min={min}
            max={max}
            step={editor.integer ? 1 : "any"}
            value={value === undefined ? "" : String(value)}
            aria-invalid={invalid}
            aria-describedby={describedBy}
            onChange={(e) =>
              onValue(
                e.target.value === "" ? undefined : Number(e.target.value),
              )
            }
          />
          {min !== undefined && max !== undefined && !error && (
            <p className="text-xs text-muted">{`From ${min} to ${max}`}</p>
          )}
          {message}
          {hints}
        </div>
      );
    }
    case "text":
      return (
        <div className="flex max-w-sm flex-col gap-1">
          {label}
          <Input
            id={id}
            value={typeof value === "string" ? value : ""}
            aria-invalid={invalid}
            aria-describedby={describedBy}
            onChange={(e) => onValue(e.target.value)}
          />
          {message}
          {hints}
        </div>
      );
    case "yaml":
      return (
        <div className="flex min-w-0 flex-col gap-1">
          <YamlEditor
            label={rule.label}
            defaultValue={value}
            schema={editor.schema}
            error={error}
            describedBy={rules.length > 0 ? `${id}-rules` : undefined}
            onChange={(v, found) => onValue(v, v === undefined ? found : [])}
          />
          {hints}
        </div>
      );
    default:
      return (
        <div className="flex min-w-0 flex-col gap-1">
          <RowsEditor
            label={rule.label}
            editor={editor}
            value={value}
            onChange={(v, errors) => {
              setRowErrors(errors.length > 0);
              onValue(claimValueOf(editor, v), errors);
            }}
          />
          {!rowErrors && message}
          {hints}
        </div>
      );
  }
}

// RowsEditor is the input of an editor kind that shows the errors of its
// own rows.
function RowsEditor({
  label,
  editor,
  value,
  onChange,
}: {
  label: string;
  editor: Extract<
    SchemaEditor,
    { kind: "keyValue" | "resources" | "storageSize" | "env" | "tolerations" }
  >;
  value: unknown;
  onChange: (value: unknown, errors: ValidationError[]) => void;
}) {
  const initial = editorValueOf(editor, value);
  // An edit that clears a Secret key changes value, and the Secret must
  // stay on offer.
  const [secrets] = useState(() =>
    editor.kind === "env" ? secretKeysOf(initial as EnvVar[]) : [],
  );
  switch (editor.kind) {
    case "keyValue":
      return (
        <KeyValueEditor
          label={label}
          format={editor.format}
          defaultValue={initial as Record<string, string>}
          onChange={onChange}
        />
      );
    case "resources":
      return (
        <ResourcesEditor
          label={label}
          defaultValue={initial as Resources}
          onChange={onChange}
        />
      );
    case "storageSize":
      return (
        <StorageSizeEditor
          label={label}
          defaultValue={String(initial)}
          onChange={onChange}
        />
      );
    case "env":
      return (
        <EnvEditor
          label={label}
          // SecretRefPicker drops a reference to a Secret that secrets lack.
          secrets={secrets}
          defaultValue={initial as EnvVar[]}
          onChange={onChange}
        />
      );
    case "tolerations":
      return (
        <SchedulingEditor
          label={label}
          defaultValue={initial as Toleration[]}
          onChange={onChange}
        />
      );
  }
}

function secretKeysOf(env: readonly EnvVar[]): SecretRefs {
  const keys = new Map<string, string[]>();
  for (const entry of env) {
    const ref = entry.valueFrom?.secretKeyRef;
    if (ref?.name && ref.key)
      keys.set(ref.name, [...(keys.get(ref.name) ?? []), ref.key]);
  }
  return [...keys].map(([name, list]) => ({ name, keys: list }));
}

/**
 * RuleHints lists rules that the cluster checks after the save, each by its
 * message, or by its rule text when it has none. It renders nothing for no
 * rules.
 */
export function RuleHints({
  id,
  rules,
  className,
}: {
  id: string;
  rules: SchemaRule[];
  className?: string;
}) {
  if (rules.length === 0) return null;
  return (
    <div
      id={id}
      className={cn("flex flex-col gap-0.5 text-xs text-muted", className)}
    >
      <p>Checked after you save</p>
      <ul className="flex flex-col gap-0.5">
        {rules.map((r, index) => (
          <li key={index} className="break-words">
            {r.path && `${r.path}: `}
            {r.message ?? (
              <code className="font-mono [font-variant-ligatures:none]">
                {r.rule}
              </code>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
