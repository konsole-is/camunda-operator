import "server-only";
import type { ApiError } from "@/lib/write/types";

/** SETTINGS_CONFIGMAP is the ConfigMap in FLEET_NAMESPACE that holds the settings. */
export const SETTINGS_CONFIGMAP = "fleet-manager-settings";

/** SETTING_KEYS are the keys of the settings, as the ConfigMap stores them. */
export const SETTING_KEYS = [
  "kubernetesClusterName",
  "git.repository",
  "git.branch",
  "git.path",
  "git.credentialsSecretRef",
  "secretStore.kind",
  "secretStore.name",
  "prometheus.url",
  "prometheus.credentialsSecretRef",
] as const;

export type SettingKey = (typeof SETTING_KEYS)[number];

/** Settings are the settings of the fleet manager. An empty string means not set. */
export type Settings = Record<SettingKey, string>;

/** SEED_VARIABLES maps each setting to the variable that the chart sets for it. */
export const SEED_VARIABLES: Record<SettingKey, string> = {
  kubernetesClusterName: "FLEET_SETTINGS_KUBERNETES_CLUSTER_NAME",
  "git.repository": "FLEET_SETTINGS_GIT_REPOSITORY",
  "git.branch": "FLEET_SETTINGS_GIT_BRANCH",
  "git.path": "FLEET_SETTINGS_GIT_PATH",
  "git.credentialsSecretRef": "FLEET_SETTINGS_GIT_CREDENTIALS_SECRET_REF",
  "secretStore.kind": "FLEET_SETTINGS_SECRET_STORE_KIND",
  "secretStore.name": "FLEET_SETTINGS_SECRET_STORE_NAME",
  "prometheus.url": "FLEET_SETTINGS_PROMETHEUS_URL",
  "prometheus.credentialsSecretRef":
    "FLEET_SETTINGS_PROMETHEUS_CREDENTIALS_SECRET_REF",
};

const STORE_KINDS = ["", "SecretStore", "ClusterSecretStore"];
const URL_KEYS: Partial<Record<SettingKey, string>> = {
  "git.repository": "git.credentialsSecretRef",
  "prometheus.url": "prometheus.credentialsSecretRef",
};

/** seedFromEnv returns the settings that the seed variables of env give, without the credentials of an address. An address that is not http or https is empty. */
export function seedFromEnv(env: Record<string, string | undefined>): Settings {
  return fill((key) => env[SEED_VARIABLES[key]]);
}

/** settingsFromData returns the settings of the data of the ConfigMap, without the credentials of an address. An address that is not http or https is empty. Unknown keys are left out. */
export function settingsFromData(
  data: Record<string, string> | undefined,
): Settings {
  return fill((key) => data?.[key]);
}

function fill(value: (key: SettingKey) => string | undefined): Settings {
  return Object.fromEntries(
    SETTING_KEYS.map((key) => {
      const v = value(key) ?? "";
      return [key, URL_KEYS[key] ? withoutCredentials(v) : v];
    }),
  ) as Settings;
}

// The chart values and a hand edit of the ConfigMap skip checkPatch, and
// the pages show the settings. An address that is not http or https can
// hide credentials from URL.
function withoutCredentials(value: string): string {
  const url = httpURL(value);
  if (!url) return "";
  if (!url.username && !url.password) return value;
  url.username = "";
  url.password = "";
  return url.href;
}

function httpURL(value: string): URL | undefined {
  const url = URL.parse(value);
  return url && (url.protocol === "http:" || url.protocol === "https:")
    ? url
    : undefined;
}

function isSettingKey(key: string): key is SettingKey {
  return (SETTING_KEYS as readonly string[]).includes(key);
}

/**
 * checkPatch returns one Invalid error for each entry of patch that
 * updateSettings must refuse, or none. A message never holds the refused
 * value, because a URL can hold a token.
 */
export function checkPatch(patch: Record<string, unknown>): ApiError[] {
  const errors: ApiError[] = [];
  const invalid = (path: string, message: string) =>
    errors.push({ code: "Invalid", path, message });
  for (const [key, value] of Object.entries(patch)) {
    if (!isSettingKey(key)) {
      invalid(key, `${key} is not a setting of the fleet manager`);
    } else if (typeof value !== "string") {
      invalid(key, `${key} must be a string`);
    } else if (key === "secretStore.kind" && !STORE_KINDS.includes(value)) {
      invalid(
        key,
        "secretStore.kind must be SecretStore or ClusterSecretStore",
      );
    } else if (URL_KEYS[key] && value !== "") {
      const url = httpURL(value);
      if (!url) {
        invalid(key, `${key} must be an http:// or https:// address`);
      } else if (url.username || url.password) {
        invalid(
          key,
          `${key} must not hold credentials. Put them in the Secret of ${URL_KEYS[key]}`,
        );
      }
    }
  }
  return errors;
}

/** appNamespace returns FLEET_NAMESPACE. It throws when the variable is empty. */
export function appNamespace(): string {
  const namespace = process.env.FLEET_NAMESPACE;
  if (!namespace) {
    throw new Error(
      "FLEET_NAMESPACE is not set. The chart sets it to the namespace of the pod",
    );
  }
  return namespace;
}
