"use client";

import { useEffect, useId, useRef, useState } from "react";
import { EnvEditor, type EnvVar } from "@/components/fleet/env-editor";
import { FieldValue } from "@/components/fleet/field-state";
import { KeyValueEditor } from "@/components/fleet/key-value-editor";
import { LockedField } from "@/components/fleet/locked-field";
import {
  ResourcesEditor,
  type Resources,
} from "@/components/fleet/resources-editor";
import {
  SchedulingEditor,
  type Toleration,
} from "@/components/fleet/scheduling-editor";
import {
  type SecretKeyRef,
  type SecretRefs,
  SecretRefPicker,
} from "@/components/fleet/secret-ref-picker";
import { StorageSizeEditor } from "@/components/fleet/storage-size-editor";
import { YamlEditor } from "@/components/fleet/yaml-editor";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import {
  lockLabel,
  pathFrom,
  stateLabel,
  stateNote,
  type Step,
} from "@/lib/fields";
import type { ValidationError } from "@/lib/schema";
import type { FieldError } from "@/lib/session";
import {
  editorValueOf,
  type Setting,
  type SettingEditor,
  settingChange,
} from "./editable-settings";
import type { SettingsGroup } from "./settings";

/**
 * SettingsEditor edits the settings of one group. Each setting shows its
 * state and its editor, or its lock reason and no editor. A setting that
 * the claim sets has a reset link. An embedded group notes that its
 * settings apply when the app runs standalone. A change with errors is not
 * passed on. errors are the field errors of the claim, such as a refusal of
 * the server, and each shows at the setting whose claim path holds it.
 * invalid gets the error that a setting shows, as SettingRow gives it. It
 * holds no session.
 */
export function SettingsEditor({
  group,
  settings,
  secrets,
  errors = [],
  set,
  reset,
  invalid,
}: {
  group: SettingsGroup;
  settings: readonly Setting[];
  secrets: SecretRefs;
  errors?: readonly FieldError[];
  set: (steps: Step[], value: unknown) => void;
  reset: (steps: Step[]) => void;
  invalid?: (steps: Step[], message: string | undefined) => void;
}) {
  const [resets, setResets] = useState(0);
  return (
    <section
      aria-label={`${group.label} settings`}
      className="flex min-w-0 flex-col gap-1"
    >
      <h3 className="font-heading text-sm font-semibold">
        {group.embedded ? `${group.label} (embedded)` : group.label}
      </h3>
      {group.embedded && (
        <p className="text-sm text-muted">
          Applies when {group.label} runs standalone. The exception is extraEnv
          and extraEnvFrom: they apply to the host now.
        </p>
      )}
      <div className="flex flex-col divide-y divide-divider">
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
    </section>
  );
}

/**
 * SettingRow is one setting: its editor with its state and reset link, or
 * its lock reason. A change with errors shows them and is not passed on. A
 * claim value of undefined resets the field. errors are field errors from
 * outside the row. onReset runs after a click on the reset link: the caller
 * remounts the rows, so that they show the values below the claim. invalid
 * gets the steps of the setting and "<label>: <error>" while the editor
 * shows an error, and undefined once the error goes or the row unmounts.
 */
export function SettingRow({
  setting,
  secrets,
  errors = [],
  set,
  reset,
  invalid,
  onReset,
}: {
  setting: Setting;
  secrets?: SecretRefs;
  errors?: readonly FieldError[];
  set: (steps: Step[], value: unknown) => void;
  reset: (steps: Step[]) => void;
  invalid?: (steps: Step[], message: string | undefined) => void;
  onReset: () => void;
}) {
  const [mounted] = useState(setting.value);
  const { editor, steps } = setting;
  const shows = useShownError(steps, invalid);
  if (!editor || !steps)
    return (
      <div className="py-3">
        <LockedField
          label={setting.label}
          value={<FieldValue value={setting.value} />}
          reason={
            setting.lock
              ? lockLabel(setting.lock)
              : "Change it in the Claim tab."
          }
        />
      </div>
    );
  const change = (value: unknown) => {
    const next = settingChange(setting, value);
    if (next.value === undefined) reset(next.steps);
    else set(next.steps, next.value);
  };
  const note = setting.state && stateNote(setting.state);
  return (
    <div
      role="group"
      aria-label={setting.label}
      className="flex min-w-0 flex-col gap-2 py-3"
    >
      <Editor
        label={setting.label}
        editor={editor}
        value={setting.value}
        current={"current" in setting ? setting.current : mounted}
        secrets={secrets ?? []}
        onChange={change}
        onInvalid={(message) =>
          shows(message && `${setting.label}: ${message}`)
        }
      />
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
        <span>{setting.state ? stateLabel(setting.state) : "Not set"}</span>
        {note && <span>{note}</span>}
        {setting.claimSet && (
          <Button
            variant="link"
            size="xs"
            className="h-auto px-0"
            onClick={() => {
              reset(steps);
              onReset();
            }}
          >
            {setting.resetLabel}
          </Button>
        )}
        {setting.claimSet && setting.resetNote && (
          <span>{setting.resetNote}</span>
        )}
        {setting.inherited !== undefined && (
          <span>
            The preset or the release adds {setting.inherited} before these.
          </span>
        )}
      </div>
      {errors.map((e, i) => (
        <p key={i} role="alert" className="text-xs text-bad">
          {e.message}
        </p>
      ))}
    </div>
  );
}

