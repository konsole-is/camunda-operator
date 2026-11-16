"use client";

import type { ReactNode } from "react";
import {
  type NamespacedSecretRef,
  NamespacedSecretRefPicker,
} from "@/components/fleet/namespaced-secret-ref-picker";
import {
  EditorSection,
  ErrorText,
  SelectField,
  TextField,
} from "@/components/setup/form-parts";
import { Radio } from "@/components/ui/radio";
import type { ValidationError } from "@/lib/schema";
import {
  MANAGEMENT_CLIENTS,
  type ManagementClients,
  type PlatformDraft,
} from "./draft";

type Oidc = NonNullable<NonNullable<PlatformDraft["auth"]>["oidc"]>;
type OidcText = Exclude<
  keyof Oidc,
  "clientSecretRef" | "management" | "providerType"
>;
type Client = {
  clientId?: string;
  audience?: string;
  publicApiAudience?: string;
  clientSecretRef?: NamespacedSecretRef;
};

const OIDC_PATH = "spec.auth.oidc";
const DISCOVERY_HINT =
  "Empty means the value of the discovery document. A management plane needs all three.";
const PROVIDERS = [
  ["", "Generic"],
  ["microsoft", "Microsoft Entra ID"],
] as const;

/**
 * AuthSection edits the authentication method, the OIDC fields, the
 * management plane clients, and the license Secret of draft. The OIDC
 * fields and the clients show only in OIDC mode. A Secret reference can
 * name a Secret in any of namespaces. Each field shows its error of errors
 * by spec path.
 */
export function AuthSection({
  draft,
  namespaces,
  errors,
  onChange,
}: {
  draft: PlatformDraft;
  namespaces: readonly string[];
  errors: ValidationError[];
  onChange: (draft: PlatformDraft) => void;
}): ReactNode {
  const oidcMode = draft.auth?.method === "oidc";
  const oidc = (draft.auth?.oidc ?? {}) as Partial<Oidc>;
  const errorOf = (path: string) =>
    errors.find((e) => e.path === path)?.message;
  const setOidc = (next: Partial<Oidc>) =>
    onChange({ ...draft, auth: { ...draft.auth, oidc: next as Oidc } });
  const text = (field: OidcText, label: string, hint?: string) => (
    <TextField
      label={label}
      value={oidc[field] ?? ""}
      hint={hint}
      error={errorOf(`${OIDC_PATH}.${field}`)}
      onChange={(value) => setOidc({ ...oidc, [field]: value })}
    />
  );
  const clients = (oidc.management?.clients ?? {}) as Record<
    keyof ManagementClients,
    Client | undefined
  >;
  const setClient = (key: keyof ManagementClients, client: Client) =>
    setOidc({
      ...oidc,
      management: {
        clients: { ...clients, [key]: client } as ManagementClients,
      },
    });

  return (
    <>
      <EditorSection title="Authentication">
        <fieldset className="flex flex-col gap-2">
          <legend className="mb-1 text-sm font-medium text-text">
            Authentication method
          </legend>
          {(
            [
              ["basic", "Basic"],
              ["oidc", "OIDC"],
            ] as const
          ).map(([method, label]) => (
            <label
              key={method}
              className="flex min-h-6 items-center gap-2 text-sm"
            >
              <Radio
                name="auth-method"
                checked={(oidcMode ? "oidc" : "basic") === method}
                onChange={() =>
                  onChange({ ...draft, auth: { ...draft.auth, method } })
                }
              />
              {label}
            </label>
          ))}
        </fieldset>
        {oidcMode && (
          <>
            {text("issuerUrl", "Issuer URL")}
            <SelectField
              label="Provider type"
              value={oidc.providerType ?? ""}
              options={PROVIDERS}
              onChange={(value) =>
                setOidc({
                  ...oidc,
                  providerType: (value || undefined) as Oidc["providerType"],
                })
              }
            />
            {text("clientId", "Client ID")}
            {text("audience", "Audience", "Empty means the client ID.")}
            <SecretField
              label="Client secret"
              namespaces={namespaces}
              value={oidc.clientSecretRef}
              error={errorOf(`${OIDC_PATH}.clientSecretRef`)}
              onChange={(clientSecretRef) =>
                setOidc({ ...oidc, clientSecretRef } as Partial<Oidc>)
              }
            />
            {text("usernameClaim", "Username claim", "Empty means sub.")}
            {text("clientIdClaim", "Client ID claim")}
            {text("jwksUrl", "JWKS URL", DISCOVERY_HINT)}
            {text("tokenUrl", "Token URL", DISCOVERY_HINT)}
            {text("authUrl", "Authorization URL", DISCOVERY_HINT)}
          </>
        )}
      </EditorSection>
      {oidcMode && (
        <EditorSection title="Management plane clients">
          {MANAGEMENT_CLIENTS.map(({ key, label, secret }) => {
            const client = clients[key] ?? {};
            const path = `${OIDC_PATH}.management.clients.${key}`;
            const field = (
              name: "clientId" | "audience" | "publicApiAudience",
              fieldLabel: string,
              hint?: string,
            ) => (
              <TextField
                label={fieldLabel}
                value={client[name] ?? ""}
                hint={hint}
                error={errorOf(`${path}.${name}`)}
                onChange={(value) =>
                  setClient(key, { ...client, [name]: value })
                }
              />
            );
            return (
              <fieldset
                key={key}
                className="flex min-w-0 flex-col gap-3 border-t border-border pt-3 first:border-t-0 first:pt-0"
              >
                <legend className="text-sm font-semibold text-text">
                  {label}
                </legend>
                {field("clientId", "Client ID")}
                {field("audience", "Audience")}
                {secret && (
                  <SecretField
                    label="Client secret"
                    namespaces={namespaces}
                    value={client.clientSecretRef}
                    error={errorOf(`${path}.clientSecretRef`)}
                    onChange={(clientSecretRef) =>
                      setClient(key, { ...client, clientSecretRef })
                    }
                  />
                )}
                {key === "webModelerApi" &&
                  field(
                    "publicApiAudience",
                    "Public API audience",
                    "Empty means web-modeler-public-api.",
                  )}
              </fieldset>
            );
          })}
        </EditorSection>
      )}
      <EditorSection title="License">
        <SecretField
          label="License key"
          namespaces={namespaces}
          value={draft.licenseSecretRef}
          hint="Without a license, clusters run in unlicensed non-production mode."
          onChange={(licenseSecretRef) => {
            const next = { ...draft, licenseSecretRef };
            if (!licenseSecretRef) delete next.licenseSecretRef;
            onChange(next);
          }}
        />
      </EditorSection>
    </>
  );
}

function SecretField({
  label,
  namespaces,
  value,
  hint,
  error,
  onChange,
}: {
  label: string;
  namespaces: readonly string[];
  value?: NamespacedSecretRef;
  hint?: string;
  error?: string;
  onChange: (ref: NamespacedSecretRef | undefined) => void;
}) {
  return (
    <div className="flex max-w-2xl flex-col gap-1.5">
      <NamespacedSecretRefPicker
        label={label}
        namespaces={namespaces}
        value={value}
        invalid={Boolean(error)}
        onChange={onChange}
      />
      {hint && <p className="text-xs text-muted">{hint}</p>}
      <ErrorText error={error} />
    </div>
  );
}
