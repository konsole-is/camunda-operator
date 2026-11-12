"use client";

import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import type { ParameterRule } from "@/lib/policy/types";
import { type SchemaRule, type ValidationError, schemaRules } from "@/lib/schema";
import { cn } from "@/lib/utils";
import { LockedField } from "./locked-field";
import { YamlEditor } from "./yaml-editor";

/**
 * ParameterField is the field of one parameter rule with value. A locked
 * rule shows its summary as set by the admin. An editable rule picks its
 * editor: a checkbox group for an array with allowed names, a select for
 * allowed names or a string enum, a checkbox for a boolean, a number input
 * with its range, a text input for a string, and YamlEditor for any other
 * schema type. onValue gets undefined for a cleared field, and, from
 * YamlEditor, the errors of text that does not parse.
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
  if (rule.allowed && schema.type === "array" && !rule.allowedLists) {
    const allowed = rule.allowed;
    const names = Array.isArray(value) ? (value as string[]) : [];
    return (
      <fieldset
        aria-label={rule.label}
        aria-describedby={describedBy}
        className="flex flex-col gap-2"
      >
        <legend className="mb-1 text-sm text-muted">{rule.label}</legend>
        {allowed.map((choice, index) => (
          <div
            key={choice.name}
            className="flex items-center gap-2 text-sm text-text-2"
          >
            <Checkbox
              id={`${id}-choice-${index}`}
              aria-label={choice.label}
              checked={names.includes(choice.name)}
              onCheckedChange={(on) =>
                onValue(
                  allowed
                    .map((a) => a.name)
                    .filter((n) =>
                      n === choice.name ? on === true : names.includes(n),
                    ),
                )
              }
            />
            <label htmlFor={`${id}-choice-${index}`}>{choice.label}</label>
          </div>
        ))}
        {message}
        {hints}
      </fieldset>
    );
  }
  const stringEnum =
    Array.isArray(schema.enum) &&
    schema.enum.every((v) => typeof v === "string")
      ? (schema.enum as string[])
      : undefined;
  if (rule.allowed || stringEnum) {
    const options =
      rule.allowed ??
      stringEnum!.map((v) => ({ name: v, label: v === "" ? "Empty" : v }));
    // The options use their position as the value, so an enum value of ""
    // does not collide with the placeholder.
    const picked = options.findIndex((o) => o.name === value);
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
                : options[Number(e.target.value)].name,
            )
          }
        >
          <NativeSelectOption value="">Choose one</NativeSelectOption>
          {options.map((o, index) => (
            <NativeSelectOption key={o.name} value={String(index)}>
              {o.label}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        {message}
        {hints}
      </div>
    );
  }
  if (schema.type === "boolean") {
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
  }
  if (schema.type === "integer" || schema.type === "number") {
    const min = typeof schema.minimum === "number" ? schema.minimum : undefined;
    const max = typeof schema.maximum === "number" ? schema.maximum : undefined;
    return (
      <div className="flex max-w-48 flex-col gap-1">
        {label}
        <Input
          id={id}
          type="number"
          min={min}
          max={max}
          step={schema.type === "integer" ? 1 : "any"}
          value={value === undefined ? "" : String(value)}
          aria-invalid={invalid}
          aria-describedby={describedBy}
          onChange={(e) =>
            onValue(e.target.value === "" ? undefined : Number(e.target.value))
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
  if (schema.type === "string" || schema.type === undefined) {
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
  }
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <YamlEditor
        label={rule.label}
        defaultValue={value}
        schema={rule.schema}
        error={error}
        describedBy={rules.length > 0 ? `${id}-rules` : undefined}
        onChange={(v, found) => onValue(v, v === undefined ? found : [])}
      />
      {hints}
    </div>
  );
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