// useShownError returns the function that reports the error of the editor
// at steps to invalid. The row clears a reported error when it unmounts.
function useShownError(
  steps: Step[] | undefined,
  invalid: ((steps: Step[], message: string | undefined) => void) | undefined,
) {
  const clear = useRef<(() => void) | undefined>(undefined);
  useEffect(() => () => clear.current?.(), []);
  return (message: string | undefined) => {
    if (!steps || !invalid) return;
    invalid(steps, message);
    clear.current =
      message === undefined ? undefined : () => invalid(steps, undefined);
  };
}

// Editor is the input of one editor kind. It is uncontrolled: it starts
// from value and reports each valid change. A change with errors is not
// reported; onInvalid gets its first error, and undefined after a valid
// change. current is the value before the pending changes.
function Editor({
  label,
  editor,
  value,
  current,
  secrets,
  onChange,
  onInvalid,
}: {
  label: string;
  editor: SettingEditor;
  value: unknown;
  current: unknown;
  secrets: SecretRefs;
  onChange: (value: unknown) => void;
  onInvalid: (message: string | undefined) => void;
}) {
  const initial = editorValueOf(editor, value);
  const valid = (v: unknown, errors: readonly ValidationError[]) => {
    onInvalid(errors[0]?.message);
    if (errors.length === 0) onChange(v);
  };
  switch (editor.kind) {
    case "number":
      return (
        <NumberEditor
          label={label}
          editor={editor}
          defaultValue={initial as number | undefined}
          onChange={onChange}
          onInvalid={onInvalid}
        />
      );
    case "text":
      return (
        <TextEditor
          label={label}
          defaultValue={String(initial)}
          onChange={onChange}
        />
      );
    case "toggle":
      return (
        <ToggleEditor
          label={label}
          defaultValue={initial === true}
          onChange={onChange}
        />
      );
    case "select":
      return (
        <SelectEditor
          label={label}
          options={editor.options}
          defaultValue={String(initial)}
          onChange={onChange}
        />
      );
    case "resources":
      return (
        <ResourcesEditor
          label={label}
          defaultValue={initial as Resources}
          onChange={valid}
        />
      );
    case "storageSize":
      return (
        <StorageSizeEditor
          label={label}
          defaultValue={String(initial)}
          current={
            typeof current === "string" || typeof current === "number"
              ? String(current)
              : undefined
          }
          onChange={valid}
        />
      );
    case "env":
      return (
        <EnvSetting
          label={label}
          secrets={secrets}
          defaultValue={initial as EnvVar[]}
          onChange={valid}
        />
      );
    case "keyValue":
      return (
        <KeyValueEditor
          label={label}
          format={editor.format}
          defaultValue={initial as Record<string, string>}
          onChange={valid}
        />
      );
    case "tolerations":
      return (
        <SchedulingEditor
          label={label}
          defaultValue={initial as Toleration[]}
          onChange={valid}
        />
      );
    case "secretRef":
      return (
        <SecretRefEditor
          label={label}
          secrets={secrets}
          defaultValue={initial as SecretKeyRef | undefined}
          onChange={onChange}
        />
      );
    case "yaml":
      return (
        <YamlEditor
          label={label}
          schema={editor.schema}
          rows={6}
          defaultValue={initial}
          onChange={valid}
        />
      );
  }
}

// useUserEdit passes on one change after each user edit inside the element
// that gets mark. A Secret picker also reports a change when secrets change,
// as when they load, and such a change must not reach the claim.
function useUserEdit(secrets: SecretRefs) {
  const edited = useRef(false);
  const secretsKey = JSON.stringify(secrets);
  useEffect(() => {
    edited.current = false;
  }, [secretsKey]);
  const mark = () => {
    edited.current = true;
  };
  const only =
    <A extends unknown[]>(f: (...args: A) => void) =>
    (...args: A) => {
      if (!edited.current) return;
      edited.current = false;
      f(...args);
    };
  return { mark, only };
}

