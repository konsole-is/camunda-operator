import { beforeEach, describe, expect, it, vi } from "vitest";
import { get } from "@/lib/k8s/read";
import { controllerPods } from "@/lib/observe/kube";
import { GET } from "./route";

vi.mock("@/lib/k8s/read", () => ({ get: vi.fn() }));
vi.mock("@/lib/observe/kube", () => ({ controllerPods: vi.fn() }));

const call = (query: string) =>
  GET(new Request(`http://localhost/api/controller-pods?${query}`));

const registration = {
  metadata: { name: "rustfs-bucket" },
  spec: { serviceAccount: { namespace: "plugins", name: "rustfs-controller" } },
};

beforeEach(() => vi.resetAllMocks());

describe("GET /api/controller-pods", () => {
  it.each([
    ["no registration", ""],
    ["the name ..", "registration=.."],
    ["a name with a slash", "registration=a%2Fb"],
    ["a name with capitals", "registration=Rustfs"],
  ])("answers 400 for %s and names the parameter", async (_, query) => {
    const response = await call(query);

    expect(response.status).toBe(400);
    expect(await response.text()).toContain("registration");
    expect(get).not.toHaveBeenCalled();
  });

  it("answers the namespace and the names of the controller pods", async () => {
    vi.mocked(get).mockResolvedValue(registration as never);
    vi.mocked(controllerPods).mockResolvedValue([
      { metadata: { name: "rustfs-controller-0" } },
    ] as never);

    const response = await call("registration=rustfs-bucket");

    expect(response.status).toBe(200);
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(await response.json()).toEqual({
      controller: { namespace: "plugins", pods: ["rustfs-controller-0"] },
    });
    expect(get).toHaveBeenCalledWith(
      "PluginRegistration",
      undefined,
      "rustfs-bucket",
    );
    expect(controllerPods).toHaveBeenCalledWith(registration);
  });

  it("answers 404 when the registration does not exist", async () => {
    vi.mocked(get).mockResolvedValue(undefined);

    const response = await call("registration=rustfs-bucket");

    expect(response.status).toBe(404);
    expect(await response.text()).toBe(
      "The registration rustfs-bucket does not exist",
    );
    expect(controllerPods).not.toHaveBeenCalled();
  });

  it("answers 502 with a fixed text when the read fails", async () => {
    vi.mocked(get).mockResolvedValue(registration as never);
    vi.mocked(controllerPods).mockRejectedValue(new Error("secret detail"));

    const response = await call("registration=rustfs-bucket");

    expect(response.status).toBe(502);
    expect(await response.text()).toBe("Could not read the controller pods");
  });
});
