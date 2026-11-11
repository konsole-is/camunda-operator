"use client";

import { useState } from "react";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import type { ClusterField, Step } from "@/lib/fields";
import type { OpenAPISchema } from "@/lib/schema";
import { selectionSettingsOf } from "./editable-settings";
import { SettingRow } from "./settings-editor";

/**
 * SelectionSettingsEditor edits the parameters of one selection. fields are
 * the parameterFields of that selection, schema is spec.parameters of its
 * registration. A parameter that the claim cannot change shows its lock
 * reason and no editor. An object parameter with properties, such as
 * overrides, shows one editor for each property, and a change writes the
 * whole parameter. choices turn a string parameter into a select. It holds
 * no session: set and reset get claim steps from the root of the claim.
 */
export function SelectionSettingsEditor({
  fields,
  schema,
  choices,
  secrets,
  set,
  reset,
}: {
  fields: readonly ClusterField[];
  schema: OpenAPISchema | undefined;
  choices?: Partial<Record<string, readonly string[]>>;
  secrets?: SecretRefs;
  set: (steps: Step[], value: unknown) => void;
  reset: (steps: Step[]) => void;
}) {
  const [resets, setResets] = useState(0);
  return (
    <div className="flex min-w-0 flex-col divide-y divide-divider">
      {selectionSettingsOf(fields, schema, choices).map((setting) => (
        <SettingRow
          key={`${setting.key}:${resets}`}
          setting={setting}
          secrets={secrets}
          set={set}
          reset={reset}
          onReset={() => setResets((n) => n + 1)}
        />
      ))}
    </div>
  );
}
