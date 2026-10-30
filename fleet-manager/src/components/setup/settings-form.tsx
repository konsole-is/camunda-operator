"use client";

import { useRouter } from "next/navigation";
import { type ReactNode, useId, useState } from "react";
import {
  Toast,
  type ToastResult,
  toastResultOf,
} from "@/components/fleet/toast";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { gitBase } from "@/lib/git-layout";
import type { KindInfo } from "@/lib/k8s/kinds";
import type { SettingKey, Settings, TestResult } from "@/lib/settings";
import type { WriteResult } from "@/lib/write/types";
import { GitLayoutPreview } from "./git-layout-preview";
import { SecretNameSelect } from "./secret-name-select";

export type SettingsFormProps = {
  settings: Settings;
  /** resourceVersion is the version of the ConfigMap that settings come from. */
  resourceVersion: string;
  namespace: string;
  /** secretNames are the Secrets of namespace, or the error of the list. */
  secretNames: string[] | { error: string };
  layoutKinds: { namespaced: KindInfo; clusterScoped: KindInfo };
  save: (
    patch: Partial<Settings>,
    resourceVersion: string,
  ) => Promise<WriteResult>;
  testGit: () => Promise<TestResult>;
  testPrometheus: () => Promise<TestResult>;
};

const HEADING = "font-heading text-base font-semibold";
const SAVE_FIRST = "Save first. The test uses the saved settings.";

/**
 * SettingsForm shows the settings and saves the keys that the admin
 * changed, with the resourceVersion that the page read. A new
 * resourceVersion resets the form to settings, but keeps a value or the
 * Git mode that the admin changed after a save that succeeded started.
 * Git mode is on when git.repository is set; turning it off saves
 * git.repository as "". The connection tests use the saved settings, so
 * they are off while the form has a change.
 */
