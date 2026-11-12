"use client";

import { useEffect, useRef, useState } from "react";
import { LimitsList } from "@/components/fleet/limits-list";
import { ParameterField } from "@/components/fleet/parameter-field";
import { Button } from "@/components/ui/button";
import type { Environment } from "@/generated/crds/environment";
import { accessPatterns, renderHosts } from "@/lib/claims/hosts";
import { renderJSON } from "@/lib/gotemplate";
import type { ValidationError } from "@/lib/schema";
import { AllowedEditor } from "./allowed-editor";
import { previewClaims, templateData } from "./preview";
import {
  type NamedValue,
  type OptionSettings,
  type ParameterSetting,
  presetsOf,
  withParameterPolicy,
} from "./settings";
import type { EnvironmentSources } from "./sources";
import { isStructured, TemplateEditor } from "./template-editor";
import { TemplatePreview } from "./template-preview";

type Spec = Environment["spec"];

const PRESET_WORDS = {
  ElasticsearchClusterPreset: "Elasticsearch",
  DatabaseServerPreset: "PostgreSQL",
} as const;

/**
 * OptionSettingsSection edits the parameters of each option that a section
 * of environment allows: the environment value, "A claim can change it",
 * and the allowed choices, with the limits of the registration read only.
 * environment holds the edited spec. errors are the errors of the draft by
 * path. onTextErrors gets the errors of each value text that does not
 * parse. The section keeps the text of its editors, so a parent remounts it
 * to discard them.
 */
export function OptionSettingsSection({
  settings,
  sources,
  environment,
  errors,
  onSpec,
  onTextErrors,
}: {
  settings: readonly OptionSettings[];
  sources: EnvironmentSources;
  environment: Environment;
  errors: readonly ValidationError[];
  onSpec: (spec: Spec) => void;
  onTextErrors: (errors: ValidationError[]) => void;
}) {
  const textErrors = useRef<Record<string, ValidationError[]>>({});
  const reportText = (key: string, found: ValidationError[]) => {
    textErrors.current[key] = found;
    onTextErrors(Object.values(textErrors.current).flat());
  };
  const shown = settings
    .flatMap((o) => o.parameters.map((p) => `${o.registration}/${p.name}`))
    .join("\n");
  // A parameter that goes, because its option is no longer allowed, takes
  // the errors of its text with it.
  useEffect(() => {
    const keys = new Set(shown.split("\n"));
    const kept = Object.entries(textErrors.current).filter(([key]) =>
      keys.has(key),
    );
    if (kept.length === Object.keys(textErrors.current).length) return;
    textErrors.current = Object.fromEntries(kept);
    onTextErrors(kept.flatMap(([, found]) => found));
  }, [shown, onTextErrors]);
  if (settings.length === 0)
    return (
      <p className="text-sm text-muted">
        Allow an option in a section to set its parameters.
      </p>
    );
  return (
    <div className="flex flex-col gap-6">
      {settings.map((option) => (
        <section
          key={option.registration}
          aria-label={option.label}
          className="flex flex-col gap-4 rounded-md border border-border bg-panel-2 p-4"
        >
          <header className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
            <h3 className="font-heading text-sm font-semibold">
              {option.label}
            </h3>
            <span className="font-mono text-xs text-muted">
              {option.registration}
            </span>
          </header>
          <LimitsList limits={option.limits} />
          {option.parameters.length === 0 && (
            <p className="text-sm text-muted">This option has no parameters.</p>
          )}
          {option.parameters.map((setting) => (
            <ParameterSettingRow
              key={setting.name}
              option={option}
              setting={setting}
              sources={sources}
              environment={environment}
              errors={errors}
              onSpec={onSpec}
              onTextErrors={(found) =>
                reportText(`${option.registration}/${setting.name}`, found)
              }
            />
          ))}
        </section>
      ))}
    </div>
  );
}

