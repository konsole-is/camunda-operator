import "server-only";
import { createPrivateKey, createSign } from "node:crypto";
import { reasonOf } from "./errors";
import type { ApiError } from "./types";

/** AppKeys are the GitHub App keys of the credentials Secret, already checked for presence. */
export type AppKeys = {
  appId: string;
  installationId: string;
  privateKey: string;
};

/** AppSession is an installation token and the author of its commits. */
export type AppSession = {
  token: string;
  author: { name: string; email: string };
};

type Fill =
  | { ok: true; session: AppSession; expiresAt: number }
  | { ok: false; error: ApiError };

type Entry = { promise: Promise<Fill> };

const RENEW_BEFORE_MS = 5 * 60_000;
const META_TIMEOUT_MS = 10_000;
const REQUEST_TIMEOUT_MS = 30_000;
const NUMBER = /^[1-9][0-9]*$/;

const SESSIONS_KEY: unique symbol = Symbol.for("fleet-manager.github-app");

// Next.js can load this module once for each route, so the cache lives on
// globalThis and every route shares it.
const sessions = ((
  globalThis as typeof globalThis & { [SESSIONS_KEY]?: Map<string, Entry> }
)[SESSIONS_KEY] ??= new Map());

/**
 * appJWT returns the JWT that authenticates the GitHub App appId, signed
 * with privateKey (PEM). It is valid from 60 s before nowSeconds, for
 * GitHub's clock drift, until at most 10 minutes later.
 */
export function appJWT(
  appId: string,
  privateKey: string,
  nowSeconds: number,
): string {
  const part = (value: object) =>
    Buffer.from(JSON.stringify(value)).toString("base64url");
  const iat = nowSeconds - 60;
  const unsigned = `${part({ alg: "RS256", typ: "JWT" })}.${part({ iat, exp: iat + 600, iss: appId })}`;
  const signature = createSign("RSA-SHA256")
    .update(unsigned)
    .sign(privateKey, "base64url");
  return `${unsigned}.${signature}`;
}

/**
 * appSession returns an installation token of the App keys for the
 * repository url, which must start with https://, and the author of its
 * commits. The author is the bot of the App, or fallbackAuthor when the
 * bot cannot be looked up.
 *
 * App keys work only for github.com and for a host whose /api/v3/meta
 * answers as GitHub Enterprise Server. For any other host, or for an appId
 * or installationId that is not a number, or a privateKey that is not a
 * PEM RSA private key, it fails with Unavailable and sends no JWT. A token
 * request that GitHub refuses gives Forbidden. No error holds a key, the
 * JWT, the token, or the text of a GitHub answer.
 *
 * The session is cached in this server process for each host, App, and
 * installation, and renewed 5 minutes before the token expires. Concurrent
 * calls share one token request, and a failed request is not cached.
 * forget drops the cached session, for a token that the Git server
 * refused.
 */
export async function appSession(input: {
  url: string;
  secret: string;
  keys: AppKeys;
  fallbackAuthor: { name: string; email: string };
}): Promise<
  | { ok: true; session: AppSession; forget(): void }
  | { ok: false; error: ApiError }
> {
  const { url, secret, keys } = input;
  for (const key of ["appId", "installationId"] as const) {
    if (!NUMBER.test(keys[key])) {
      return unavailable(
        `The Secret ${secret} has a key ${key} that is not a number.`,
      );
    }
  }
  let rsa = false;
  try {
    rsa = createPrivateKey(keys.privateKey).asymmetricKeyType === "rsa";
  } catch {}
  if (!rsa) {
    return unavailable(
      `The Secret ${secret} has a key privateKey that is not a PEM private key.`,
    );
  }

  const cacheKey = `${new URL(url).host} ${keys.appId} ${keys.installationId}`;
  for (;;) {
    let entry = sessions.get(cacheKey);
    const fresh = entry === undefined;
    if (entry === undefined) {
      const started: Entry = {
        promise: fill(input).catch((error: unknown) =>
          unavailable(`Could not get a token from GitHub: ${reasonOf(error)}`),
        ),
      };
      entry = started;
      sessions.set(cacheKey, started);
      void started.promise.then((result) => {
        if (!result.ok && sessions.get(cacheKey) === started) {
          sessions.delete(cacheKey);
        }
      });
    }
    const held = entry;
    const result = await held.promise;
    if (!result.ok) return result;
    // A fresh token that expires within the renewal window is still used,
    // so a short-lived token cannot make this loop ask again and again.
    if (fresh || result.expiresAt - Date.now() > RENEW_BEFORE_MS) {
      return {
        ok: true,
        session: result.session,
        forget: () => {
          if (sessions.get(cacheKey) === held) sessions.delete(cacheKey);
        },
      };
    }
    if (sessions.get(cacheKey) === held) sessions.delete(cacheKey);
  }
}

