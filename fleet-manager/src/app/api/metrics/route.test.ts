import { beforeEach, describe, expect, it, vi } from "vitest";
import { componentMetrics, requestMetrics } from "@/lib/observe/metrics";
import type { MetricsAnswer } from "@/lib/observe/types";
import { GET } from "./route";

vi.mock("@/lib/observe/metrics", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/observe/metrics")>()),
  componentMetrics: vi.fn(),
  requestMetrics: vi.fn(),
}));

const call = (query: string) =>
  GET(new Request(`http://localhost/api/metrics?${query}`));

beforeEach(() => vi.resetAllMocks());

describe("GET /api/metrics", () => {
  it.each([
    ["no namespace", "cluster=o&component=zeebe", "namespace"],
    [
      "a namespace with a slash",
      "namespace=a%2Fb&cluster=o&component=zeebe",
      "namespace",
    ],
    ["a cluster without a component", "namespace=n&cluster=o", "component"],
    [
      "an unknown component",
      "namespace=n&cluster=o&component=elasticsearch",
      "component",
    ],
    ["a component without a cluster", "namespace=n&component=zeebe", "cluster"],
    [
      "a cluster name with a slash",
      "namespace=n&cluster=a%2Fb&component=zeebe",
      "cluster",
    ],
    [
      "a cluster and a request",
      "namespace=n&cluster=o&component=zeebe&request=r",
      "request",
    ],
    [
      "a component and a request",
      "namespace=n&component=zeebe&request=r",
      "request",
    ],
    ["neither a cluster nor a request", "namespace=n", "cluster"],
    ["a request name with a slash", "namespace=n&request=a%2Fb", "request"],
    ["an unknown range", "namespace=n&request=r&range=7d", "range"],
  ])("answers 400 for %s and names the parameter", async (_, query, name) => {
    const response = await call(query);

    expect(response.status).toBe(400);
    expect(await response.text()).toContain(name);
    expect(componentMetrics).not.toHaveBeenCalled();
    expect(requestMetrics).not.toHaveBeenCalled();
  });

  it("answers the charts of a component over the last hour by default", async () => {
    vi.mocked(componentMetrics).mockResolvedValue({
      prometheus: "notConfigured",
    });

    const response = await call(
      "namespace=team-a&cluster=orders&component=zeebe",
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(await response.json()).toEqual({
      metrics: { prometheus: "notConfigured" },
    });
    expect(componentMetrics).toHaveBeenCalledWith(
      "team-a",
      "orders",
      "zeebe",
      "1h",
      expect.any(Number),
    );
  });

  it("answers the charts of Optimize", async () => {
    vi.mocked(componentMetrics).mockResolvedValue({
      prometheus: "unavailable",
    });

    const response = await call(
      "namespace=team-a&cluster=orders&component=optimize",
    );

    expect(response.status).toBe(200);
    expect(vi.mocked(componentMetrics).mock.calls[0][2]).toBe("optimize");
  });

  it("answers the charts of a request over the range", async () => {
    const metrics: MetricsAnswer = {
      prometheus: "answered",
      charts: [],
      errors: [],
    };
    vi.mocked(requestMetrics).mockResolvedValue(metrics);

    const response = await call(
      "namespace=team-a&request=orders-backup-storage&range=24h",
    );

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ metrics });
    expect(requestMetrics).toHaveBeenCalledWith(
      "team-a",
      "orders-backup-storage",
      "24h",
      expect.any(Number),
    );
    expect(componentMetrics).not.toHaveBeenCalled();
  });

  it("answers 404 for a cluster that does not exist", async () => {
    vi.mocked(componentMetrics).mockResolvedValue(undefined);

    const response = await call(
      "namespace=team-a&cluster=gone&component=zeebe",
    );

    expect(response.status).toBe(404);
    expect(await response.text()).toBe(
      "The cluster team-a/gone does not exist",
    );
  });

  it("answers 404 for a request that does not exist", async () => {
    vi.mocked(requestMetrics).mockResolvedValue(undefined);

    const response = await call("namespace=team-a&request=gone");

    expect(response.status).toBe(404);
    expect(await response.text()).toBe(
      "The request team-a/gone does not exist",
    );
  });

  it("answers 502 with a fixed text when the read fails", async () => {
    vi.mocked(componentMetrics).mockRejectedValue(
      new Error("secret token xyz"),
    );

    const response = await call(
      "namespace=team-a&cluster=orders&component=zeebe",
    );

    expect(response.status).toBe(502);
    const text = await response.text();
    expect(text).toBe("Could not read the metrics");
    expect(text).not.toContain("xyz");
  });
});