function ParameterSettingRow({
  option,
  setting,
  sources,
  environment,
  errors,
  onSpec,
  onTextErrors,
}: {
  option: OptionSettings;
  setting: ParameterSetting;
  sources: EnvironmentSources;
  environment: Environment;
  errors: readonly ValidationError[];
  onSpec: (spec: Spec) => void;
  onTextErrors: (errors: ValidationError[]) => void;
}) {
  const { spec } = environment;
  const base = `spec.parameters.${option.registration}.${setting.name}`;
  const valueErrors = useRef<ValidationError[]>([]);
  const allowedErrors = useRef<ValidationError[]>([]);
  const report = () =>
    onTextErrors([...valueErrors.current, ...allowedErrors.current]);
  const set = (policy: {
    value?: unknown;
    claimEditable?: boolean;
    allowed?: NamedValue[];
  }) =>
    onSpec(
      withParameterPolicy(spec, option.registration, setting.name, {
        value: setting.value,
        claimEditable: setting.claimEditable,
        allowed: setting.allowed,
        ...policy,
      }),
    );
  const others = option.parameters
    .map((p) => p.name)
    .filter((n) => n !== setting.name);
  const registration = sources.registrations.find(
    (r) => r.metadata.name === option.registration,
  );
  const claims = previewClaims(
    sources,
    environment.metadata.name ?? "",
    option.registration,
  );
  const presetWords = setting.presetKind && PRESET_WORDS[setting.presetKind];
  const presets = setting.presetKind ? presetsOf(spec, setting.presetKind) : [];
  // A value that the list no longer holds shows, with its error, so the
  // admin can see it and pick another.
  const retained =
    typeof setting.value === "string" && !presets.includes(setting.value)
      ? [{ name: setting.value, label: `${setting.value} (not allowed)` }]
      : [];
  const [generation, setGeneration] = useState(0);
  const allowedPrefix = `${base}.allowed.`;
  // A cleared text field gives "", which is no value, unless "" is a member
  // of the enum.
  const emptyMember =
    Array.isArray(setting.schema?.enum) && setting.schema.enum.includes("");

  return (
    <div
      role="group"
      aria-label={setting.label}
      className="flex min-w-0 flex-col gap-2 border-t border-divider pt-3"
    >
      {setting.template ? (
        <TemplateEditor
          key={generation}
          label={setting.label}
          value={setting.value}
          object={isStructured(setting.schema)}
          parameters={others}
          onChange={(value, found) => {
            valueErrors.current = value === undefined ? found : [];
            report();
            set({ value });
          }}
        />
      ) : (
        <ParameterField
          key={generation}
          id={`${option.registration}-${setting.name}`}
          rule={{
            name: setting.name,
            label: setting.label,
            claimEditable: true,
            ...(setting.schema && { schema: setting.schema }),
            ...(presetWords && {
              allowed: [
                ...retained,
                ...presets.map((name) => ({ name, label: name })),
              ],
            }),
          }}
          value={setting.value}
          error={errors.find((e) => e.path === `${base}.value`)?.message}
          onValue={(value, parse) => {
            valueErrors.current = parse ?? [];
            report();
            set({ value: value === "" && !emptyMember ? undefined : value });
          }}
        />
      )}
      {setting.value === undefined ? (
        <p className="text-xs text-muted">
          From the registration: {setting.registrationSummary}
        </p>
      ) : (
        <Button
          variant="ghost"
          size="sm"
          className="self-start"
          onClick={() => {
            valueErrors.current = [];
            report();
            set({ value: undefined });
            setGeneration((g) => g + 1);
          }}
        >
          Use the value of the registration
        </Button>
      )}
      {presetWords && presets.length === 0 && (
        <p className="text-xs text-warn">
          No {presetWords} preset is allowed. A claim that picks this option is
          refused until you check one under Presets.
        </p>
      )}
      {setting.hostPattern && registration && (
        <HostLine
          claims={claims}
          parameter={setting.name}
          patterns={accessPatterns(registration, environment) ?? {}}
        />
      )}
      {setting.template && setting.value !== undefined && registration && (
        <TemplatePreview
          claims={claims}
          render={(claim) => {
            const section = sources.sections.find(
              (s) =>
                registration.spec.sections.includes(s.metadata.name ?? "") &&
                s.spec.appliesTo.includes(claim.kind),
            );
            if (!section)
              return { error: "No section of this option applies.", path: "" };
            return renderJSON(
              setting.value,
              templateData({
                claim,
                section,
                registration,
                environment,
                registrations: sources.registrations,
                requests: sources.requests,
              }),
              "parameter",
            );
          }}
        />
      )}
      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          className="size-4 accent-accent"
          checked={setting.claimEditable}
          onChange={(e) => {
            if (!e.target.checked) {
              allowedErrors.current = [];
              report();
            }
            set({ claimEditable: e.target.checked });
          }}
        />
        A claim can change it
      </label>
      {setting.claimEditable && (
        <AllowedEditor
          setting={setting}
          parameters={others}
          errors={errors
            .filter((e) => e.path.startsWith(allowedPrefix))
            .map((e) => ({ ...e, path: e.path.slice(allowedPrefix.length) }))}
          onAllowed={(allowed) => set({ allowed })}
          onTextErrors={(found) => {
            allowedErrors.current = found;
            report();
          }}
        />
      )}
    </div>
  );
}

function HostLine({
  claims,
  parameter,
  patterns,
}: {
  claims: ReturnType<typeof previewClaims>;
  parameter: string;
  patterns: Record<string, unknown>;
}) {
  const claim = claims[0];
  if (!claim) return null;
  const host = renderHosts(patterns, claim.name, claim.namespace).find(
    (h) => h.parameter === parameter,
  )?.host;
  if (!host) return null;
  return (
    <p className="text-xs text-muted">
      {`${claim.name} in ${claim.namespace} gets ${host}`}
    </p>
  );
}