async function fill(input: {
  url: string;
  secret: string;
  keys: AppKeys;
  fallbackAuthor: { name: string; email: string };
}): Promise<Fill> {
  const { url, secret, keys, fallbackAuthor } = input;
  const api = await apiOf(url, secret);
  if (!api.ok) return api;

  const jwt = appJWT(
    keys.appId,
    keys.privateKey,
    Math.floor(Date.now() / 1000),
  );
  const subject = `the token request of the App ${keys.appId} for the installation ${keys.installationId}`;
  let response: Response;
  try {
    response = await githubFetch(
      `${api.base}/app/installations/${keys.installationId}/access_tokens`,
      { method: "POST", headers: headers(jwt) },
      REQUEST_TIMEOUT_MS,
    );
  } catch (error) {
    return unavailable(`Could not reach ${api.base}: ${reasonOf(error)}`);
  }
  if ([401, 403, 404, 422].includes(response.status)) {
    return {
      ok: false,
      error: {
        code: "Forbidden",
        message: `GitHub refused ${subject} (HTTP ${response.status}).`,
      },
    };
  }
  if (response.status !== 201) {
    return unavailable(
      `GitHub answered ${subject} with HTTP ${response.status}.`,
    );
  }
  const body = (await response.json().catch(() => undefined)) as
    { token?: unknown; expires_at?: unknown } | undefined;
  const expiresAt =
    typeof body?.expires_at === "string" ? Date.parse(body.expires_at) : NaN;
  if (
    typeof body?.token !== "string" ||
    !body.token ||
    Number.isNaN(expiresAt)
  ) {
    return unavailable(
      `GitHub did not answer the token request of the App ${keys.appId} with a token.`,
    );
  }
  const token = body.token;
  const author = (await botOf(api, jwt, token)) ?? fallbackAuthor;
  return { ok: true, session: { token, author }, expiresAt };
}

type Api = { base: string; noreplyHost: string };

// apiOf returns the GitHub API of the host of url. Only a host that answers
// as GitHub Enterprise Server gets the JWT, so meta is asked without it.
async function apiOf(
  url: string,
  secret: string,
): Promise<({ ok: true } & Api) | { ok: false; error: ApiError }> {
  const { host, hostname, port } = new URL(url);
  if (hostname === "github.com" && port === "") {
    return {
      ok: true,
      base: "https://api.github.com",
      noreplyHost: "github.com",
    };
  }
  const base = `https://${host}/api/v3`;
  let response: Response;
  try {
    response = await githubFetch(
      `${base}/meta`,
      { headers: { Accept: "application/json" } },
      META_TIMEOUT_MS,
    );
  } catch (error) {
    return unavailable(`Could not reach ${base}: ${reasonOf(error)}`);
  }
  const meta =
    response.status === 200
      ? ((await response.json().catch(() => undefined)) as
          { installed_version?: unknown } | undefined)
      : undefined;
  if (typeof meta?.installed_version !== "string") {
    return unavailable(
      `${url} is not on GitHub or GitHub Enterprise Server. The keys appId, installationId, and privateKey of the Secret ${secret} work only there.`,
    );
  }
  return { ok: true, base, noreplyHost: hostname };
}

// botOf returns the bot user of the App, or undefined when GitHub does not
// give it.
async function botOf(
  api: Api,
  jwt: string,
  token: string,
): Promise<AppSession["author"] | undefined> {
  try {
    const app = await githubFetch(
      `${api.base}/app`,
      { headers: headers(jwt) },
      REQUEST_TIMEOUT_MS,
    );
    if (app.status !== 200) return undefined;
    const { slug } = (await app.json()) as { slug?: unknown };
    if (typeof slug !== "string" || !slug) return undefined;
    const name = `${slug}[bot]`;
    const user = await githubFetch(
      `${api.base}/users/${encodeURIComponent(name)}`,
      { headers: headers(token) },
      REQUEST_TIMEOUT_MS,
    );
    if (user.status !== 200) return undefined;
    const { id } = (await user.json()) as { id?: unknown };
    if (typeof id !== "number") return undefined;
    return { name, email: `${id}+${name}@users.noreply.${api.noreplyHost}` };
  } catch {
    return undefined;
  }
}

const headers = (bearer: string) => ({
  Accept: "application/vnd.github+json",
  Authorization: `Bearer ${bearer}`,
  "X-GitHub-Api-Version": "2022-11-28",
});

// A redirect would carry the Authorization header to another address, so
// no request follows one, and a 3xx answer is a failure.
function githubFetch(
  url: string,
  init: RequestInit,
  timeoutMs: number,
): Promise<Response> {
  return fetch(url, {
    ...init,
    redirect: "manual",
    signal: AbortSignal.timeout(timeoutMs),
  });
}

function unavailable(message: string): { ok: false; error: ApiError } {
  return { ok: false, error: { code: "Unavailable", message } };
}
