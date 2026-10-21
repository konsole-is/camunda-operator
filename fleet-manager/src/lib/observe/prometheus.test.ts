import http from "node:http";
import type { AddressInfo } from "node:net";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { KubernetesRequestError } from "@/lib/k8s/client";
import { readSecretValues } from "@/lib/secrets/values";
import { getSettings } from "@/lib/settings";
import { SETTING_KEYS, type Settings } from "@/lib/settings/keys";
import { promRange } from "./prometheus";
import type { TimeRange } from "./types";

vi.mock("@/lib/settings", () => ({ getSettings: vi.fn() }));
vi.mock("@/lib/secrets/values", () => ({ readSecretValues: vi.fn() }));

const settings = (values: Partial<Settings>): Settings => ({
  ...(Object.fromEntries(SETTING_KEYS.map((k) => [k, ""])) as Settings),
  ...values,
});

type Handler = (req: http.IncomingMessage, res: http.ServerResponse) => void;
const servers: http.Server[] = [];

async function serve(handler: Handler): Promise<string> {
  const server = http.createServer(handler);
  servers.push(server);
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
}

async function closedAddress(): Promise<string> {
  const url = await serve(() => {});
  const server = servers.pop()!;
  await new Promise((resolve) => server.close(resolve));
  return url;
}

const range: TimeRange = { start: 1_000, end: 61_000, stepSeconds: 15 };

function matrix(result: unknown[]) {
  return JSON.stringify({
    status: "success",
    data: { resultType: "matrix", result },
  });
}

// answer serves the matrix to a request with the Authorization header auth,
// and 401 to any other.
function answer(auth: string | undefined, result: unknown[] = []): Handler {
  return (req, res) => {
    if (req.headers.authorization !== auth) {
      res.writeHead(401).end();
      return;
    }
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(matrix(result));
  };
}

beforeEach(() => {
  vi.resetAllMocks();
  process.env.FLEET_NAMESPACE = "fleet";
});
afterEach(async () => {
  await Promise.all(
    servers.splice(0).map((s) => new Promise((resolve) => s.close(resolve))),
  );
  vi.restoreAllMocks();
});

describe("promRange", () => {
  it("returns unavailable and sends nothing when no URL is set", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    vi.mocked(getSettings).mockResolvedValue(settings({}));

    expect(await promRange("up", range)).toBe("unavailable");
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("returns the matrix as series in milliseconds", async () => {
    let asked: URL | undefined;
    const url = await serve((req, res) => {
      asked = new URL(req.url!, "http://x");
      res.end(
        matrix([
          {
            metric: { __name__: "up", pod: "a", namespace: "n" },
            values: [
              [1, "1"],
              [16, "0.5"],
              [31, "NaN"],
            ],
          },
        ]),
      );
    });
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "prometheus.url": `${url}/prom/` }),
    );

    expect(await promRange("up", range)).toEqual([
      {
        name: 'namespace="n", pod="a"',
        points: [
          [1000, 1],
          [16000, 0.5],
        ],
      },
    ]);
    expect(asked?.pathname).toBe("/prom/api/v1/query_range");
    expect(Object.fromEntries(asked!.searchParams)).toEqual({
      query: "up",
      start: "1",
      end: "61",
      step: "15",
    });
  });

  it("names a series with only a metric name by that name", async () => {
    const url = await serve(
      answer(undefined, [{ metric: { __name__: "up" }, values: [] }]),
    );
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "prometheus.url": url }),
    );

    expect(await promRange("up", range)).toEqual([{ name: "up", points: [] }]);
  });

  it("names a series without labels by its query", async () => {
    const url = await serve(answer(undefined, [{ metric: {}, values: [] }]));
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "prometheus.url": url }),
    );

    expect(await promRange("sum(up)", range)).toEqual([
      { name: "sum(up)", points: [] },
    ]);
  });

  it("sends a bearer token from the credentials Secret", async () => {
    const url = await serve(answer("Bearer tok-1"));
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "prometheus.url": url,
        "prometheus.credentialsSecretRef": "prom",
      }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({ token: "tok-1" });

    expect(await promRange("up", range)).toEqual([]);
    expect(readSecretValues).toHaveBeenCalledWith("fleet", "prom");
  });

  it("sends basic auth from username and password", async () => {
    const url = await serve(
      answer("Basic " + Buffer.from("admin:pw-1").toString("base64")),
    );
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "prometheus.url": url,
        "prometheus.credentialsSecretRef": "prom",
      }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({
      username: "admin",
      password: "pw-1",
      token: "unused",
    });

    expect(await promRange("up", range)).toEqual([]);
  });

  it("does not follow a redirect with the credentials", async () => {
    let moved = 0;
    const target = await serve((_req, res) => {
      moved++;
      res.end(matrix([]));
    });
    const url = await serve((req, res) =>
      res.writeHead(302, { Location: `${target}${req.url}` }).end(),
    );
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "prometheus.url": url,
        "prometheus.credentialsSecretRef": "prom",
      }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({ token: "tok-1" });

    expect(await promRange("up", range)).toBe("unavailable");
    expect(moved).toBe(0);
  });

  it.each<[string, Handler]>([
    ["HTTP 500", (_req, res) => res.writeHead(500).end()],
    ["a refusal", (_req, res) => res.writeHead(401).end()],
    [
      "a PromQL error",
      (_req, res) =>
        res
          .writeHead(400)
          .end(JSON.stringify({ status: "error", errorType: "bad_data" })),
    ],
    [
      "a body without status success",
      (_req, res) => res.end(JSON.stringify({ status: "error" })),
    ],
    ["a body that is not JSON", (_req, res) => res.end("<html>")],
    [
      "a result that is not a matrix",
      (_req, res) =>
        res.end(
          JSON.stringify({
            status: "success",
            data: { resultType: "vector", result: [] },
          }),
        ),
    ],
  ])("returns unavailable for %s", async (_, handler) => {
    const url = await serve(handler);
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "prometheus.url": url }),
    );

    expect(await promRange("up", range)).toBe("unavailable");
  });

  it("returns unavailable for a server that is down", async () => {
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "prometheus.url": await closedAddress() }),
    );

    expect(await promRange("up", range)).toBe("unavailable");
  });

  it.each([
    ["a missing credentials Secret", undefined],
    ["a Secret without credentials", { other: "x" }],
  ])("returns unavailable and sends nothing for %s", async (_, values) => {
    let asked = 0;
    const url = await serve((_req, res) => {
      asked++;
      res.end(matrix([]));
    });
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "prometheus.url": url,
        "prometheus.credentialsSecretRef": "prom",
      }),
    );
    vi.mocked(readSecretValues).mockResolvedValue(values);

    expect(await promRange("up", range)).toBe("unavailable");
    expect(asked).toBe(0);
  });

  it("rejects when the Secret cannot be read", async () => {
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "prometheus.url": "http://127.0.0.1:1",
        "prometheus.credentialsSecretRef": "prom",
      }),
    );
    vi.mocked(readSecretValues).mockRejectedValue(
      new KubernetesRequestError(403, "forbidden"),
    );

    await expect(promRange("up", range)).rejects.toThrow(KubernetesRequestError);
  });
});
