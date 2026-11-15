"use client";

import { type ReactNode, useRef } from "react";
import { EnvEditor } from "@/components/fleet/env-editor";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import { YamlEditor } from "@/components/fleet/yaml-editor";
import type { CamundaReleaseSpec } from "@/generated/crds/camundarelease";
import { fullSchemaOf } from "@/lib/fields/schema";
import type { ValidationError } from "@/lib/schema";
import {
  ENV_SCOPES,
  type EnvFromSource,
  type EnvScope,
  envOf,
  type ScopeEnv,
  withEnv,
} from "./env";

const EXTRA_ENV_FROM = fullSchemaOf("extraEnvFrom");

/**
 * EnvScopes edits the env lists of each scope of a release: extraEnv as
 * rows of a name with a value or a Secret key, and extraEnvFrom as YAML.
 * The editors read spec when they mount, so a caller that replaces the
 * spec gives the component a new key. After each edit onChange gets a
 * function that applies the edit to a spec, and the errors of every
 * editor. An error path starts with the scope and the list, such as
 * "zeebe.extraEnv.0.name".
 */
export function EnvScopes({
  spec,
  secrets,
  missingIn,
  onChange,
}: {
  spec: CamundaReleaseSpec;
  secrets: SecretRefs;
  missingIn: (name: string, key: string) => readonly string[];
  onChange: (
    update: (spec: CamundaReleaseSpec) => CamundaReleaseSpec,
    errors: ValidationError[],
  ) => void;
}): ReactNode {
  const errors = useRef<Record<string, ValidationError[]>>({});
  const report = (
    scope: EnvScope,
    list: "extraEnv" | "extraEnvFrom",
    edit: ScopeEnv,
    listErrors: ValidationError[],
  ) => {
    const prefix = `${scope}.${list}`;
    errors.current[prefix] = listErrors.map((e) => ({
      path: e.path ? `${prefix}.${e.path}` : prefix,
      message: e.message,
    }));
    onChange(
      (current) =>
        withEnv(current, scope, { ...envOf(current, scope), ...edit }),
      Object.values(errors.current).flat(),
    );
  };

  return (
    <div className="flex flex-col gap-2">
      {ENV_SCOPES.map(({ scope, label }) => {
        const env = envOf(spec, scope);
        const count =
          (env.extraEnv?.length ?? 0) + (env.extraEnvFrom?.length ?? 0);
        return (
          <details
            key={scope}
            open={count > 0}
            className="rounded-md border border-border"
          >
            <summary className="flex cursor-pointer items-center gap-2 px-4 py-2.5 text-sm font-medium select-none">
              <span>{label}</span>
              <span className="rounded-full bg-panel-2 px-2 py-0.5 text-xs text-muted">
                {count}
              </span>
            </summary>
            <div className="flex flex-col gap-4 border-t border-divider px-4 py-4">
              <EnvEditor
                label={`Variables of ${label}`}
                secrets={secrets}
                missingIn={missingIn}
                defaultValue={env.extraEnv}
                onChange={(extraEnv, listErrors) =>
                  report(scope, "extraEnv", { extraEnv }, listErrors)
                }
              />
              <YamlEditor
                label={`extraEnvFrom of ${label}`}
                rows={4}
                schema={EXTRA_ENV_FROM}
                defaultValue={env.extraEnvFrom}
                onChange={(value, listErrors) =>
                  report(
                    scope,
                    "extraEnvFrom",
                    {
                      extraEnvFrom: Array.isArray(value)
                        ? (value as EnvFromSource[])
                        : undefined,
                    },
                    listErrors,
                  )
                }
              />
            </div>
          </details>
        );
      })}
    </div>
  );
}
