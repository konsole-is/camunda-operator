"use client";

import { type ReactNode, useId } from "react";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import {
  SecretRefPicker,
  type SecretRefs,
} from "@/components/fleet/secret-ref-picker";
import { Input } from "@/components/ui/input";
import {
  type ExternalKeycloakDraft,
  type IdentityModeId,
  oidcDisabledReason,
} from "@/lib/planes/identity";
import { cn } from "@/lib/utils";

const MODES: { id: IdentityModeId; title: string; summary: string }[] = [
  {
    id: "keycloak",
    title: "Keycloak, run by this plane",
    summary: "The plane runs Keycloak through the Keycloak Operator.",
  },
  {
    id: "externalKeycloak",
    title: "External Keycloak",
    summary: "The plane uses a Keycloak that you run.",
  },
  {
    id: "oidc",
    title: "OIDC from the platform config",
    summary: "Users sign in at the OIDC provider of the platform config.",
  },
];

/**
 * IdentityCards is the choice of where the users of a plane sign in. The
 * OIDC card cannot be picked while platformConfig does not use OIDC
 * authentication, and shows why.
 */
export function IdentityCards({
  value,
  platformConfig,
  onChange,
}: {
  value?: IdentityModeId;
  platformConfig?: CamundaPlatformConfig;
  onChange: (mode: IdentityModeId) => void;
}) {
  const id = useId();
  const oidcOff = oidcDisabledReason(platformConfig);
  return (
    <div className="flex min-w-0 flex-col gap-3">
      <h2 id={`${id}-label`} className="font-heading text-base font-semibold">
        Where users sign in
      </h2>
      <div
        role="radiogroup"
        aria-labelledby={`${id}-label`}
        className="grid gap-3 sm:grid-cols-3"
      >
        {MODES.map((mode) => {
          const disabled = mode.id === "oidc" && oidcOff !== undefined;
          const checked = value === mode.id;
          return (
            <label
              key={mode.id}
              className={cn(
                "flex min-w-0 gap-3 rounded-md border bg-panel p-3",
                checked ? "border-accent" : "border-border",
                disabled ? "cursor-not-allowed" : "cursor-pointer",
              )}
            >
              <input
                type="radio"
                name={`${id}-mode`}
                value={mode.id}
                checked={checked}
                disabled={disabled}
                onChange={() => onChange(mode.id)}
                className="mt-1 size-4 shrink-0 accent-[var(--accent)]"
              />
              <span className="flex min-w-0 flex-col gap-1">
                <span
                  className={cn(
                    "text-sm font-medium",
                    disabled ? "text-muted" : "text-text",
                  )}
                >
                  {mode.title}
                </span>
                <span className="text-sm text-muted">
                  {disabled ? oidcOff : mode.summary}
                </span>
              </span>
            </label>
          );
        })}
      </div>
    </div>
  );
}

/**
 * ExternalKeycloakFields edits the Keycloak that a plane uses in External
 * Keycloak mode. secrets are the Secrets of the namespace of the plane,
 * none for a new namespace.
 */
export function ExternalKeycloakFields({
  value,
  secrets,
  namespaceIsNew,
  onChange,
}: {
  value: ExternalKeycloakDraft;
  secrets: SecretRefs;
  namespaceIsNew: boolean;
  onChange: (value: ExternalKeycloakDraft) => void;
}) {
  const id = useId();
  const field = (key: "url" | "realm" | "adminSecret", text: string) =>
    onChange({ ...value, [key]: text });
  return (
    <section className="flex max-w-md min-w-0 flex-col gap-4">
      <h2 className="font-heading text-base font-semibold">
        External Keycloak
      </h2>
      <Field id={`${id}-url`} label="Keycloak URL">
        <p id={`${id}-url-hint`} className="text-sm text-muted">
          The URL from inside the Kubernetes cluster, with the /auth path when
          Keycloak has one.
        </p>
        <Input
          id={`${id}-url`}
          value={value.url}
          autoComplete="off"
          spellCheck={false}
          placeholder="https://keycloak.example.com/auth"
          aria-describedby={`${id}-url-hint`}
          onChange={(event) => field("url", event.target.value)}
        />
      </Field>
      <Field id={`${id}-realm`} label="Realm">
        <Input
          id={`${id}-realm`}
          value={value.realm}
          autoComplete="off"
          spellCheck={false}
          onChange={(event) => field("realm", event.target.value)}
        />
      </Field>
      <Field id={`${id}-admin`} label="Administrator credentials Secret">
        <p id={`${id}-admin-hint`} className="text-sm text-muted">
          The Secret holds the keys username and password. Create it in the
          namespace before the plane starts.
        </p>
        <Input
          id={`${id}-admin`}
          value={value.adminSecret}
          list={`${id}-secrets`}
          autoComplete="off"
          spellCheck={false}
          aria-describedby={`${id}-admin-hint`}
          onChange={(event) => field("adminSecret", event.target.value)}
        />
        <datalist id={`${id}-secrets`}>
          {secrets.map((s) => (
            <option key={s.name} value={s.name} />
          ))}
        </datalist>
      </Field>
      <div className="flex flex-col gap-1">
        <span className="text-sm font-medium text-text">
          CA bundle (optional)
        </span>
        <p className="text-sm text-muted">
          {namespaceIsNew
            ? "A new namespace has no Secrets yet. Set the CA bundle later on the Claim tab."
            : "Set it when a public authority did not sign the certificate of Keycloak. It needs an https URL."}
        </p>
        <SecretRefPicker
          label="CA bundle"
          secrets={secrets}
          defaultValue={value.caBundle}
          onChange={(ref) => onChange({ ...value, caBundle: ref })}
        />
      </div>
    </section>
  );
}

function Field({
  id,
  label,
  children,
}: {
  id: string;
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-sm font-medium text-text">
        {label}
      </label>
      {children}
    </div>
  );
}
