"use client";

import { Plus, X } from "lucide-react";
import { useId, useRef, useState } from "react";
import { ParameterField } from "@/components/fleet/parameter-field";
import { TextField } from "@/components/setup/form-parts";
import { Button } from "@/components/ui/button";
import { isObject } from "@/lib/policy";
import type { OpenAPISchema, ValidationError } from "@/lib/schema";
import type { NamedValue, ParameterSetting } from "./settings";
import { isStructured, TemplateEditor } from "./template-editor";

const MAX_CHOICES = 32;

/**
 * AllowedEditor edits the allowed choices of a claim-editable parameter: a
 * row of name, label, and value for each. The value of a choice of an
 * array parameter is one item or a whole list. A template parameter edits
 * each value with TemplateEditor. errors are the errors of the editor, by
 * path "<row>.name", "<row>.label", or "<row>.value". onTextErrors gets
 * the errors of each value text that does not parse, which the choices
 * cannot hold. The editor keeps its rows, so a parent remounts it to
 * discard them.
 */
export function AllowedEditor({
  setting,
  parameters,
  errors,
  onAllowed,
  onTextErrors,
}: {
  setting: ParameterSetting;
  /** parameters are the other parameters of the registration, for the chips of a template. */
  parameters: string[];
  errors: readonly ValidationError[];
  onAllowed: (allowed: NamedValue[]) => void;
  onTextErrors: (errors: ValidationError[]) => void;
}) {
  const prefix = useId();
  const [ids, setIds] = useState(() => setting.allowed.map((_, i) => i));
  // A choice that holds a whole list keeps the editor of a list while its
  // text does not parse.
  const [lists] = useState(
    () =>
      new Set(
        setting.allowed.flatMap((c, i) => (Array.isArray(c.value) ? [i] : [])),
      ),
  );
  const next = useRef(ids.length);
  const textErrors = useRef<Record<number, ValidationError[]>>({});
  const { allowed } = setting;
  const schema = itemSchema(setting.schema);
  const reportText = () =>
    onTextErrors(Object.values(textErrors.current).flat());
  const update = (row: number, change: Partial<NamedValue>) =>
    onAllowed(allowed.map((c, i) => (i === row ? { ...c, ...change } : c)));
  const errorAt = (path: string) =>
    errors.find((e) => e.path === path)?.message;

  return (
    <div className="flex flex-col gap-3 rounded-md border border-divider p-3">
      <p className="text-sm text-muted">
        A claim picks a choice by its label. The claim stores the name, and only
        the request gets the value.
      </p>
      {allowed.map((choice, row) => {
        const id = ids[row] ?? row;
        const valueSchema = lists.has(id) ? setting.schema : schema;
        return (
          <fieldset key={id} className="flex min-w-0 flex-col gap-2">
            <legend className="sr-only">{`Choice ${row + 1}`}</legend>
            <div className="flex flex-wrap items-start gap-3">
              <TextField
                label="Name"
                value={choice.name}
                mono
                error={errorAt(`${row}.name`)}
                onChange={(name) => update(row, { name })}
              />
              <TextField
                label="Label"
                value={choice.label}
                error={errorAt(`${row}.label`)}
                onChange={(label) => update(row, { label })}
              />
              <Button
                variant="ghost"
                size="icon-sm"
                className="mt-7"
                aria-label={`Remove choice ${row + 1}`}
                onClick={() => {
                  delete textErrors.current[id];
                  setIds(ids.filter((_, i) => i !== row));
                  onAllowed(allowed.filter((_, i) => i !== row));
                  reportText();
                }}
              >
                <X />
              </Button>
            </div>
            {setting.template ? (
              <TemplateEditor
                label="Value"
                value={choice.value}
                object={isStructured(valueSchema)}
                error={errorAt(`${row}.value`)}
                parameters={parameters}
                onChange={(value, found) => {
                  textErrors.current[id] = value === undefined ? found : [];
                  update(row, { value });
                  reportText();
                }}
              />
            ) : (
              <ParameterField
                id={`${prefix}-choice-${id}-value`}
                rule={{
                  name: "value",
                  label: "Value",
                  claimEditable: true,
                  ...(valueSchema && { schema: valueSchema }),
                }}
                value={choice.value}
                error={errorAt(`${row}.value`)}
                onValue={(value, parse) => {
                  textErrors.current[id] = parse ?? [];
                  update(row, { value });
                  reportText();
                }}
              />
            )}
          </fieldset>
        );
      })}
      <Button
        variant="outline"
        size="sm"
        className="self-start"
        disabled={allowed.length >= MAX_CHOICES}
        onClick={() => {
          setIds([...ids, next.current++]);
          onAllowed([...allowed, { name: "", label: "", value: undefined }]);
        }}
      >
        <Plus aria-hidden />
        Add a choice
      </Button>
    </div>
  );
}

// A choice of an array parameter is one item of the list.
function itemSchema(
  schema: OpenAPISchema | undefined,
): OpenAPISchema | undefined {
  if (schema?.type !== "array") return schema;
  return isObject(schema.items) ? (schema.items as OpenAPISchema) : undefined;
}
