import "server-only";
import { readSecretValues } from "@/lib/secrets/values";
import { appNamespace } from "@/lib/settings/keys";
import { reasonOf } from "./errors";
import { appSession } from "./github-app";
import type { ApiError } from "./types";

/** GitAuthor is the author of a commit. */
export type GitAuthor = { name: string; email: string };

/** GIT_AUTHOR is the author of a commit pushed without GitHub App keys. */
export const GIT_AUTHOR: GitAuthor = {
  name: "fleet-manager",
  email: "fleet-manager@noreply.invalid",
};

/**
 * GitCredentials are the credentials of a push and the author of its
 * commit. auth is unset when no Secret is set. forget drops a cached
 * GitHub App token; call it when the Git server refuses auth.
 */
export type GitCredentials =
  | {
      ok: true;
      auth?: { username: string; password: string };
      author: GitAuthor;
      forget(): void;
    }
  | { ok: false; error: ApiError };

const TOKEN_KEYS = ["username", "token"];
const APP_KEYS = ["appId", "installationId", "privateKey"] as const;

const noop = () => {};

/**
 * gitCredentials returns the credentials of the Secret secret, in the
 * namespace of the app, for the repository url. Without a secret, there
 * is no auth and the author is GIT_AUTHOR.
 *
 * The Secret holds username and token, or the GitHub App keys appId,
 * installationId, and privateKey. A key with an empty value counts as
 * missing. A token gives GIT_AUTHOR as the author. App keys give an
 * installation token as the password of x-access-token, and the bot of
 * the App as the author, or GIT_AUTHOR when GitHub does not give the bot.
 *
 * It fails with Invalid on git.repository when url is not https://, before
 * it reads the Secret. It fails with Unavailable when the Secret cannot be
 * read or does not exist, holds keys of both sets, or lacks a key of its
 * set, and with the errors of appSession for App keys. No error holds a
 * credential.
 */
export async function gitCredentials(
  url: string,
  secret: string,
): Promise<GitCredentials> {
  if (!secret) return { ok: true, author: GIT_AUTHOR, forget: noop };
  if (URL.parse(url)?.protocol !== "https:") {
    return {
      ok: false,
      error: {
        code: "Invalid",
        path: "git.repository",
        message: `${url} must start with https:// to use the credentials of the Secret ${secret}.`,
      },
    };
  }
  const namespace = appNamespace();
  let values: Record<string, string> | undefined;
  try {
    values = await readSecretValues(namespace, secret);
  } catch (error) {
    return unavailable(
      `Could not read the Secret ${secret} in namespace ${namespace}: ${reasonOf(error)}`,
    );
  }
  if (!values) {
    return unavailable(
      `The Secret ${secret} does not exist in namespace ${namespace}.`,
    );
  }

  const has = (key: string) => Boolean(values[key]);
  if (APP_KEYS.some(has)) {
    if (TOKEN_KEYS.some(has)) {
      return unavailable(
        `The Secret ${secret} has keys for a token and keys for a GitHub App. Keep username and token, or appId, installationId, and privateKey.`,
      );
    }
    const missing = APP_KEYS.filter((key) => !has(key));
    if (missing.length > 0) {
      return unavailable(
        `The Secret ${secret} needs the keys appId, installationId, and privateKey for a GitHub App. It has no key ${listOf(missing)}.`,
      );
    }
    const app = await appSession({
      url,
      secret,
      keys: {
        appId: values.appId,
        installationId: values.installationId,
        privateKey: values.privateKey,
      },
      fallbackAuthor: GIT_AUTHOR,
    });
    if (!app.ok) return app;
    return {
      ok: true,
      auth: { username: "x-access-token", password: app.session.token },
      author: app.session.author,
      forget: app.forget,
    };
  }

  const missing = TOKEN_KEYS.find((key) => !has(key));
  if (missing)
    return unavailable(`The Secret ${secret} has no key ${missing}.`);
  return {
    ok: true,
    auth: { username: values.username, password: values.token },
    author: GIT_AUTHOR,
    forget: noop,
  };
}

function listOf(words: readonly string[]): string {
  return words.length < 2
    ? words.join("")
    : `${words.slice(0, -1).join(", ")} and ${words.at(-1)}`;
}

function unavailable(message: string): { ok: false; error: ApiError } {
  return { ok: false, error: { code: "Unavailable", message } };
}
