"use client";

import {
  createContext,
  type ReactNode,
  useContext,
  useId,
  useRef,
  useState,
} from "react";
import {
  type SecretRefs,
  secretRefsOf,
} from "@/components/fleet/secret-ref-picker";
import { SecretNameSelect } from "@/components/setup/secret-name-select";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { useResources, useWatchesFresh } from "@/lib/k8s/hooks";

/** NamespacedSecretRef names one key of a Secret in any namespace. */
export type NamespacedSecretRef = {
  namespace: string;
  name: string;
  key: string;
};

/** SecretRefsReader is a hook: the Secret names and keys of one namespace, undefined until the list is known. */
export type SecretRefsReader = (namespace: string) => {
  secrets?: SecretRefs;
  error?: string;
};

function useLiveSecretRefs(namespace: string): {
  secrets?: SecretRefs;
  error?: string;
} {
  const { items, error } = useResources("Secret", { namespace }, []);
  const [fresh] = useWatchesFresh([{ kind: "Secret", opts: { namespace } }]);
  return {
    ...(fresh && { secrets: secretRefsOf(items) }),
    ...(error && { error }),
  };
}

/**
 * SecretRefsReaderContext holds the reader. Its default follows the live
 * Secrets of the namespace. A tree sets it once: the reader is a hook, so it
 * must not change between renders.
 */
export const SecretRefsReaderContext =
  createContext<SecretRefsReader>(useLiveSecretRefs);

/** staticSecretRefs is a reader over fixed lists, for stories and tests. A namespace that byNamespace lacks has no Secrets. */
export function staticSecretRefs(
  byNamespace: Readonly<Record<string, SecretRefs>>,
): SecretRefsReader {
  return (namespace) => ({
    secrets: Object.hasOwn(byNamespace, namespace)
      ? byNamespace[namespace]
      : [],
  });
}

/**
 * NamespacedSecretRefPicker picks a namespace, a Secret of it, and a key of
 * that Secret. It shows names only, never a Secret value. A value whose
 * namespace, Secret, or key does not exist stays picked, so a save keeps it,
 * and a missing Secret or key is marked. While the Secrets of the namespace
 * cannot be read, nothing is marked. onChange runs only on a pick of the user: it gets the
 * reference when all three are picked, and undefined when a pick clears a
 * reference that it reported before or that value gave.
 */
export function NamespacedSecretRefPicker({
  label,
  namespaces,
  value,
  invalid = false,
  onChange,
}: {
  label: string;
  namespaces: readonly string[];
  value?: NamespacedSecretRef;
  invalid?: boolean;
  onChange: (ref: NamespacedSecretRef | undefined) => void;
}): ReactNode {
  const id = useId();
  const [picked, setPicked] = useState<NamespacedSecretRef>({
    namespace: value?.namespace ?? "",
    name: value?.name ?? "",
    key: value?.key ?? "",
  });
  const reported = useRef(complete(picked));
  const pick = (next: NamespacedSecretRef) => {
    setPicked(next);
    const ref = complete(next);
    if (ref === undefined && reported.current === undefined) return;
    reported.current = ref;
    onChange(ref);
  };
  const options =
    namespaces.includes(picked.namespace) || picked.namespace === ""
      ? namespaces
      : [picked.namespace, ...namespaces];
  return (
    <div
      role="group"
      aria-label={label}
      className="flex min-w-0 flex-col gap-2 rounded-md border border-border p-3"
    >
      <p className="text-sm font-medium text-text">{label}</p>
      <div className="grid min-w-0 gap-3 sm:grid-cols-3">
        <div className="flex min-w-0 flex-col gap-1">
          <label
            htmlFor={`${id}-namespace`}
            className="text-sm font-medium text-text"
          >
            Namespace
          </label>
          <NativeSelect
            id={`${id}-namespace`}
            value={picked.namespace}
            aria-invalid={invalid && picked.namespace === "" ? true : undefined}
            onChange={(event) =>
              pick({ namespace: event.target.value, name: "", key: "" })
            }
          >
            <NativeSelectOption value="">Pick a namespace</NativeSelectOption>
            {options.map((namespace) => (
              <NativeSelectOption key={namespace} value={namespace}>
                {namespace}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
        {picked.namespace !== "" && (
          <SecretAndKey
            id={id}
            picked={picked}
            invalid={invalid}
            onPick={pick}
          />
        )}
      </div>
    </div>
  );
}

// SecretAndKey mounts only with a namespace, so the picker reads the Secrets
// of the picked namespace and of no other.
function SecretAndKey({
  id,
  picked,
  invalid,
  onPick,
}: {
  id: string;
  picked: NamespacedSecretRef;
  invalid: boolean;
  onPick: (next: NamespacedSecretRef) => void;
}) {
  const read = useContext(SecretRefsReaderContext);
  const { secrets: listed, error } = read(picked.namespace);
  const secrets = error ? undefined : listed;
  const keys = secrets?.find((s) => s.name === picked.name)?.keys;
  const keyMissing =
    keys !== undefined && picked.key !== "" && !keys.includes(picked.key);
  const keyUnlisted = picked.key !== "" && !(keys ?? []).includes(picked.key);
  return (
    <>
      <div className="min-w-0">
        <SecretNameSelect
          label="Secret"
          value={picked.name}
          names={secrets?.map((s) => s.name)}
          namespace={picked.namespace}
          invalid={invalid && picked.name === ""}
          onChange={(name) => onPick({ ...picked, name, key: "" })}
        />
      </div>
      <div className="flex min-w-0 flex-col gap-1">
        <label htmlFor={`${id}-key`} className="text-sm font-medium text-text">
          Key
        </label>
        <NativeSelect
          id={`${id}-key`}
          value={picked.key}
          disabled={picked.name === ""}
          aria-invalid={
            keyMissing || (invalid && picked.key === "") || undefined
          }
          onChange={(event) => onPick({ ...picked, key: event.target.value })}
        >
          <NativeSelectOption value="">Pick a key</NativeSelectOption>
          {keyUnlisted && (
            <NativeSelectOption value={picked.key}>
              {keyMissing ? `${picked.key} (not found)` : picked.key}
            </NativeSelectOption>
          )}
          {(keys ?? []).map((key) => (
            <NativeSelectOption key={key} value={key}>
              {key}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>
      {error && (
        <p role="alert" className="text-sm text-bad sm:col-span-3">
          Could not read the Secrets of {picked.namespace}: {error}
        </p>
      )}
    </>
  );
}

function complete(ref: NamespacedSecretRef): NamespacedSecretRef | undefined {
  return ref.namespace && ref.name && ref.key ? { ...ref } : undefined;
}
