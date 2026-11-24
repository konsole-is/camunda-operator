"use client";

import { type ReactNode, useId } from "react";
import {
  type NamespacedSecretRef,
  NamespacedSecretRefPicker,
} from "@/components/fleet/namespaced-secret-ref-picker";
import {
  CREDENTIALS_SLOTS,
  KEY_SLOT,
  type PickedKeys,
  S3_CREDENTIALS_SLOTS,
  SecretKeysPicker,
  type SecretKeySlot,
} from "@/components/fleet/secret-keys-picker";
import {
  ErrorText,
  SelectField,
  TextField,
} from "@/components/setup/form-parts";
import { Checkbox } from "@/components/ui/checkbox";
import { Radio } from "@/components/ui/radio";
import type { DatabaseServerConfig } from "@/generated/crds/databaseserverconfig";
import type { ManagementAuthConfig } from "@/generated/crds/managementauthconfig";
import type { ObjectStorageConfig } from "@/generated/crds/objectstorageconfig";
import type { SecondaryStorageConfig } from "@/generated/crds/secondarystorageconfig";
import { isObject } from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import type { ContractKind } from "@/lib/usage";
import { type ContractDraft, withAuthType, withStorageType } from "./draft";
import { NODE_COUNT_HINT } from "./facts";

const STORAGE_TYPE = { S3: "S3", GCS: "GCS", AzureBlob: "Azure Blob" };
const SECONDARY_TYPE = { elasticsearch: "Elasticsearch", rdbms: "RDBMS" };
const AUTH_TYPE = {
  workloadIdentity: "Workload identity",
  credentials: "Credentials",
};

/**
 * ContractForm edits the values of draft, a contract of kind in namespace,
 * in the shape that the contract has. The storage type and the auth type
 * show as text, because an edit keeps them. Each field shows its error of errors by
 * spec path. The Secret pickers read the Secrets of namespace; the client
 * secret of a Management Identity contract can be in any of namespaces.
 * databaseConfigs and databaseServerConfigs are the names that the pickers
 * of a reference offer, undefined while the list is not known; a name that
 * a known list lacks shows as not found.
 *
 * With choose, for a new contract, the storage type and the auth type are
 * radio choices: a pick of a storage type starts its block empty, and a
 * pick of an auth type keeps the other fields of the block.
 */
export function ContractForm({
  kind,
  namespace = "",
  draft,
  errors,
  namespaces,
  databaseConfigs,
  databaseServerConfigs,
  choose = false,
  onChange,
}: {
  kind: ContractKind;
  namespace?: string;
  draft: ContractDraft;
  errors: readonly ValidationError[];
  namespaces: readonly string[];
  databaseConfigs?: readonly string[];
  databaseServerConfigs?: readonly string[];
  choose?: boolean;
  onChange: (draft: ContractDraft) => void;
}): ReactNode {
  const spec = draft.spec as unknown as Record<string, unknown>;
  const form: Form = {
    namespace,
    choose,
    pick: onChange,
    draft,
    errorOf: (path) => errors.find((e) => e.path === `spec.${path}`)?.message,
    get: (path) => valueAt(spec, path),
    set: (path, value) =>
      onChange({
        spec: withValue(spec, path, value) as unknown as ContractDraft["spec"],
      }),
  };
  switch (kind) {
    case "ObjectStorageConfig":
      return (
        <ObjectStorageForm
          form={form}
          spec={draft.spec as ObjectStorageConfig["spec"]}
        />
      );
    case "SecondaryStorageConfig":
      return (
        <SecondaryStorageForm
          form={form}
          spec={draft.spec as SecondaryStorageConfig["spec"]}
          databaseConfigs={databaseConfigs}
        />
      );
    case "DatabaseServerConfig":
      return (
        <Fields>
          <Fixed
            label="Engine"
            value={
              (draft.spec as DatabaseServerConfig["spec"]).engine === "postgres"
                ? "PostgreSQL"
                : String((draft.spec as DatabaseServerConfig["spec"]).engine)
            }
          />
          <Text form={form} path="host" label="Host" />
          <NumberField form={form} path="port" label="Port" />
          <Keys
            form={form}
            path="adminCredentialsSecretRef"
            label="Admin credentials"
            slots={CREDENTIALS_SLOTS}
          />
        </Fields>
      );
    case "DatabaseConfig":
      return (
        <Fields>
          <Text form={form} path="databaseName" label="Database name" />
          <NameSelect
            form={form}
            path="serverRef"
            label="Database server"
            names={databaseServerConfigs}
            none="Pick a database server"
          />
          <Keys
            form={form}
            path="credentialsSecretRef"
            label="Credentials"
            slots={CREDENTIALS_SLOTS}
          />
          <Keys
            form={form}
            path="backupCredentialsSecretRef"
            label="Backup credentials"
            slots={CREDENTIALS_SLOTS}
            optional
          />
        </Fields>
      );
    case "ManagementAuthConfig":
      return (
        <ManagementIdentityForm
          form={form}
          spec={draft.spec as ManagementAuthConfig["spec"]}
          namespaces={namespaces}
        />
      );
  }
}

