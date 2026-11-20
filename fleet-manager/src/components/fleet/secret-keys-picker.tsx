"use client";

import { type ReactNode, useContext, useId, useState } from "react";
import { SecretRefsReaderContext } from "@/components/fleet/namespaced-secret-ref-picker";
import { SecretNameSelect } from "@/components/setup/secret-name-select";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";

/** SecretKeySlot is one key that a reference reads from its Secret: the field that holds the key name, its label, and the key that the operator reads when the field is unset. */
export type SecretKeySlot = { field: string; label: string; fallback?: string };

/** PickedKeys is a Secret name and the key name of each picked slot, by the field of the slot. */
export type PickedKeys = { name: string } & Record<string, string>;

/** CREDENTIALS_SLOTS are the keys of a username and password Secret, with the defaults of LocalCredentialsSecretRef. */
export const CREDENTIALS_SLOTS: readonly SecretKeySlot[] = [
  { field: "usernameKey", label: "Username key", fallback: "username" },
  { field: "passwordKey", label: "Password key", fallback: "password" },
];

/** S3_CREDENTIALS_SLOTS are the keys of an S3 access key pair. */
export const S3_CREDENTIALS_SLOTS: readonly SecretKeySlot[] = [
  { field: "accessKeyIdKey", label: "Access key ID key" },
  { field: "secretAccessKeyKey", label: "Secret access key key" },
];

/** KEY_SLOT is the one key of a Secret key reference. */
export const KEY_SLOT: readonly SecretKeySlot[] = [
  { field: "key", label: "Key" },
];

/**
 * SecretKeysPicker picks a Secret of namespace and one of its keys for each
 * slot. It reads the names and keys through SecretRefsReaderContext and
 * never shows a Secret value. A slot with a fallback may stay empty: its
 * field is left out, and the operator reads the fallback. A value whose
 * Secret or key does not exist stays picked, so a save keeps it, and is
 * marked as not found; while the Secrets cannot be read, nothing is marked.
 * onChange runs only on a pick of the user: it gets the Secret name and the
 * key of each picked slot, also while a slot without a fallback is not
 * picked, and undefined when the Secret is cleared.
 */
export function SecretKeysPicker({
  label,
  namespace,
  slots,
  value,
  invalid = false,
  onChange,
}: {
  label: string;
  namespace: string;
  slots: readonly SecretKeySlot[];
  value?: PickedKeys;
  invalid?: boolean;
  onChange: (ref: PickedKeys | undefined) => void;
}): ReactNode {
  const id = useId();
  const [picked, setPicked] = useState<PickedKeys>(() => ({
    name: "",
    ...value,
  }));
  const read = useContext(SecretRefsReaderContext);
  const { secrets: listed, error } = read(namespace);
  const secrets = error ? undefined : listed;
  const keys = secrets?.find((s) => s.name === picked.name)?.keys;
  const pick = (next: PickedKeys) => {
    setPicked(next);
    onChange(next.name === "" ? undefined : next);
  };
  return (
    <div
      role="group"
      aria-label={label}
      className="flex min-w-0 flex-col gap-2 rounded-md border border-border p-3"
    >
      <p className="text-sm font-medium text-text">{label}</p>
      {secrets?.length === 0 && picked.name === "" ? (
        <p className="text-sm text-muted">
          The namespace {namespace} has no Secret.
        </p>
      ) : (
        <div className="grid min-w-0 gap-3 sm:grid-cols-3">
          <div className="min-w-0">
            <SecretNameSelect
              label="Secret"
              value={picked.name}
              names={secrets?.map((s) => s.name)}
              namespace={namespace}
              invalid={invalid && picked.name === ""}
              onChange={(name) => pick({ name } as PickedKeys)}
            />
          </div>
          {slots.map((slot) => {
            const current = picked[slot.field] ?? "";
            const missing =
              keys !== undefined && current !== "" && !keys.includes(current);
            const unlisted = current !== "" && !(keys ?? []).includes(current);
            return (
              <div key={slot.field} className="flex min-w-0 flex-col gap-1">
                <label
                  htmlFor={`${id}-${slot.field}`}
                  className="text-sm font-medium text-text"
                >
                  {slot.label}
                </label>
                <NativeSelect
                  id={`${id}-${slot.field}`}
                  value={current}
                  disabled={picked.name === ""}
                  aria-invalid={
                    missing ||
                    (invalid && current === "" && !slot.fallback) ||
                    undefined
                  }
                  onChange={(event) =>
                    pick(withKey(picked, slot.field, event.target.value))
                  }
                >
                  <NativeSelectOption value="">
                    {slot.fallback
                      ? `Default (${slot.fallback})`
                      : "Pick a key"}
                  </NativeSelectOption>
                  {unlisted && (
                    <NativeSelectOption value={current}>
                      {missing ? `${current} (not found)` : current}
                    </NativeSelectOption>
                  )}
                  {(keys ?? []).map((key) => (
                    <NativeSelectOption key={key} value={key}>
                      {key}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              </div>
            );
          })}
        </div>
      )}
      {error && (
        <p role="alert" className="text-sm text-bad">
          Could not read the Secrets of {namespace}: {error}
        </p>
      )}
    </div>
  );
}

function withKey(picked: PickedKeys, field: string, key: string): PickedKeys {
  const next = { ...picked };
  if (key === "") delete next[field];
  else next[field] = key;
  return next;
}
