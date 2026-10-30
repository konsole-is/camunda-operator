import { beforeEach, describe, expect, it, vi } from "vitest";
import { podUsage } from "@/lib/observe/kube";
import { GET } from "./route";

vi.mock("@/lib/observe/kube", () => ({ podUsage: vi.fn() }));

const call = (query: string) =>
  GET(new Request(`http://localhost/api/usage?${query}`));

const usage = [
  {
    pod: "orders-zeebe-0",
    cpu: { usedCores: 0.5, limitCores: 1 },
    memory: { usedBytes: 100 },
  },
];

beforeEach(() => vi.resetAllMocks());

describe("GET /api/usage", () => {
  it.each([
    ["no namespace", "selector=a%3Db", "namespace"],
    [
      "a namespace with ..",
      `namespace=${encodeURIComponent("../../../../api/v1/namespaces/kube-system/secrets?")}&selector=a%3Db`,
      "namespace",
    ],
    ["the namespace ..", "namespace=..&selector=a%3Db", "namespace"],
    ["a namespace of spaces", "namespace=%20%20&selector=a%3Db", "namespace"],
    ["no selector", "namespace=n", "selector"],
    ["an empty selector", "namespace=n&selector=", "selector"],
    ["a selector of spaces", "namespace=n&selector=%20%20", "selector"],
  ])("answers 400 for %s and names the parameter", async (_, query, name) => {
    const response = await call(query);

    expect(response.status).toBe(400);
    expect(await response.text()).toContain(name);
    expect(podUsage).not.toHaveBeenCalled();
  });

  it.each(["", "%20%20"])(
    "the empty selector %j is refused and lists no pods",
    async (selector) => {
      await call(`namespace=n&selector=${selector}`);

      expect(podUsage).not.toHaveBeenCalled();
    },
  );

  it("passes the selector without the spaces around it", async () => {
    vi.mocked(podUsage).mockResolvedValue([]);

    await call("namespace=n&selector=%20a%3Db%20");

    expect(podUsage).toHaveBeenCalledWith("n", "a=b");
  });

  it("answers the usage of the pods that the selector selects", async () => {
    vi.mocked(podUsage).mockResolvedValue(usage);

    const response = await call(
      `namespace=team-a&selector=${encodeURIComponent("camunda.io/cluster=orders")}`,
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(await response.json()).toEqual({ usage });
    expect(podUsage).toHaveBeenCalledWith(
      "team-a",
      "camunda.io/cluster=orders",
    );
  });

  it("answers unavailable when no metrics server serves the usage", async () => {
    vi.mocked(podUsage).mockResolvedValue("unavailable");

    const response = await call("namespace=n&selector=a%3Db");

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ usage: "unavailable" });
  });

  it("answers 502 with a fixed text when the read fails", async () => {
    vi.mocked(podUsage).mockRejectedValue(new Error("secret detail"));

    const response = await call("namespace=n&selector=a%3Db");

    expect(response.status).toBe(502);
    expect(await response.text()).toBe("Could not read the usage of the pods");
  });
});
