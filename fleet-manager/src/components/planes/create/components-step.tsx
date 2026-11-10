"use client";

import { type ReactNode, useId } from "react";
import { TriangleAlert } from "lucide-react";
import type { DraftPath } from "@/components/create/use-create-flow";
import {
  SecretRefPicker,
  type SecretRefs,
} from "@/components/fleet/secret-ref-picker";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { type MailDraft, runningOf } from "@/lib/planes/components";
import {
  type PlaneContext,
  type PlaneDraft,
  planePrefill,
  planeRelease,
  planeVersions,
} from "@/lib/planes/draft";
import {
  VERSION_LABEL,
  prefillNote,
  runningFields,
  versionWarnings,
} from "@/lib/planes/versions";

/**
 * ComponentsStep is step 3 of the plane create flow: the first
 * administrator of the identity mode, the Console and Web Modeler
 * switches, the mail of Web Modeler, and the version of each app that
 * runs. secrets are the Secrets of the namespace of draft, none for a new
 * one.
 */
export function ComponentsStep({
  draft,
  set,
  ctx,
  secrets,
}: {
  draft: PlaneDraft;
  set: (path: DraftPath, value: unknown) => void;
  ctx: PlaneContext;
  secrets: SecretRefs;
}) {
  const id = useId();
  const running = runningOf(draft);
  const versions = planeVersions(draft, ctx);
  const warnings = versionWarnings(versions, running);
  const { admin } = draft;
  const text = (path: DraftPath) => (value: string) => set(path, value);

  return (
    <div className="flex max-w-3xl min-w-0 flex-col gap-8">
      <Section title="Management Identity" note="Always runs.">
        {draft.mode === "oidc" ? (
          <>
            <TextField
              id={`${id}-claim-name`}
              label="Claim name"
              value={admin.claimName}
              placeholder="sub"
              onChange={text(["admin", "claimName"])}
            />
            <TextField
              id={`${id}-claim-value`}
              label="Claim value"
              value={admin.claimValue}
              onChange={text(["admin", "claimValue"])}
            />
          </>
        ) : (
          <>
            <TextField
              id={`${id}-username`}
              label="Username"
              value={admin.username}
              onChange={text(["admin", "username"])}
            />
            <TextField
              id={`${id}-email`}
              label="Email"
              type="email"
              value={admin.email}
              onChange={text(["admin", "email"])}
            />
            <div className="flex flex-col gap-1">
              <span className="text-sm font-medium text-text">
                Password Secret
              </span>
              <p className="text-sm text-muted">Generated if empty.</p>
              <SecretRefPicker
                label="Password Secret"
                secrets={secrets}
                defaultValue={admin.password}
                onChange={(ref) => set(["admin", "password"], ref)}
              />
            </div>
          </>
        )}
        <p className="text-sm text-muted">
          Read once at first start. Locked afterwards.
        </p>
      </Section>
      <Section title="Console">
        <Switch
          id={`${id}-console`}
          label="Run Console"
          checked={draft.console}
          onChange={(on) => set(["console"], on)}
        />
      </Section>
      <Section title="Web Modeler">
        <Switch
          id={`${id}-modeler`}
          label="Run Web Modeler"
          checked={draft.webModeler}
          onChange={(on) => set(["webModeler"], on)}
        />
        {draft.webModeler && (
          <MailFields
            mail={draft.mail}
            secrets={secrets}
            onChange={(key, value) => set(["mail", key], value)}
          />
        )}
      </Section>
      <Section title="Versions">
        {runningFields(running).map((field) => (
          <TextField
            key={field}
            id={`${id}-version-${field}`}
            label={`${VERSION_LABEL[field]} version`}
            value={versions[field] ?? ""}
            mono
            onChange={text(["versions", field])}
          />
        ))}
        <p className="text-sm text-muted">
          {prefillNote(planePrefill(draft, ctx), planeRelease(draft, ctx))}
        </p>
        {warnings.map((warning) => (
          <div
            key={warning}
            role="note"
            className="flex gap-3 rounded-md border border-warn/40 bg-warn/10 p-3 text-sm text-text"
          >
            <TriangleAlert
              aria-hidden
              className="mt-0.5 size-4 shrink-0 text-warn"
            />
            <p>{warning}</p>
          </div>
        ))}
      </Section>
    </div>
  );
}

function MailFields({
  mail,
  secrets,
  onChange,
}: {
  mail: MailDraft;
  secrets: SecretRefs;
  onChange: (key: keyof MailDraft, value: unknown) => void;
}) {
  const id = useId();
  return (
    <div className="flex flex-col gap-4 border-l-2 border-border pl-4">
      <TextField
        id={`${id}-host`}
        label="SMTP host"
        value={mail.smtpHost}
        placeholder="smtp.example.com"
        onChange={(value) => onChange("smtpHost", value)}
      />
      <TextField
        id={`${id}-port`}
        label="SMTP port"
        type="number"
        value={String(mail.smtpPort)}
        onChange={(value) => onChange("smtpPort", Number(value))}
      />
      <TextField
        id={`${id}-from`}
        label="From address"
        type="email"
        value={mail.fromAddress}
        placeholder="modeler@example.com"
        onChange={(value) => onChange("fromAddress", value)}
      />
      <TextField
        id={`${id}-from-name`}
        label="From name"
        value={mail.fromName}
        onChange={(value) => onChange("fromName", value)}
      />
      <Switch
        id={`${id}-tls`}
        label="STARTTLS"
        checked={mail.tls}
        onChange={(on) => onChange("tls", on)}
      />
      <div className="flex max-w-md flex-col gap-1">
        <label htmlFor={`${id}-credentials`} className="text-sm font-medium">
          Credentials Secret
        </label>
        <p id={`${id}-credentials-hint`} className="text-sm text-muted">
          The Secret holds the keys username and password. Leave it empty for a
          server that needs no sign-in.
        </p>
        <NativeSelect
          id={`${id}-credentials`}
          value={mail.credentialsSecret}
          aria-describedby={`${id}-credentials-hint`}
          onChange={(event) =>
            onChange("credentialsSecret", event.target.value)
          }
        >
          <NativeSelectOption value="">None</NativeSelectOption>
          {secrets.map((s) => (
            <NativeSelectOption key={s.name} value={s.name}>
              {s.name}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>
    </div>
  );
}

function Section({
  title,
  note,
  children,
}: {
  title: string;
  note?: string;
  children: ReactNode;
}) {
  return (
    <section className="flex min-w-0 flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h2 className="font-heading text-base font-semibold">{title}</h2>
        {note && <p className="text-sm text-muted">{note}</p>}
      </div>
      {children}
    </section>
  );
}

function TextField({
  id,
  label,
  value,
  type = "text",
  placeholder,
  mono = false,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  type?: "text" | "email" | "number";
  placeholder?: string;
  mono?: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <div className="flex max-w-md flex-col gap-1">
      <label htmlFor={id} className="text-sm font-medium text-text">
        {label}
      </label>
      <Input
        id={id}
        type={type}
        value={value}
        placeholder={placeholder}
        autoComplete="off"
        spellCheck={false}
        className={mono ? "font-mono" : undefined}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}

function Switch({
  id,
  label,
  checked,
  onChange,
}: {
  id: string;
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <div className="flex items-center gap-2">
      <Checkbox
        id={id}
        checked={checked}
        onCheckedChange={(state) => onChange(state === true)}
      />
      <label htmlFor={id} className="text-sm text-text">
        {label}
      </label>
    </div>
  );
}