type Form = {
  namespace: string;
  choose: boolean;
  pick: (draft: ContractDraft) => void;
  draft: ContractDraft;
  errorOf: (path: string) => string | undefined;
  get: (path: string) => unknown;
  set: (path: string, value: unknown) => void;
};

function ObjectStorageForm({
  form,
  spec,
}: {
  form: Form;
  spec: ObjectStorageConfig["spec"];
}) {
  const block =
    spec.type === "AzureBlob"
      ? "azureBlob"
      : spec.type === "GCS"
        ? "gcs"
        : "s3";
  const auth =
    (form.get(`${block}.auth.type`) as keyof typeof AUTH_TYPE | undefined) ??
    "workloadIdentity";
  const p = (field: string) => `${block}.${field}`;
  return (
    <Fields>
      <TypeField
        form={form}
        label="Storage type"
        value={spec.type}
        options={STORAGE_TYPE}
        onPick={(type) =>
          form.pick(withStorageType("ObjectStorageConfig", type))
        }
      />
      {block === "azureBlob" ? (
        <>
          <Text form={form} path={p("accountName")} label="Account name" />
          <Text form={form} path={p("container")} label="Container" />
          <Text form={form} path={p("endpoint")} label="Endpoint" mono />
        </>
      ) : (
        <Text form={form} path={p("bucketName")} label="Bucket name" />
      )}
      {block === "s3" && (
        <>
          <Text form={form} path={p("region")} label="Region" />
          <Text form={form} path={p("endpoint")} label="Endpoint" mono />
        </>
      )}
      <Text form={form} path={p("basePath")} label="Base path" mono />
      {block === "s3" && (
        <Toggle
          form={form}
          path={p("forcePathStyle")}
          label="Path-style access"
        />
      )}
      <TypeField
        form={form}
        label="Auth"
        value={auth}
        options={AUTH_TYPE}
        onPick={(type) => form.pick(withAuthType(form.draft, type))}
      />
      {auth === "credentials" ? (
        <Keys
          form={form}
          path={p("auth.credentials.secretRef")}
          label={block === "s3" ? "Access key" : "Credentials"}
          slots={block === "s3" ? S3_CREDENTIALS_SLOTS : KEY_SLOT}
        />
      ) : block === "s3" ? (
        <Text
          form={form}
          path={p("auth.workloadIdentity.roleArn")}
          label="Role ARN"
          mono
        />
      ) : block === "gcs" ? (
        <Text
          form={form}
          path={p("auth.workloadIdentity.serviceAccountEmail")}
          label="Service account email"
        />
      ) : (
        <Text
          form={form}
          path={p("auth.workloadIdentity.clientId")}
          label="Client ID"
          mono
        />
      )}
    </Fields>
  );
}

function SecondaryStorageForm({
  form,
  spec,
  databaseConfigs,
}: {
  form: Form;
  spec: SecondaryStorageConfig["spec"];
  databaseConfigs?: readonly string[];
}) {
  return (
    <Fields>
      <TypeField
        form={form}
        label="Storage type"
        value={spec.type}
        options={SECONDARY_TYPE}
        onPick={(type) =>
          form.pick(withStorageType("SecondaryStorageConfig", type))
        }
      />
      {spec.type === "rdbms" ? (
        <NameSelect
          form={form}
          path="rdbms.databaseConfigRef"
          label="Database config"
          names={databaseConfigs}
          none="Pick a database config"
        />
      ) : (
        <>
          <Text
            form={form}
            path="elasticsearch.endpoint"
            label="Endpoint"
            mono
          />
          <NumberField
            form={form}
            path="elasticsearch.nodeCount"
            label="Node count"
            hint={NODE_COUNT_HINT}
          />
          <Text
            form={form}
            path="elasticsearch.snapshotRepository"
            label="Snapshot repository"
            mono
          />
          <Keys
            form={form}
            path="elasticsearch.credentialsSecretRef"
            label="Credentials"
            slots={CREDENTIALS_SLOTS}
          />
          <Keys
            form={form}
            path="elasticsearch.caSecretRef"
            label="CA certificate"
            slots={KEY_SLOT}
            optional
          />
        </>
      )}
    </Fields>
  );
}

function ManagementIdentityForm({
  form,
  spec,
  namespaces,
}: {
  form: Form;
  spec: ManagementAuthConfig["spec"];
  namespaces: readonly string[];
}) {
  const error = form.errorOf("clientSecretRef");
  return (
    <Fields>
      <Text form={form} path="issuerUrl" label="Issuer URL" mono />
      <Text
        form={form}
        path="issuerBackendUrl"
        label="Issuer backend URL"
        mono
      />
      <Text form={form} path="authUrl" label="Auth URL" mono />
      <Text form={form} path="tokenUrl" label="Token URL" mono />
      <Text form={form} path="jwksUrl" label="JWKS URL" mono />
      <Text form={form} path="baseUrl" label="Base URL" mono />
      <Text form={form} path="clientId" label="Client ID" mono />
      <Text form={form} path="audience" label="Audience" mono />
      <div className="flex flex-col gap-1">
        <NamespacedSecretRefPicker
          label="Client secret"
          namespaces={namespaces}
          value={spec.clientSecretRef as NamespacedSecretRef | undefined}
          invalid={error !== undefined}
          onChange={(ref) => form.set("clientSecretRef", ref)}
        />
        <ErrorText error={error} />
      </div>
    </Fields>
  );
}

