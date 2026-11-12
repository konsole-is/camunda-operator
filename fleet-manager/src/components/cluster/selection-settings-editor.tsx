"use client";

import { useState } from "react";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import type { ClusterField, Step } from "@/lib/fields";
import { namesOf } from "@/lib/policy/options";
import type { OpenAPISchema } from "@/lib/schema";
import { selectionSettingsOf } from "./editable-settings";
import type { FieldError } from "@/lib/session";
import { errorsAt, SettingRow } from "./settings-editor";

type Choice = { name: string; value: unknown };

/**
 * SelectionSettingsEditor edits the parameters of one selection. fields are
 * the parameterFields of that selection, schema is spec.parameters of its
 * registration. A parameter that the claim cannot change shows its lock
 * reason and no editor. An object parameter with properties, such as
 * overrides, shows one editor for each property, and a change writes the
 * whole parameter. choices make a parameter pick the names of its
 * choices, whatever its schema. allowed does the same with the choices of
 * the policy. claimed holds the parameters of the selection in the claim.
 * It holds no session: set and reset get claim steps from the root of the
 * claim. invalid gets the error that a parameter shows, as SettingRow
 * gives it. errors show at the parameter whose claim path holds them or is
 * above them.
 */
export function SelectionSettingsEditor({
  fields,
  schema,
  choices,
  allowed,
  claimed,
  secrets,
  errors = [],
  set,
  reset,
  invalid,
}: {
  fields: readonly ClusterField[];
  schema: OpenAPISchema | undefined;
  choices?: Partial<Record<string, readonly string[]>>;
  allowed?: Partial<Record<string, readonly Choice[]>>;
  claimed?: Readonly<Record<string, unknown>>;
  secrets?: SecretRefs;
  errors?: readonly FieldError[];
  set: (steps: Step[], value: unknown) => void;
  reset: (steps: Step[]) => void;
  invalid?: (steps: Step[], message: string | undefined) => void;
}) {
  const [resets, setResets] = useState(0);
  const lists = choiceLists(allowed);
  const settings = selectionSettingsOf(
    fields.map((f) => withChoiceName(f, lists, claimed)),
    schemaWithNames(schema, lists),
    {
      ...choices,
      ...Object.fromEntries(
        Object.entries(lists).map(([name, list]) => [
          name,
          list.map((c) => c.name),
        ]),
      ),
    },
  );
  return (
    <div className="flex min-w-0 flex-col divide-y divide-divider">
      {settings.map((setting) => (
        <SettingRow
          key={`${setting.key}:${resets}`}
          setting={setting}
          secrets={secrets}
          errors={errorsAt(errors, setting.steps)}
          set={set}
          reset={reset}
          invalid={invalid}
          onReset={() => setResets((n) => n + 1)}
        />
      ))}
    </div>
  );
}

function choiceLists(
  allowed: Partial<Record<string, readonly Choice[]>> | undefined,
): Record<string, readonly Choice[]> {
  return Object.fromEntries(
    Object.entries(allowed ?? {}).flatMap(([name, list]) =>
      list?.length ? [[name, list]] : [],
    ),
  );
}

function withChoiceName(
  field: ClusterField,
  lists: Record<string, readonly Choice[]>,
  claimed: Readonly<Record<string, unknown>> | undefined,
): ClusterField {
  if (!Object.hasOwn(lists, field.label)) return field;
  return {
    ...field,
    value:
      claimed && Object.hasOwn(claimed, field.label)
        ? claimed[field.label]
        : namesOf(field.value, lists[field.label]),
  };
}

// schemaWithNames gives a list parameter whose choices are each a whole
// list the schema of one name: the claim stores one name for it.
function schemaWithNames(
  schema: OpenAPISchema | undefined,
  lists: Record<string, readonly Choice[]>,
): OpenAPISchema | undefined {
  const properties = schema?.properties;
  if (!properties) return schema;
  const names = Object.keys(lists).filter((name) =>
    lists[name].every((c) => Array.isArray(c.value)),
  );
  if (names.length === 0) return schema;
  return {
    ...schema,
    properties: {
      ...properties,
      ...Object.fromEntries(names.map((name) => [name, { type: "string" }])),
    },
  };
}