export function SettingsForm({
  settings,
  resourceVersion,
  namespace,
  secretNames,
  layoutKinds,
  save,
  testGit,
  testPrometheus,
}: SettingsFormProps) {
  const router = useRouter();
  const [version, setVersion] = useState(resourceVersion);
  const [values, setValues] = useState(settings);
  const [gitOn, setGitOn] = useState(settings["git.repository"] !== "");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [toast, setToast] = useState<ToastResult>();
  const [stale, setStale] = useState(false);
  const [sent, setSent] = useState<{ values: Settings; gitOn: boolean }>();

  if (version !== resourceVersion) {
    const editedSinceSave = (key: SettingKey) =>
      sent !== undefined && values[key] !== sent.values[key];
    setVersion(resourceVersion);
    setValues(
      Object.fromEntries(
        (Object.keys(settings) as SettingKey[]).map((key) => [
          key,
          editedSinceSave(key) ? values[key] : settings[key],
        ]),
      ) as Settings,
    );
    setGitOn(
      sent !== undefined && gitOn !== sent.gitOn
        ? gitOn
        : settings["git.repository"] !== "",
    );
    setSent(undefined);
    setFieldErrors({});
    setStale(false);
  }

  const effective: Settings = {
    ...values,
    "git.repository": gitOn ? values["git.repository"] : "",
  };
  const patch = Object.fromEntries(
    Object.entries(effective).filter(
      ([key, value]) => settings[key as SettingKey] !== value,
    ),
  ) as Partial<Settings>;
  const dirty = Object.keys(patch).length > 0;
  const noRepository = gitOn && values["git.repository"].trim() === "";
  const badPath = gitOn && gitBase(values["git.path"]) === undefined;

  const set = (key: SettingKey) => (value: string) =>
    setValues((v) => ({ ...v, [key]: value }));

  async function submit() {
    // A save that succeeded stays pending until the refresh brings the new
    // resourceVersion, so a second save cannot send the old one.
    setSent({ values, gitOn });
    let result: WriteResult;
    try {
      result = await save(patch, resourceVersion);
    } catch (error) {
      setSent(undefined);
      setToast({
        kind: "error",
        message: `Could not save the settings: ${errorText(error)}`,
      });
      return;
    }
    setToast(toastResultOf(result, "Saved the settings."));
    if (result.ok) {
      setFieldErrors({});
      setStale(false);
      router.refresh();
      return;
    }
    setSent(undefined);
    setFieldErrors(
      Object.fromEntries(
        result.errors
          .filter((e) => e.code === "Invalid" && e.path)
          .map((e) => [e.path!, e.message]),
      ),
    );
    setStale(result.errors.some((e) => e.code === "Stale"));
  }

  const names = Array.isArray(secretNames) ? secretNames : undefined;

  const field = (key: SettingKey, label: string) => ({
    label,
    settingKey: key,
    value: values[key],
    error: fieldErrors[key],
    onChange: set(key),
  });

  return (
    <form
      className="max-w-2xl space-y-8"
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      {!names && (
        <p className="text-sm text-warn">
          Could not list the Secrets of namespace {namespace}:{" "}
          {(secretNames as { error: string }).error}
        </p>
      )}
      <section className="space-y-3">
        <h2 className={HEADING}>Kubernetes cluster</h2>
        <TextField
          {...field("kubernetesClusterName", "Kubernetes cluster name")}
        />
      </section>

      <section className="space-y-3">
        <h2 className={HEADING}>Git mode</h2>
        <div className="flex items-start gap-2">
          <input
            type="checkbox"
            id="git-mode"
            aria-describedby="git-mode-help"
            checked={gitOn}
            onChange={(event) => setGitOn(event.target.checked)}
            className="mt-0.5 size-4 shrink-0 accent-accent focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
          />
          <div className="space-y-0.5">
            <label htmlFor="git-mode" className="text-sm font-medium text-text">
              Git mode
            </label>
            <p id="git-mode-help" className="text-sm text-muted">
              The pages push each change to a Git repository, and a GitOps tool
              applies it. Off, the pages write to the cluster.
            </p>
          </div>
        </div>
        {gitOn && (
          <div className="space-y-3 border-l-2 border-divider pl-4">
            <p className="text-sm text-text-2">
              Settings stay in the cluster. A save writes the ConfigMap
              fleet-manager-settings and is never pushed to Git.
            </p>
            <TextField
              {...field("git.repository", "Repository")}
              hint={
                noRepository
                  ? "Set the repository to turn on Git mode."
                  : undefined
              }
            />
            <TextField {...field("git.branch", "Branch")} />
            <TextField {...field("git.path", "Path")} />
            <GitLayoutPreview path={values["git.path"]} {...layoutKinds} />
            <SecretField
              {...field("git.credentialsSecretRef", "Git credentials Secret")}
              names={names}
              namespace={namespace}
            />
            <SelectField
              {...field("secretStore.kind", "SecretStore kind")}
              options={[
                ["", "None"],
                ["SecretStore", "SecretStore"],
                ["ClusterSecretStore", "ClusterSecretStore"],
              ]}
            />
            <TextField {...field("secretStore.name", "SecretStore name")} />
            <TestButton
              key={resourceVersion}
              label="Test the connection"
              dirty={dirty}
              run={testGit}
            />
          </div>
        )}
      </section>

      <section className="space-y-3">
        <h2 className={HEADING}>Prometheus</h2>
        {settings["prometheus.url"] === "" && (
          <p className="text-sm text-muted">
            Charts do not show. Everything else works.
          </p>
        )}
        <TextField {...field("prometheus.url", "Prometheus URL")} />
        <SecretField
          {...field(
            "prometheus.credentialsSecretRef",
            "Prometheus credentials Secret",
          )}
          names={names}
          namespace={namespace}
        />
        <TestButton
          key={resourceVersion}
          label="Test the query"
          dirty={dirty}
          run={testPrometheus}
        />
      </section>

      <div className="flex flex-wrap items-start gap-3">
        <Button
          type="submit"
          disabled={!dirty || noRepository || badPath || sent !== undefined}
        >
          Save
        </Button>
        {toast && (
          <div className="flex flex-wrap items-start gap-2">
            <Toast result={toast} onDismiss={() => setToast(undefined)} />
            {stale && (
              <Button
                type="button"
                variant="outline"
                onClick={() => router.refresh()}
              >
                Reload
              </Button>
            )}
          </div>
        )}
      </div>
    </form>
  );
}

