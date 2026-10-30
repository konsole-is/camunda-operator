import { beforeEach, describe, expect, it, vi } from "vitest";
import { get } from "@/lib/k8s/read";
import { brokerPartitions } from "@/lib/observe/partitions";
import { GET } from "./route";

vi.mock("@/lib/k8s/read", () => ({ get: vi.fn() }));
vi.mock("@/lib/observe/partitions", () => ({ brokerPartitions: vi.fn() }));

const call = (query: string) =>
  GET(new Request(`http://localhost/api/partitions?${query}`));

const cluster = { metadata: { name: "orders", namespace: "team-a" } };
const partitions = [
  { partition: 1, leader: "orders-zeebe-0", followers: [], health: "healthy" },
] as const;

beforeEach(() => vi.resetAllMocks());

describe("GET /api/partitions", () => {
  it.each([
    ["no namespace", "name=orders", "namespace"],
    ["no name", "namespace=team-a", "name"],
  ])("answers 400 for %s and names the parameter", async (_, query, name) => {
    const response = await call(query);

    expect(response.status).toBe(400);
    expect(await response.text()).toContain(name);
    expect(get).not.toHaveBeenCalled();
  });

  it("answers the partitions of the cluster", async () => {
    vi.mocked(get).mockResolvedValue(cluster as never);
    vi.mocked(brokerPartitions).mockResolvedValue([...partitions]);

    const response = await call("namespace=team-a&name=orders");

    expect(response.status).toBe(200);
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(await response.json()).toEqual({ partitions });
    expect(get).toHaveBeenCalledWith("CamundaCluster", "team-a", "orders");
    expect(brokerPartitions).toHaveBeenCalledWith(cluster, expect.any(Number));
  });

  it("answers unavailable when the management API does not answer", async () => {
    vi.mocked(get).mockResolvedValue(cluster as never);
    vi.mocked(brokerPartitions).mockResolvedValue("unavailable");

    const response = await call("namespace=team-a&name=orders");

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ partitions: "unavailable" });
  });

  it("answers 404 when the cluster does not exist", async () => {
    vi.mocked(get).mockResolvedValue(undefined);

    const response = await call("namespace=team-a&name=orders");

    expect(response.status).toBe(404);
    expect(await response.text()).toBe(
      "The cluster team-a/orders does not exist",
    );
    expect(brokerPartitions).not.toHaveBeenCalled();
  });

  it("answers 502 with a fixed text when the read fails", async () => {
    vi.mocked(get).mockResolvedValue(cluster as never);
    vi.mocked(brokerPartitions).mockRejectedValue(new Error("secret detail"));

    const response = await call("namespace=team-a&name=orders");

    expect(response.status).toBe(502);
    expect(await response.text()).toBe(
      "Could not read the partitions of cluster team-a/orders",
    );
  });
});
