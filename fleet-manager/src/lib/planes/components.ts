// This module runs in the browser and on the server.
import type { SecretKeyRef } from "@/components/fleet/secret-ref-picker";
import type { PlaneDraft } from "./draft";
import type { Running } from "./versions";

/**
 * AdminDraft holds the first administrator of a plane in every mode. The
 * claim takes username, email, and password in the Keycloak modes, and
 * claimName and claimValue in OIDC mode.
 */
export type AdminDraft = {
  username: string;
  email: string;
  password?: SecretKeyRef;
  claimName: string;
  claimValue: string;
};

/** MailDraft is the SMTP server of Web Modeler. credentialsSecret is the name of a Secret, "" for none. */
export type MailDraft = {
  smtpHost: string;
  smtpPort: number;
  fromAddress: string;
  fromName: string;
  tls: boolean;
  credentialsSecret: string;
};

/** EMPTY_ADMIN is the admin of a new draft. */
export const EMPTY_ADMIN: AdminDraft = {
  username: "",
  email: "",
  claimName: "",
  claimValue: "",
};

/** EMPTY_MAIL is the mail of a new draft: port 587 with STARTTLS. */
export const EMPTY_MAIL: MailDraft = {
  smtpHost: "",
  smtpPort: 587,
  fromAddress: "",
  fromName: "",
  tls: true,
  credentialsSecret: "",
};

/** runningOf says which optional apps the plane of draft runs. Keycloak runs only in Keycloak mode. */
export function runningOf(draft: PlaneDraft): Running {
  return {
    keycloak: draft.mode === "keycloak",
    console: draft.console,
    webModeler: draft.webModeler,
  };
}

/** adminErrors returns the errors of the admin fields of the mode of draft, none without a mode. */
export function adminErrors(draft: PlaneDraft): string[] {
  const { admin } = draft;
  if (draft.mode === "oidc") {
    return [
      ...(admin.claimName === ""
        ? ["Enter the claim name of the first administrator."]
        : []),
      ...(admin.claimName.includes("=")
        ? ["The claim name cannot hold an equals sign."]
        : []),
      ...(admin.claimValue === ""
        ? ["Enter the claim value of the first administrator."]
        : []),
    ];
  }
  if (draft.mode === undefined) return [];
  return [
    ...(admin.username === ""
      ? ["Enter the username of the first administrator."]
      : []),
    ...(admin.email !== "" && admin.email.length < 3
      ? ["Enter a valid email address."]
      : []),
    ...(admin.email === "" && draft.webModeler
      ? ["Web Modeler needs the email of the first administrator."]
      : []),
  ];
}

// The claim names no keys of the SMTP Secret, so the operator reads these.
const SMTP_KEYS = ["username", "password"];

/** smtpSecrets returns the Secrets of secrets that hold the keys username and password. */
export function smtpSecrets<S extends { keys: readonly string[] }>(
  secrets: readonly S[],
): S[] {
  return secrets.filter((s) => SMTP_KEYS.every((key) => s.keys.includes(key)));
}

/**
 * mailSecretErrors returns an error when mail names a credentials Secret
 * that smtpSecrets of secrets does not hold.
 */
export function mailSecretErrors(
  mail: MailDraft,
  secrets: readonly { name: string; keys: readonly string[] }[],
): string[] {
  const name = mail.credentialsSecret;
  return name !== "" && !smtpSecrets(secrets).some((s) => s.name === name)
    ? [
        `Pick another SMTP credentials Secret. ${name} is gone, or it lacks the key username or password.`,
      ]
    : [];
}

/**
 * secretRefErrors returns an error when secrets do not hold ref, a Secret
 * and one of its keys. label names the field of ref.
 */
export function secretRefErrors(
  label: string,
  ref: SecretKeyRef,
  secrets: readonly { name: string; keys: readonly string[] }[],
): string[] {
  return secrets.some((s) => s.name === ref.name && s.keys.includes(ref.key))
    ? []
    : [
        `Pick another ${label}. ${ref.name} is gone, or it lacks the key ${ref.key}.`,
      ];
}

/** mailErrors returns the errors of the mail of Web Modeler. */
export function mailErrors(mail: MailDraft): string[] {
  const port = mail.smtpPort;
  return [
    ...(mail.smtpHost === "" ? ["Enter the SMTP host."] : []),
    ...(!Number.isInteger(port) || port < 1 || port > 65535
      ? ["Enter an SMTP port from 1 to 65535."]
      : []),
    ...(mail.fromAddress.length < 3
      ? ["Enter the address that Web Modeler sends from."]
      : []),
  ];
}
