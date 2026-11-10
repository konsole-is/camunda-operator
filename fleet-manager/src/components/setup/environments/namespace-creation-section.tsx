"use client";

import { Plus, X } from "lucide-react";
import { useRef, useState } from "react";
import { KeyValueEditor } from "@/components/fleet/key-value-editor";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import type { ValidationError } from "@/lib/schema";
import type { EnvironmentDraft } from "./draft";
import { ErrorText } from "@/components/setup/form-parts";

type Creation = NonNullable<EnvironmentDraft["spec"]["namespaceCreation"]>;

const MAX_KEYS = 16;

/**
 * NamespaceCreationSection edits whether the create flow can create a
 * namespace, its fixed labels, and its required label keys. The
 * environment label never shows in either list. onChange gets the errors
 * of the fixed label rows too, such as a key that two rows set, which the
 * value cannot hold. The section keeps the rows that the admin types, so a
 * parent remounts it to discard them.
 */
export function NamespaceCreationSection({
  value = {},
  errors,
  onChange,
}: {
  value?: Creation;
  errors: readonly ValidationError[];
  onChange: (value: Creation, rowErrors: ValidationError[]) => void;
}) {
  const [keys, setKeys] = useState(() =>
    (value.requiredLabelKeys ?? [])
      .filter((key) => key !== ENVIRONMENT_LABEL)
      .map((key, id) => ({ id, key })),
  );
  const next = useRef(keys.length);
  const labelRowErrors = useRef<ValidationError[]>([]);
  const shownLabels = Object.fromEntries(
    Object.entries(value.labels ?? {}).filter(([k]) => k !== ENVIRONMENT_LABEL),
  );
  const updateKeys = (rows: typeof keys) => {
    setKeys(rows);
    onChange(
      {
        ...value,
        requiredLabelKeys: rows.map((r) => r.key.trim()).filter(Boolean),
      },
      labelRowErrors.current,
    );
  };
  const errorAt = (path: string) =>
    errors.find((e) => e.path === `spec.namespaceCreation.${path}`)?.message;
  return (
    <div className="flex flex-col gap-4 border-t border-divider pt-4">
      <h3 className="font-heading text-sm font-semibold">Namespace creation</h3>
      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          className="size-4 accent-accent"
          checked={value.allowed ?? false}
          onChange={(event) =>
            onChange(
              { ...value, allowed: event.target.checked },
              labelRowErrors.current,
            )
          }
        />
        The create flow can create a namespace
      </label>
      <div className="flex max-w-xl flex-col gap-1">
        <KeyValueEditor
          label="Fixed labels of a new namespace"
          defaultValue={shownLabels}
          format="label"
          onChange={(labels, rowErrors) => {
            labelRowErrors.current = rowErrors;
            onChange({ ...value, labels }, rowErrors);
          }}
        />
        <ErrorText error={errorAt("labels")} />
      </div>
      <fieldset className="flex max-w-md flex-col gap-2">
        <legend className="mb-1 text-sm font-medium text-muted">
          Required label keys
        </legend>
        <p className="text-xs text-muted">
          The create flow asks for a value of each key.
        </p>
        {keys.map((row, index) => (
          <div key={row.id} className="flex items-center gap-2">
            <Input
              aria-label={`Required label key ${index + 1}`}
              placeholder="team"
              value={row.key}
              spellCheck={false}
              className="font-mono"
              onChange={(event) =>
                updateKeys(
                  keys.map((r) =>
                    r.id === row.id ? { ...r, key: event.target.value } : r,
                  ),
                )
              }
            />
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`Remove required label key ${index + 1}`}
              onClick={() => updateKeys(keys.filter((r) => r.id !== row.id))}
            >
              <X />
            </Button>
          </div>
        ))}
        <Button
          variant="outline"
          size="sm"
          className="self-start"
          disabled={keys.length >= MAX_KEYS}
          onClick={() => updateKeys([...keys, { id: next.current++, key: "" }])}
        >
          <Plus aria-hidden />
          Add a key
        </Button>
        <ErrorText error={errorAt("requiredLabelKeys")} />
      </fieldset>
    </div>
  );
}
