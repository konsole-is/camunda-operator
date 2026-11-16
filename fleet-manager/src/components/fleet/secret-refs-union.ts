import { byCodeUnit } from "@/lib/policy";
import type { SecretRefs } from "./secret-ref-picker";

/**
 * secretRefsUnion returns each Secret name that a namespace of byNamespace
 * holds, with the union of its key names, sorted. missingIn returns the
 * namespaces of byNamespace, sorted, whose Secret name is missing or lacks
 * key. With no namespace, refs is empty and missingIn returns none.
 */
export function secretRefsUnion(
  byNamespace: Readonly<Record<string, SecretRefs>>,
): { refs: SecretRefs; missingIn(name: string, key: string): string[] } {
  const namespaces = Object.keys(byNamespace).sort(byCodeUnit);
  return {
    refs: namespaces.reduce<SecretRefs>(
      (refs, namespace) => withSecretRefs(refs, byNamespace[namespace]),
      [],
    ),
    missingIn: (name, key) =>
      namespaces.filter(
        (namespace) =>
          !byNamespace[namespace].some(
            (secret) => secret.name === name && secret.keys.includes(key),
          ),
      ),
  };
}

/** withSecretRefs returns refs with each name and key of more added, sorted. */
export function withSecretRefs(refs: SecretRefs, more: SecretRefs): SecretRefs {
  const keys = new Map<string, Set<string>>();
  for (const secret of [...refs, ...more]) {
    const set = keys.get(secret.name) ?? new Set<string>();
    for (const key of secret.keys) set.add(key);
    keys.set(secret.name, set);
  }
  return [...keys.keys()].sort(byCodeUnit).map((name) => ({
    name,
    keys: [...(keys.get(name) ?? [])].sort(byCodeUnit),
  }));
}
