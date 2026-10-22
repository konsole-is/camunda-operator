"use client";

import {
  Fragment,
  type ReactNode,
  useEffect,
  useId,
  useRef,
  useState,
} from "react";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import type {
  EditableParameter,
  OptionView,
  ParameterRule,
  PluginSelection,
  SectionView,
} from "@/lib/policy/types";
import {
  type SchemaRule,
  type ValidationError,
  schemaRules,
  validate,
} from "@/lib/schema";
import { cn } from "@/lib/utils";
import { LockedField } from "./locked-field";
import { YamlEditor } from "./yaml-editor";

/** Values holds the parameter values that the form edits, by registration and parameter. */
type Values = Record<string, Record<string, unknown>>;

/** ParseErrors holds the errors of YAML that does not parse, by registration and parameter. */
type ParseErrors = Record<string, Record<string, ValidationError[]>>;

/** FieldError is an error of the form, with the key of its parameter. An error of the section has no key. */
type FieldError = { key?: string; error: ValidationError };

/**
 * parameterKey names one parameter of one registration in the touched set,
 * the generations, and the errors. A Kubernetes object name cannot hold "/",
 * so a registration whose name starts with another name never shares a key
 * with it. The merge FieldPath and the schema error path have other roots
 * and stay other grammars on purpose.
 */
function parameterKey(registration: string, parameter: string): string {
  return `${registration}/${parameter}`;
}

/**
 * SectionCards is one section of a create flow: a card for each allowed
 * option, the Default label on the option that the environment preselects,
 * and, under each selected option, its parameters. A claim-editable parameter
 * is a field that is checked against its schema, or, with an allowed list, a
 * select or a checkbox group for an array that shows the labels and submits
 * the names. Nothing is preselected in an allowed list unless the environment
 * value is one of its names. A selection holds only the parameters that differ
 * from the policy. Each x-kubernetes-validations rule of a claim-editable
 * parameter shows as a hint under it, and each rule of the root of an
 * option's schema shows under the option. The form does not check the rules.
 * A selected option that new options no longer hold, or disable, leaves the
 * selection. onChange gets the initial selections and errors once, then each
 * change. An error path is `<registration>/<parameter>`, then `.<path>` for an
 * error inside the value, or the section name when a required section has no
 * selection.
 */
export function SectionCards({
  section,
  defaultValue,
  onChange,
}: {
  section: SectionView;
  defaultValue?: PluginSelection[];
  onChange?: (selections: PluginSelection[], errors: ValidationError[]) => void;
}) {
  const id = useId();
  const [selected, setSelected] = useState<string[]>(() =>
    initialSelection(section, defaultValue),
  );
  const [values, setValues] = useState<Values>(() =>
    initialValues(section, defaultValue),
  );
  // A parameter that the claim or the user set. The others follow the policy.
  const [touched, setTouched] = useState<ReadonlySet<string>>(
    () =>
      new Set(
        (defaultValue ?? []).flatMap((s) =>
          Object.keys(s.parameters ?? {}).map((name) =>
            parameterKey(s.registration, name),
          ),
        ),
      ),
  );
  const [parseErrors, setParseErrors] = useState<ParseErrors>({});
  const [seenSection, setSeenSection] = useState(section);
  // A YAML field keeps its own text, so a new default needs a new field.
  const [generations, setGenerations] = useState<Record<string, number>>({});

  if (section !== seenSection) {
    setSeenSection(section);
    const kept = selected.filter((r) => {
      const option = findOption(section, r);
      return !!option && !option.disabledReason;
    });
    setSelected(kept);
    setParseErrors(errorsOf(parseErrors, kept));
    setValues(withPolicy(section, values, touched));
    setGenerations(nextGenerations(section, generations, touched));
  }

  const selections = selectionsOf(section, selected, values);
  const fieldErrors = sectionErrors(section, selected, values, parseErrors);
  const errors = fieldErrors.map((f) => f.error);
  const report = JSON.stringify([selections, errors]);
  const reported = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (reported.current === report) return;
    reported.current = report;
    onChange?.(selections, errors);
  });

  // An option that leaves the selection loses its YAML drafts, so their
  // errors go too.
  const pick = (next: string[]) => {
    setSelected(next);
    setParseErrors(errorsOf(parseErrors, next));
  };
  const setParameter = (
    registration: string,
    name: string,
    value: unknown,
    parse: ValidationError[] = [],
  ) => {
    setValues({
      ...values,
      [registration]: { ...values[registration], [name]: value },
    });
    setParseErrors({
      ...parseErrors,
      [registration]: { ...parseErrors[registration], [name]: parse },
    });
    setTouched(new Set(touched).add(parameterKey(registration, name)));
  };

  const many = section.selection === "Many";
  const showEmpty = !many && !section.required && !!section.emptyOptionLabel;

  return (
    <fieldset className="flex min-w-0 flex-col gap-3">
      <legend className="mb-1 flex items-center gap-2 font-heading text-base font-semibold text-text">
        {section.label}
        {section.required && <Badge variant="outline">Required</Badge>}
      </legend>
      {section.description && (
        <p className="-mt-1 text-sm text-muted">{section.description}</p>
      )}
      <div className="grid gap-3 md:grid-cols-2">
        {showEmpty && (
          <OptionCard
            name={`${id}-option`}
            type="radio"
            checked={selected.length === 0}
            label={section.emptyOptionLabel!}
            onPick={() => pick([])}
          />
        )}
        {section.options.map((option, index) => {
          const isSelected = selected.includes(option.registration);
          return (
            <OptionCard
              key={option.registration}
              name={`${id}-option`}
              type={many ? "checkbox" : "radio"}
              checked={isSelected}
              label={option.label}
              description={option.description}
              isDefault={option.isDefault}
              disabledReason={option.disabledReason}
              onPick={(on) =>
                pick(
                  many
                    ? on
                      ? [...selected, option.registration]
                      : selected.filter((r) => r !== option.registration)
                    : [option.registration],
                )
              }
            >
              {isSelected && (
                <OptionParameters
                  idPrefix={`${id}-${index}`}
                  option={option}
                  values={values[option.registration] ?? {}}
                  generations={generations}
                  errors={fieldErrors.filter(
                    (f) => f.key !== undefined && touched.has(f.key),
                  )}
                  onValue={(name, value, parse) =>
                    setParameter(option.registration, name, value, parse)
                  }
                />
              )}
            </OptionCard>
          );
        })}
      </div>
    </fieldset>
  );
}

