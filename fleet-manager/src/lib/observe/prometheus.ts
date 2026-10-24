import "server-only";
import { readSecretValues } from "@/lib/secrets/values";
import { getSettings } from "@/lib/settings";
import { appNamespace } from "@/lib/settings/keys";
import { prometheusAuthHeader, prometheusURL } from "@/lib/settings/prometheus";
import type { Series, TimeRange, Unavailable } from "./types";

const TIMEOUT_MS = 10_000;

type MatrixAnswer = {
  status?: unknown;
  data?: {
    resultType?: unknown;
    result?: { metric?: Record<string, string>; values?: [number, string][] }[];
  };
};

/**
 * promRange runs query over range on the Prometheus of the settings, with
 * the credentials of prometheus.credentialsSecretRef. It returns
 * "unavailable", and sends nothing, when no prometheus.url is set or the
 * Secret is missing or has no credentials. It also returns "unavailable"
 * when the server does not answer in 10 seconds, redirects, answers outside
 * HTTP 200, or answers without a matrix, which includes a query that
 * PromQL refuses. It rejects when the settings or the Secret cannot be
 * read. A series is named by its labels other than __name__, else by its
 * metric name, else by query. A point that is not a finite number is left
 * out.
 */
export async function promRange(
  query: string,
  range: TimeRange,
): Promise<Series[] | Unavailable> {
  const settings = await getSettings();
  const address = settings["prometheus.url"];
  if (!address) return "unavailable";
  const secret = settings["prometheus.credentialsSecretRef"];
  const values = secret
    ? await readSecretValues(appNamespace(), secret)
    : undefined;
  if (secret && !values) return "unavailable";
  const auth = prometheusAuthHeader(values);
  if ("missing" in auth) return "unavailable";

  const headers: Record<string, string> = { Accept: "application/json" };
  if (auth.header) headers.Authorization = auth.header;
  const url = prometheusURL(address, "/api/v1/query_range", {
    query,
    start: String(range.start / 1000),
    end: String(range.end / 1000),
    step: String(range.stepSeconds),
  });
  let body: MatrixAnswer;
  try {
    // A redirect would carry the credentials to another address.
    const response = await fetch(url, {
      headers,
      signal: AbortSignal.timeout(TIMEOUT_MS),
      redirect: "manual",
    });
    if (response.status !== 200) return "unavailable";
    body = (await response.json()) as MatrixAnswer;
  } catch {
    return "unavailable";
  }
  if (
    body?.status !== "success" ||
    body.data?.resultType !== "matrix" ||
    !Array.isArray(body.data.result)
  ) {
    return "unavailable";
  }
  return body.data.result.map(({ metric = {}, values = [] }) => ({
    name: seriesName(metric, query),
    points: values
      .map(([time, value]): [number, number] => [time * 1000, Number(value)])
      .filter(([, value]) => Number.isFinite(value)),
  }));
}

function seriesName(metric: Record<string, string>, query: string): string {
  const labels = Object.entries(metric)
    .filter(([key]) => key !== "__name__")
    .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
    .map(([key, value]) => `${key}="${value}"`);
  return labels.join(", ") || metric.__name__ || query;
}