type FieldProps = {
  label: string;
  settingKey: SettingKey;
  value: string;
  error?: string;
  hint?: string;
  onChange: (value: string) => void;
};

function FieldFrame({
  id,
  label,
  settingKey,
  error,
  hint,
  children,
}: Omit<FieldProps, "value" | "onChange"> & {
  id: string;
  children: ReactNode;
}) {
  return (
    <div className="space-y-1">
      <label htmlFor={id} className="text-sm font-medium text-text">
        {label}
      </label>
      {children}
      <p id={`${id}-key`} className="text-xs text-muted">
        Key <code className="font-mono">{settingKey}</code>
      </p>
      {hint && (
        <p id={`${id}-hint`} className="text-sm text-warn">
          {hint}
        </p>
      )}
      {error && (
        <p id={`${id}-error`} className="text-sm text-bad">
          {error}
        </p>
      )}
    </div>
  );
}

function describedBy(id: string, error?: string, hint?: string): string {
  return [`${id}-key`, hint && `${id}-hint`, error && `${id}-error`]
    .filter(Boolean)
    .join(" ");
}

function TextField(props: FieldProps) {
  const id = useId();
  return (
    <FieldFrame id={id} {...props}>
      <Input
        id={id}
        value={props.value}
        aria-invalid={props.error ? true : undefined}
        aria-describedby={describedBy(id, props.error, props.hint)}
        onChange={(event) => props.onChange(event.target.value)}
      />
    </FieldFrame>
  );
}

function SelectField(props: FieldProps & { options: [string, string][] }) {
  const id = useId();
  return (
    <FieldFrame id={id} {...props}>
      <NativeSelect
        id={id}
        value={props.value}
        aria-invalid={props.error ? true : undefined}
        aria-describedby={describedBy(id, props.error)}
        onChange={(event) => props.onChange(event.target.value)}
      >
        {props.options.map(([value, text]) => (
          <NativeSelectOption key={value} value={value}>
            {text}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    </FieldFrame>
  );
}

function SecretField(
  props: FieldProps & { names?: string[]; namespace: string },
) {
  return (
    <div className="space-y-1">
      <SecretNameSelect
        label={props.label}
        value={props.value}
        names={props.names}
        namespace={props.namespace}
        onChange={props.onChange}
      />
      <p className="text-xs text-muted">
        Key <code className="font-mono">{props.settingKey}</code>
      </p>
      {props.error && <p className="text-sm text-bad">{props.error}</p>}
    </div>
  );
}

function TestButton({
  label,
  dirty,
  run,
}: {
  label: string;
  dirty: boolean;
  run: () => Promise<TestResult>;
}) {
  const id = useId();
  const [result, setResult] = useState<TestResult>();
  const [running, setRunning] = useState(false);
  return (
    <div className="space-y-1">
      <Button
        type="button"
        variant="outline"
        disabled={dirty || running}
        aria-describedby={dirty ? `${id}-dirty` : undefined}
        onClick={async () => {
          setRunning(true);
          try {
            setResult(await run());
          } catch (error) {
            setResult({
              ok: false,
              message: `Could not run the test: ${errorText(error)}`,
            });
          } finally {
            setRunning(false);
          }
        }}
      >
        {label}
      </Button>
      {dirty && (
        <p id={`${id}-dirty`} className="text-sm text-muted">
          {SAVE_FIRST}
        </p>
      )}
      {result && (
        <p
          role={result.ok ? "status" : "alert"}
          className={`text-sm ${result.ok ? "text-ok" : "text-bad"}`}
        >
          {result.message}
        </p>
      )}
    </div>
  );
}

function errorText(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