/**
 * OptionCard is one option. The parameters (children) sit inside the card
 * but outside its label, so a click in a field does not toggle the option.
 */
function OptionCard({
  name,
  type,
  checked,
  label,
  description,
  isDefault = false,
  disabledReason,
  onPick,
  children,
}: {
  name: string;
  type: "radio" | "checkbox";
  checked: boolean;
  label: string;
  description?: string;
  isDefault?: boolean;
  disabledReason?: string;
  onPick: (on: boolean) => void;
  children?: ReactNode;
}) {
  return (
    <div
      className={cn(
        "flex min-w-0 flex-col gap-3 rounded-md border bg-panel p-3",
        checked ? "border-accent" : "border-border",
      )}
    >
      <label
        className={cn(
          "flex gap-3",
          disabledReason ? "cursor-not-allowed" : "cursor-pointer",
        )}
      >
        <input
          type={type}
          name={name}
          checked={checked}
          disabled={!!disabledReason}
          onChange={(event) => onPick(event.target.checked)}
          className="mt-1 size-4 shrink-0 accent-[var(--accent)]"
        />
        <span className="flex min-w-0 flex-col gap-0.5">
          <span
            className={cn(
              "flex flex-wrap items-center gap-2 text-sm font-medium",
              disabledReason ? "text-muted" : "text-text",
            )}
          >
            {label}
            {isDefault && <Badge variant="secondary">Default</Badge>}
          </span>
          {description && (
            <span className="text-sm text-muted">{description}</span>
          )}
          {disabledReason && (
            <span className="text-sm text-warn">{disabledReason}</span>
          )}
        </span>
      </label>
      {children}
    </div>
  );
}

