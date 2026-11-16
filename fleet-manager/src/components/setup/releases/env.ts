import type { EnvVar } from "@/components/fleet/env-editor";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import { withSecretRefs } from "@/components/fleet/secret-refs-union";
import type { CamundaReleaseSpec } from "@/generated/crds/camundarelease";

/** EnvScope is a place of a spec that holds extraEnv and extraEnvFrom: the top level or one component block. */
export type EnvScope =
  | "release"
  | "connectors"
  | "zeebe"
  | "gateway"
  | "operate"
  | "tasklist"
  | "admin";

/** ENV_SCOPES lists each scope with its label, in the order of the page. */
export const ENV_SCOPES: readonly { scope: EnvScope; label: string }[] = [
  { scope: "release", label: "Every workload" },
  { scope: "connectors", label: "Connectors" },
  { scope: "zeebe", label: "Brokers" },
  { scope: "gateway", label: "Gateway" },
  { scope: "operate", label: "Operate" },
  { scope: "tasklist", label: "Tasklist" },
  { scope: "admin", label: "Admin" },
];

/** EnvFromSource is one entry of extraEnvFrom. */
export type EnvFromSource = NonNullable<
  CamundaReleaseSpec["extraEnvFrom"]
>[number];

/** ScopeEnv is the extraEnv and extraEnvFrom of one scope. */
export type ScopeEnv = { extraEnv?: EnvVar[]; extraEnvFrom?: EnvFromSource[] };

/**
 * EnvSpec is a spec with the env lists of each scope of ENV_SCOPES: a
 * release, or the body of a cluster preset.
 */
export type EnvSpec = ScopeEnv & {
  [S in Exclude<EnvScope, "release">]?: ScopeEnv;
};

/** envOf returns the env lists of scope in spec, without the lists that are unset. */
export function envOf(spec: EnvSpec, scope: EnvScope): ScopeEnv {
  const block: ScopeEnv | undefined = scope === "release" ? spec : spec[scope];
  return {
    ...(block?.extraEnv && { extraEnv: block.extraEnv }),
    ...(block?.extraEnvFrom && { extraEnvFrom: block.extraEnvFrom }),
  };
}

/**
 * withEnv returns spec with the env lists of scope replaced by env. An
 * empty list is left out. A component block that holds nothing else is
 * removed.
 */
export function withEnv<S extends EnvSpec>(
  spec: S,
  scope: EnvScope,
  env: ScopeEnv,
): S {
  const lists = {
    ...(env.extraEnv?.length ? { extraEnv: env.extraEnv } : {}),
    ...(env.extraEnvFrom?.length ? { extraEnvFrom: env.extraEnvFrom } : {}),
  };
  if (scope === "release") return { ...withoutEnv(spec), ...lists };
  const rest: EnvSpec = { ...spec };
  delete rest[scope];
  const block = { ...withoutEnv(spec[scope] ?? {}), ...lists };
  return (
    Object.keys(block).length > 0 ? { ...rest, [scope]: block } : rest
  ) as S;
}

function withoutEnv<T extends ScopeEnv>(block: T): T {
  const copy = { ...block };
  delete copy.extraEnv;
  delete copy.extraEnvFrom;
  return copy;
}

/** namedSecretRefs returns each Secret key that an extraEnv entry of spec names, in any scope. */
export function namedSecretRefs(spec: EnvSpec): SecretRefs {
  const refs = ENV_SCOPES.flatMap(({ scope }) =>
    (envOf(spec, scope).extraEnv ?? []).flatMap((entry) => {
      const ref = entry.valueFrom?.secretKeyRef;
      return ref?.name ? [{ name: ref.name, keys: [ref.key] }] : [];
    }),
  );
  return withSecretRefs([], refs);
}