function Fields({ children }: { children: ReactNode }) {
  return <div className="flex min-w-0 flex-col gap-4">{children}</div>;
}

// Fixed shows a value that an edit keeps, without a control.
function Fixed({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-sm font-medium text-text">{label}</span>
      <span className="text-sm text-muted">{value}</span>
    </div>
  );
}

function TypeField<T extends string>({
  form,
  label,
  value,
  options,
  onPick,
}: {
  form: Form;
  label: string;
  value: string;
  options: Record<T, string>;
  onPick: (type: T) => void;
}) {
  const name = useId();
  if (!form.choose)
    return <Fixed label={label} value={options[value as T] ?? String(value)} />;
  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="mb-1 text-sm font-medium text-text">{label}</legend>
      {(Object.entries(options) as [T, string][]).map(([type, text]) => (
        <label key={type} className="flex min-h-6 items-center gap-2 text-sm">
          <Radio
            name={name}
            checked={value === type}
            onChange={() => onPick(type)}
          />
          {text}
        </label>
      ))}
    </fieldset>
  );
}

function Text({
  form,
  path,
  label,
  hint,
  mono,
}: {
  form: Form;
  path: string;
  label: string;
  hint?: string;
  mono?: boolean;
}) {
  const value = form.get(path);
  return (
    <TextField
      label={label}
      value={typeof value === "string" ? value : ""}
      hint={hint}
      mono={mono}
      error={form.errorOf(path)}
      onChange={(next) => form.set(path, next === "" ? undefined : next)}
    />
  );
}

// NumberField keeps what the admin types: a value that is not a number
// stays in the draft as NaN, so draftErrors can name it.
function NumberField({
  form,
  path,
  label,
  hint,
}: {
  form: Form;
  path: string;
  label: string;
  hint?: string;
}) {
  const value = form.get(path);
  return (
    <TextField
      label={label}
      value={
        typeof value === "number" && !Number.isNaN(value) ? String(value) : ""
      }
      hint={hint}
      error={form.errorOf(path)}
      onChange={(next) =>
        form.set(path, next.trim() === "" ? undefined : Number(next))
      }
    />
  );
}

function Toggle({
  form,
  path,
  label,
}: {
  form: Form;
  path: string;
  label: string;
}) {
  const id = useId();
  return (
    <div className="flex items-center gap-2">
      <Checkbox
        id={id}
        checked={form.get(path) === true}
        onCheckedChange={(checked) =>
          form.set(path, checked === true ? true : undefined)
        }
      />
      <label htmlFor={id} className="text-sm text-text">
        {label}
      </label>
    </div>
  );
}

function Keys({
  form,
  path,
  label,
  slots,
  optional = false,
}: {
  form: Form;
  path: string;
  label: string;
  slots: readonly SecretKeySlot[];
  optional?: boolean;
}) {
  const value = form.get(path);
  const error = form.errorOf(path);
  return (
    <div className="flex flex-col gap-1">
      <SecretKeysPicker
        label={label}
        namespace={form.namespace}
        slots={slots}
        value={isObject(value) ? (value as PickedKeys) : undefined}
        invalid={error !== undefined}
        onChange={(ref) =>
          form.set(path, ref ?? (optional ? undefined : { name: "" }))
        }
      />
      <ErrorText error={error} />
    </div>
  );
}

function NameSelect({
  form,
  path,
  label,
  names,
  none,
}: {
  form: Form;
  path: string;
  label: string;
  names?: readonly string[];
  none: string;
}) {
  const value = form.get(path);
  const current = typeof value === "string" ? value : "";
  const missing =
    names !== undefined && current !== "" && !names.includes(current);
  const options: [string, string][] = [
    ["", none],
    ...(current !== "" && !(names ?? []).includes(current)
      ? [
          [current, missing ? `${current} (not found)` : current] as [
            string,
            string,
          ],
        ]
      : []),
    ...(names ?? []).map((n): [string, string] => [n, n]),
  ];
  return (
    <SelectField
      label={label}
      value={current}
      options={options}
      error={form.errorOf(path)}
      onChange={(next) => form.set(path, next === "" ? undefined : next)}
    />
  );
}

function valueAt(object: unknown, path: string): unknown {
  return path
    .split(".")
    .reduce<unknown>((at, key) => (isObject(at) ? at[key] : undefined), object);
}

// withValue returns a copy of object with value at path. An undefined
// value removes the field.
function withValue(
  object: Record<string, unknown>,
  path: string,
  value: unknown,
): Record<string, unknown> {
  const [key, ...rest] = path.split(".");
  const copy = { ...object };
  if (rest.length === 0) {
    if (value === undefined) delete copy[key];
    else copy[key] = value;
    return copy;
  }
  const child = isObject(copy[key]) ? copy[key] : {};
  copy[key] = withValue(child, rest.join("."), value);
  return copy;
}