/** OptionParameters shows the choice cards of an option, then its other parameters. */
function OptionParameters({
  idPrefix,
  option,
  values,
  generations,
  errors,
  onValue,
}: {
  idPrefix: string;
  option: OptionView;
  values: Record<string, unknown>;
  generations: Record<string, number>;
  errors: FieldError[];
  onValue: (name: string, value: unknown, parse?: ValidationError[]) => void;
}) {
  const choiceRule =
    option.choices &&
    option.parameters.find((r) => r.name === option.choices!.parameter);
  // A locked pick shows as a locked field, because it has no value to mark.
  const choices =
    choiceRule?.claimEditable === false ? undefined : option.choices;
  const errorOf = (name: string) =>
    errors.find((f) => f.key === parameterKey(option.registration, name))?.error
      .message;
  const rest = option.parameters.filter((r) => r.name !== choices?.parameter);
  const choiceError = choices && errorOf(choices.parameter);
  const choiceRules =
    choices && choiceRule?.claimEditable
      ? schemaRules(choiceRule.schema ?? {})
      : [];
  const optionRules = schemaRules(option.schema ?? {}).filter(
    (r) => r.path === "",
  );
  if (!choices && rest.length === 0 && optionRules.length === 0) return null;
  return (
    <div className="flex flex-col gap-3 border-t border-divider pt-3">
      {choices && (
        <fieldset
          aria-describedby={
            [
              choiceError && `${idPrefix}-choice-error`,
              choiceRules.length > 0 && `${idPrefix}-choice-rules`,
            ]
              .filter(Boolean)
              .join(" ") || undefined
          }
          className="flex flex-col gap-2"
        >
          <legend className="mb-1 text-sm font-medium text-muted">
            {choiceRule?.label ?? "Preset"}
          </legend>
          <div className="grid gap-2 sm:grid-cols-2">
            {choices.items.map((item) => {
              const current = values[choices.parameter];
              return (
                <label
                  key={item.name}
                  className={cn(
                    "flex gap-2 rounded-md border p-2 text-sm",
                    current === item.name ? "border-accent" : "border-border",
                  )}
                >
                  <input
                    type="radio"
                    name={`${idPrefix}-choice`}
                    checked={current === item.name}
                    onChange={() => onValue(choices.parameter, item.name)}
                    className="mt-0.5 size-4 shrink-0 accent-[var(--accent)]"
                  />
                  <span className="flex min-w-0 flex-col gap-1">
                    <span className="font-medium text-text">{item.name}</span>
                    {item.specs && item.specs.length > 0 && (
                      <dl className="grid grid-cols-[auto_1fr] gap-x-3 text-xs">
                        {item.specs.map((spec) => (
                          <Fragment key={spec.label}>
                            <dt className="text-muted">{spec.label}</dt>
                            <dd className="text-text-2">{spec.value}</dd>
                          </Fragment>
                        ))}
                      </dl>
                    )}
                  </span>
                </label>
              );
            })}
          </div>
          {choiceError && (
            <p id={`${idPrefix}-choice-error`} className="text-xs text-bad">
              {choiceError}
            </p>
          )}
          <RuleHints id={`${idPrefix}-choice-rules`} rules={choiceRules} />
        </fieldset>
      )}
      {rest.map((rule, index) => (
        <ParameterField
          key={`${rule.name}-${generations[parameterKey(option.registration, rule.name)] ?? 0}`}
          id={`${idPrefix}-${index}`}
          rule={rule}
          value={values[rule.name]}
          error={errorOf(rule.name)}
          onValue={(v, parse) => onValue(rule.name, v, parse)}
        />
      ))}
      <RuleHints
        id={`${idPrefix}-rules`}
        rules={optionRules}
        className="border-t border-divider pt-3"
      />
    </div>
  );
}