function EnvSetting({
  label,
  secrets,
  defaultValue,
  onChange,
}: {
  label: string;
  secrets: SecretRefs;
  defaultValue: EnvVar[];
  onChange: (env: EnvVar[], errors: ValidationError[]) => void;
}) {
  const { mark, only } = useUserEdit(secrets);
  return (
    <div className="min-w-0" onChange={mark} onClick={mark}>
      <EnvEditor
        label={label}
        secrets={secrets}
        defaultValue={defaultValue}
        onChange={only(onChange)}
      />
    </div>
  );
}

function SecretRefEditor({
  label,
  secrets,
  defaultValue,
  onChange,
}: {
  label: string;
  secrets: SecretRefs;
  defaultValue: SecretKeyRef | undefined;
  onChange: (value: unknown) => void;
}) {
  const { mark, only } = useUserEdit(secrets);
  return (
    <div className="flex min-w-0 flex-col gap-1" onChange={mark}>
      <span className="text-sm font-medium text-muted">{label}</span>
      <SecretRefPicker
        label={label}
        secrets={secrets}
        defaultValue={defaultValue}
        onChange={only(onChange)}
      />
    </div>
  );
}

function NumberEditor({
  label,
  editor,
  defaultValue,
  onChange,
  onInvalid,
}: {
  label: string;
  editor: Extract<SettingEditor, { kind: "number" }>;
  defaultValue: number | undefined;
  onChange: (value: unknown) => void;
  onInvalid: (message: string | undefined) => void;
}) {
  const id = useId();
  const [text, setText] = useState(
    defaultValue === undefined ? "" : String(defaultValue),
  );
  const error = numberError(text, editor);
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <label htmlFor={id} className="text-sm font-medium text-muted">
        {label}
      </label>
      <Input
        id={id}
        type="number"
        className="w-32"
        min={editor.minimum}
        max={editor.maximum}
        step={editor.integer ? 1 : "any"}
        value={text}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${id}-error` : undefined}
        onChange={(event) => {
          const next = event.target.value;
          setText(next);
          const nextError = numberError(next, editor);
          onInvalid(nextError);
          if (!nextError) onChange(next === "" ? undefined : Number(next));
        }}
      />
      {error && (
        <p id={`${id}-error`} className="text-xs text-bad">
          {error}
        </p>
      )}
    </div>
  );
}

function numberError(
  text: string,
  editor: Extract<SettingEditor, { kind: "number" }>,
): string | undefined {
  if (text === "") return undefined;
  const n = Number(text);
  if (!Number.isFinite(n)) return "Enter a number";
  if (editor.integer && !Number.isInteger(n)) return "Enter a whole number";
  if (editor.minimum !== undefined && n < editor.minimum)
    return `Enter ${editor.minimum} or more`;
  if (editor.maximum !== undefined && n > editor.maximum)
    return `Enter ${editor.maximum} or less`;
  return undefined;
}

function TextEditor({
  label,
  defaultValue,
  onChange,
}: {
  label: string;
  defaultValue: string;
  onChange: (value: unknown) => void;
}) {
  const id = useId();
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <label htmlFor={id} className="text-sm font-medium text-muted">
        {label}
      </label>
      <Input
        id={id}
        defaultValue={defaultValue}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}

function ToggleEditor({
  label,
  defaultValue,
  onChange,
}: {
  label: string;
  defaultValue: boolean;
  onChange: (value: unknown) => void;
}) {
  const id = useId();
  return (
    <div className="flex items-center gap-2">
      <Checkbox
        id={id}
        defaultChecked={defaultValue}
        onCheckedChange={(checked) => onChange(checked === true)}
      />
      <label htmlFor={id} className="text-sm font-medium text-muted">
        {label}
      </label>
    </div>
  );
}

function SelectEditor({
  label,
  options,
  defaultValue,
  onChange,
}: {
  label: string;
  options: readonly string[];
  defaultValue: string;
  onChange: (value: unknown) => void;
}) {
  const id = useId();
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <label htmlFor={id} className="text-sm font-medium text-muted">
        {label}
      </label>
      <NativeSelect
        id={id}
        className="w-56"
        defaultValue={defaultValue}
        onChange={(event) => onChange(event.target.value)}
      >
        <NativeSelectOption value="">Not set</NativeSelectOption>
        {options.map((option) => (
          <NativeSelectOption key={option} value={option}>
            {option}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    </div>
  );
}

// errorsAt returns the errors at the claim path of steps or below it.
function errorsAt(
  errors: readonly FieldError[],
  steps: Step[] | undefined,
): FieldError[] {
  if (!steps) return [];
  const path = pathFrom("", steps);
  return errors.filter(
    (e) =>
      e.path !== undefined &&
      (e.path === path ||
        e.path.startsWith(`${path}.`) ||
        e.path.startsWith(`${path}[`)),
  );
}
