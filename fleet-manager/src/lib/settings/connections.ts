import "server-only";
import { listServerRefs } from "isomorphic-git";
import gitHttp from "isomorphic-git/http/node";
import { readSecretValues } from "@/lib/secrets/values";
import { appNamespace } from "./keys";
import { getSettings } from "./store";

/** TestResult is the outcome of a connection test, with a message for the page. */
export type TestResult = { ok: boolean; message: string };

const TIMEOUT_MS = 10_000;

type Credentials =
  | { ok: true; values: Record<string, string> | undefined }
  | { ok: false; result: TestResult };

/**
 * testGit lists the branches of git.repository over HTTPS, with the
 * username and token of the Secret of git.credentialsSecretRef when it is
 * set. It fails when git.branch is set and the repository does not have
 * it. The message never holds a credential or a response body.
 */
export async function testGit(): Promise<TestResult> {
  const settings = await getSettings();
  const url = settings["git.repository"];
  if (!url) return { ok: false, message: "No Git repository is set." };
  const credentials = await readCredentials(
    settings["git.credentialsSecretRef"],
  );
  if (!credentials.ok) return credentials.result;
  const values = credentials.values;
  if (values) {
    for (const key of ["username", "token"]) {
      if (!values[key]) {
        return {
          ok: false,
          message: `The Secret ${settings["git.credentialsSecretRef"]} has no key ${key}.`,
        };
      }
    }
  }
  try {
    // The deadline also closes the request, so a server that never ends
    // its answer keeps no socket open.
    const signal = AbortSignal.timeout(TIMEOUT_MS);
    const refs = await listServerRefs({
      http: { request: (request) => gitHttp.request({ ...request, signal }) },
      url,
      prefix: "refs/heads/",
      protocolVersion: 1,
      onAuth: () =>
        values ? { username: values.username, password: values.token } : {},
      // Without cancel, a refused token makes isomorphic-git ask again.
      onAuthFailure: () => ({ cancel: true }),
    });
    const branch = settings["git.branch"];
    if (branch && !refs.some((r) => r.ref === `refs/heads/${branch}`)) {
      return { ok: false, message: `${url} has no branch ${branch}.` };
    }
    return { ok: true, message: `Connected to ${url}.` };
  } catch (error) {
    const status = (error as { data?: { statusCode?: number } }).data
      ?.statusCode;
    const code = (error as { code?: string }).code;
    if (code === "UserCanceledError" || status === 401 || status === 403) {
      return { ok: false, message: `${url} refused the credentials.` };
    }
    // An HttpError message holds only the status. Its response stays out.
    return {
      ok: false,
      message: `Could not reach ${url}: ${redact(messageOf(error), values)}`,
    };
  }
}

/**
 * testPrometheus sends the query vector(1) to prometheus.url, with basic
 * auth from username and password, or a bearer token from token, of the
 * Secret of prometheus.credentialsSecretRef when it is set. The message
 * never holds a credential or a response body.
 */
export async function testPrometheus(): Promise<TestResult> {
  const settings = await getSettings();
  const url = settings["prometheus.url"];
  if (!url) return { ok: false, message: "No Prometheus address is set." };
  const secret = settings["prometheus.credentialsSecretRef"];
  const credentials = await readCredentials(secret);
  if (!credentials.ok) return credentials.result;
  const values = credentials.values;
  const headers: Record<string, string> = { Accept: "application/json" };
  if (values) {
    if (values.username && values.password) {
      headers.Authorization =
        "Basic " +
        Buffer.from(`${values.username}:${values.password}`).toString("base64");
    } else if (values.token) {
      headers.Authorization = `Bearer ${values.token}`;
    } else {
      return {
        ok: false,
        message: `The Secret ${secret} needs the keys username and password, or token.`,
      };
    }
  }
  const at = `Prometheus at ${url}`;
  try {
    const response = await fetch(
      `${url.replace(/\/+$/, "")}/api/v1/query?query=vector(1)`,
      { headers, signal: AbortSignal.timeout(TIMEOUT_MS) },
    );
    if (response.status === 401 || response.status === 403) {
      return { ok: false, message: `${at} refused the credentials.` };
    }
    if (response.status !== 200) {
      return { ok: false, message: `${at} answered HTTP ${response.status}.` };
    }
    const body = (await response.json().catch(() => undefined)) as
      { status?: unknown } | undefined;
    if (body?.status !== "success") {
      return { ok: false, message: `${at} did not answer with a result.` };
    }
    return { ok: true, message: `${at} answered.` };
  } catch (error) {
    return {
      ok: false,
      message: `Could not reach ${at}: ${redact(messageOf(error), values)}`,
    };
  }
}

async function readCredentials(secret: string): Promise<Credentials> {
  if (!secret) return { ok: true, values: undefined };
  const namespace = appNamespace();
  const values = await readSecretValues(namespace, secret);
  if (!values) {
    return {
      ok: false,
      result: {
        ok: false,
        message: `The Secret ${secret} does not exist in namespace ${namespace}.`,
      },
    };
  }
  return { ok: true, values };
}

// A library message can quote a header, so each token and password and its
// basic auth form are removed before a message leaves this module.
function redact(
  message: string,
  values: Record<string, string> | undefined,
): string {
  const secrets: string[] = [];
  for (const v of [values?.token, values?.password]) {
    if (!v) continue;
    secrets.push(v);
    if (values?.username) {
      secrets.push(Buffer.from(`${values.username}:${v}`).toString("base64"));
    }
  }
  return secrets
    .sort((a, b) => b.length - a.length)
    .reduce((text, secret) => text.split(secret).join("***"), message);
}

function messageOf(error: unknown): string {
  if (!(error instanceof Error)) return String(error);
  // fetch rejects with "fetch failed" and puts the reason in cause.
  const cause = error.cause;
  return cause instanceof Error ? cause.message : error.message;
}