function ParameterField({
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
  if (rule.allowed && schema.type === "array") {
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
function RuleHints({
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
function initialSelection(
  section: SectionView,
  defaultValue?: PluginSelection[],
): string[] {
  const usable = (r: string) =>
    section.options.some((o) => o.registration === r && !o.disabledReason);
  if (defaultValue)
    return defaultValue.map((s) => s.registration).filter(usable);
  const preset = section.options.find((o) => o.isDefault && !o.disabledReason);
  return preset ? [preset.registration] : [];
}

function findOption(
  section: SectionView,
  registration: string,
): OptionView | undefined {
  return section.options.find((o) => o.registration === registration);
}

function isEditable(rule: ParameterRule): rule is EditableParameter {
  return rule.claimEditable;
}

function initialValues(
  section: SectionView,
  defaultValue?: PluginSelection[],
): Values {
  return Object.fromEntries(
    section.options.map((option) => {
      const stored =
        defaultValue?.find((s) => s.registration === option.registration)
          ?.parameters ?? {};
      return [
        option.registration,
        Object.fromEntries(
          option.parameters.map((rule) => [
            rule.name,
            Object.hasOwn(stored, rule.name)
              ? stored[rule.name]
              : startValue(rule),
          ]),
        ),
      ];
    }),
  );
}

function errorsOf(
  parseErrors: ParseErrors,
  registrations: string[],
): ParseErrors {
  return Object.fromEntries(
    Object.entries(parseErrors).filter(([registration]) =>
      registrations.includes(registration),
    ),
  );
}

function withPolicy(
  section: SectionView,
  values: Values,
  touched: ReadonlySet<string>,
): Values {
  return Object.fromEntries(
    Object.entries(initialValues(section)).map(([registration, starts]) => [
      registration,
      Object.fromEntries(
        Object.entries(starts).map(([name, start]) => [
          name,
          touched.has(parameterKey(registration, name)) &&
          Object.hasOwn(values[registration] ?? {}, name)
            ? values[registration][name]
            : start,
        ]),
      ),
    ]),
  );
}

function nextGenerations(
  section: SectionView,
  generations: Record<string, number>,
  touched: ReadonlySet<string>,
): Record<string, number> {
  const next = { ...generations };
  for (const option of section.options) {
    for (const rule of option.parameters) {
      const key = parameterKey(option.registration, rule.name);
      if (!touched.has(key)) next[key] = (next[key] ?? 0) + 1;
    }
  }
  return next;
}

/** The start value of a field. An allowed list keeps only the names it holds. */
function startValue(rule: ParameterRule): unknown {
  if (!isEditable(rule)) return undefined;
  if (!rule.allowed) return rule.value;
  const names = rule.allowed.map((a) => a.name);
  if (rule.schema?.type === "array")
    return Array.isArray(rule.value)
      ? rule.value.filter((n) => names.includes(n))
      : [];
  return typeof rule.value === "string" && names.includes(rule.value)
    ? rule.value
    : undefined;
}

function selectionsOf(
  section: SectionView,
  selected: string[],
  values: Values,
): PluginSelection[] {
  return selected.flatMap((registration) => {
    const option = findOption(section, registration);
    if (!option) return [];
    const parameters = Object.fromEntries(
      option.parameters
        .filter(isEditable)
        .map((rule) => [rule.name, values[registration]?.[rule.name]] as const)
        .filter(
          ([name, value]) =>
            value !== undefined && !same(value, policyValue(option, name)),
        ),
    );
    return Object.keys(parameters).length
      ? { section: section.name, registration, parameters }
      : { section: section.name, registration };
  });
}

function policyValue(option: OptionView, name: string): unknown {
  const rule = option.parameters.find((r) => r.name === name);
  return rule && startValue(rule);
}

function sectionErrors(
  section: SectionView,
  selected: string[],
  values: Values,
  parseErrors: ParseErrors,
): FieldError[] {
  const errors: FieldError[] = [];
  if (section.required && selected.length === 0)
    errors.push({ error: { path: section.name, message: "Choose an option" } });
  for (const registration of selected) {
    const option = findOption(section, registration);
    if (!option) continue;
    for (const rule of option.parameters.filter(isEditable)) {
      const value = values[registration]?.[rule.name];
      const key = parameterKey(registration, rule.name);
      const add = (message: string, path = key) =>
        errors.push({ key, error: { path, message } });
      const parse = ownEntry(ownEntry(parseErrors, registration), rule.name);
      if (parse?.length) {
        for (const e of parse) add(e.message);
        continue;
      }
      const emptyList = Array.isArray(value) && value.length === 0;
      if (
        value === undefined ||
        ((value === "" || emptyList) && rule.required)
      ) {
        if (rule.required) add("is required");
        continue;
      }
      const choices = option.choices;
      if (
        choices?.parameter === rule.name &&
        !choices.items.some((item) => item.name === value)
      ) {
        add(`${value} is not an available choice`);
        continue;
      }
      if (rule.allowed) {
        const names = new Set(rule.allowed.map((a) => a.name));
        for (const picked of Array.isArray(value) ? value : [value])
          if (!names.has(picked as string))
            add(`${picked} is not an allowed choice`);
        // The schema checks only the count: its items are the names, and
        // the values of the choices stay on the server.
        const { minItems, maxItems } = rule.schema ?? {};
        if (
          Array.isArray(value) &&
          typeof minItems === "number" &&
          value.length < minItems
        )
          add(`Choose at least ${minItems}`);
        if (
          Array.isArray(value) &&
          typeof maxItems === "number" &&
          value.length > maxItems
        )
          add(`Choose at most ${maxItems}`);
        continue;
      }
      if (!rule.schema) continue;
      const result = validate(rule.schema, value);
      if (!result.ok)
        for (const e of result.errors)
          add(e.message, e.path ? `${key}.${e.path}` : key);
    }
  }
  return errors;
}

/** ownEntry ignores inherited keys, so a name such as "constructor" finds nothing. */
function ownEntry<T>(
  record: Record<string, T> | undefined,
  key: string,
): T | undefined {
  return record && Object.hasOwn(record, key) ? record[key] : undefined;
}

function same(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}
