"use client";

import { type ReactNode, useRef, useState } from "react";
import { DisclosureGroup } from "@/components/fleet/disclosure-group";
import { EnvEditor } from "@/components/fleet/env-editor";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import { YamlEditor } from "@/components/fleet/yaml-editor";
import { fullSchemaOf } from "@/lib/fields/schema";
import type { ValidationError } from "@/lib/schema";
import {
  ENV_SCOPES,
  type EnvFromSource,
  type EnvScope,
  type EnvSpec,
  envOf,
  type ScopeEnv,
  withEnv,
} from "./env";

const EXTRA_ENV_FROM = fullSchemaOf("extraEnvFrom");

/**
 * EnvScopes edits the env lists of each scope of spec, a release or the
 * body of a cluster preset: extraEnv as rows of a name with a value or a
 * Secret key, and extraEnvFrom as YAML. A scope with entries starts open.
 * The editors read spec once, so a caller gives the component a new key
 * when it replaces the spec. After each edit onChange gets a function that
 * applies the edit to a spec, and the errors of every editor. While the
 * YAML of a scope has an error, the function keeps its extraEnvFrom. An
 * error path starts with the scope and the list, such as
 * "zeebe.extraEnv.0.name".
 */
export function EnvScopes<S extends EnvSpec>({
  spec,
  secrets,
  missingIn,
  onChange,
}: {
  spec: S;
  secrets: SecretRefs;
  missingIn: (name: string, key: string) => readonly string[];
  onChange: (update: (spec: S) => S, errors: ValidationError[]) => void;
}): ReactNode {
  const [open, setOpen] = useState<ReadonlySet<EnvScope>>(
    () =>
      new Set(
        ENV_SCOPES.map((s) => s.scope).filter((scope) => {
          const env = envOf(spec, scope);
          return (
            (env.extraEnv?.length ?? 0) + (env.extraEnvFrom?.length ?? 0) > 0
          );
        }),
      ),
  );
  const toggle = (scope: EnvScope) =>
    setOpen((current) => {
      const next = new Set(current);
      if (!next.delete(scope)) next.add(scope);
      return next;
    });
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
          <DisclosureGroup
            key={scope}
            label={label}
            counts={[{ text: String(count) }]}
            open={open.has(scope)}
            onToggle={() => toggle(scope)}
          >
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
                  listErrors.length > 0
                    ? {}
                    : {
                        extraEnvFrom: Array.isArray(value)
                          ? (value as EnvFromSource[])
                          : undefined,
                      },
                  listErrors,
                )
              }
            />
          </DisclosureGroup>
        );
      })}
    </div>
  );
}
