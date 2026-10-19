import "server-only";
import { listServerRefs } from "isomorphic-git";
import gitHttp from "isomorphic-git/http/node";
import { KubernetesRequestError } from "@/lib/k8s/client";
import { readSecretValues } from "@/lib/secrets/values";
import { type Settings, appNamespace } from "./keys";
import { getSettings } from "./store";

/** TestResult is the outcome of a connection test, with a message for the page. */
export type TestResult = { ok: boolean; message: string };

const TIMEOUT_MS = 10_000;

type Credentials =
  | { ok: true; values: Record<string, string> | undefined }
  | { ok: false; result: TestResult };

/**
 * testGit lists the branches of git.repository, with the username and
 * token of the Secret of git.credentialsSecretRef when it is set. It fails
 * when git.branch is set and the repository does not have it, and it
 * refuses to send credentials to an address that is not https://. The
 * message never holds a credential or a response body.
 */
export async function testGit(): Promise<TestResult> {
  let settings: Settings;
  try {
    settings = await getSettings();
  } catch (error) {
    return settingsFailure(error);
  }
  const url = settings["git.repository"];
  if (!url) return { ok: false, message: "No Git repository is set." };
  const secret = settings["git.credentialsSecretRef"];
  if (secret && URL.parse(url)?.protocol !== "https:") {
    return {
      ok: false,
      message: `${url} must start with https:// to use the credentials of the Secret ${secret}.`,
    };
  }
  const credentials = await readCredentials(secret);
  if (!credentials.ok) return credentials.result;
  const values = credentials.values;
  if (values) {
    for (const key of ["username", "token"]) {
      if (!values[key]) {
        return {
          ok: false,
          message: `The Secret ${secret} has no key ${key}.`,
        };
      }
    }
  }
  try {
    // The signal also closes a request that never ends. The transport keeps
    // the credentials on a redirect, also to http://, so it follows none.
    const signal = AbortSignal.timeout(TIMEOUT_MS);
    const refs = await listServerRefs({
      http: {
        request: (request) =>
          gitHttp.request({
            ...request,
            signal,
            fetchOptions: { maxRedirects: 0 },
          }),
      },
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
    return { ok: false, message: `Could not reach ${url}: ${reasonOf(error)}` };
  }
}

/**
 * testPrometheus sends the query vector(1) to prometheus.url, with basic
 * auth from username and password, or a bearer token from token, of the
 * Secret of prometheus.credentialsSecretRef when it is set. The message
 * never holds a credential or a response body.
 */
export async function testPrometheus(): Promise<TestResult> {
  let settings: Settings;
  try {
    settings = await getSettings();
  } catch (error) {
    return settingsFailure(error);
  }
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
      {
        headers,
        signal: AbortSignal.timeout(TIMEOUT_MS),
        redirect: "manual",
      },
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
      message: `Could not reach ${at}: ${reasonOf(error)}`,
    };
  }
}

function settingsFailure(error: unknown): TestResult {
  return {
    ok: false,
    message: `Could not read the settings: ${reasonOf(error)}`,
  };
}

async function readCredentials(secret: string): Promise<Credentials> {
  if (!secret) return { ok: true, values: undefined };
  const namespace = appNamespace();
  let values;
  try {
    values = await readSecretValues(namespace, secret);
  } catch (error) {
    const reason = reasonOf(error);
    return {
      ok: false,
      result: {
        ok: false,
        message: `Could not read the Secret ${secret} in namespace ${namespace}: ${reason}`,
      },
    };
  }
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

// A library message can quote the answer of the server, and the answer can
// hold the credentials. So a message names only an error code, an HTTP
// status, or a timeout, never the text of the error.
function reasonOf(error: unknown): string {
  if (error instanceof KubernetesRequestError) {
    return `the API server answered HTTP ${error.statusCode}`;
  }
  const cause = error instanceof Error && error.cause ? error.cause : error;
  const name = (cause as { name?: unknown }).name;
  const message = (cause as { message?: unknown }).message;
  if (
    name === "AbortError" ||
    name === "TimeoutError" ||
    message === "Request timed out"
  ) {
    return "the server did not answer in time";
  }
  const code = (cause as { code?: unknown }).code;
  if (typeof code === "string" && /^E[A-Z_]+$/.test(code)) return code;
  return "the server did not give a valid answer";
}
