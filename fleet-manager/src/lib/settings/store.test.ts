import { beforeEach, describe, expect, it, vi } from "vitest";
import { KubernetesRequestError, getJSON } from "@/lib/k8s/client";
import type { Change, Writer } from "@/lib/write/types";
import { getWriter } from "@/lib/write/writer";
import { getSettings, updateSettings } from "./store";

vi.mock("@/lib/k8s/client", async (real) => ({
  ...(await real<typeof import("@/lib/k8s/client")>()),
  getJSON: vi.fn(),
  requestJSON: vi.fn(),
}));
vi.mock("@/lib/write/writer", async (real) => ({
  ...(await real<typeof import("@/lib/write/writer")>()),
  getWriter: vi.fn(),
}));

const apply = vi.fn<Writer["apply"]>();
const writer: Writer = { mode: "direct", preview: vi.fn(), apply };

const configMap = (data: Record<string, string>, resourceVersion: string) => ({
  apiVersion: "v1",
  kind: "ConfigMap",
  metadata: {
    name: "fleet-manager-settings",
    namespace: "fleet",
    resourceVersion,
  },
  data,
});

beforeEach(() => {
  vi.resetAllMocks();
  process.env.FLEET_NAMESPACE = "fleet";
  vi.mocked(getWriter).mockResolvedValue(writer);
  apply.mockResolvedValue({ ok: true, mode: "direct" });
});

describe("getSettings", () => {
  it("reads the ConfigMap of FLEET_NAMESPACE and writes nothing when it exists", async () => {
    vi.mocked(getJSON).mockResolvedValue(
      configMap({ kubernetesClusterName: "a" }, "7"),
    );

    expect((await getSettings()).kubernetesClusterName).toBe("a");
    expect(vi.mocked(getJSON).mock.calls[0][1]).toBe(
      "/api/v1/namespaces/fleet/configmaps/fleet-manager-settings",
    );
    expect(apply).not.toHaveBeenCalled();
  });

  it("a refused create throws with the message of the API server", async () => {
    vi.mocked(getJSON).mockRejectedValue(
      new KubernetesRequestError(404, "not found"),
    );
    apply.mockResolvedValue({
      ok: false,
      errors: [{ code: "Forbidden", message: "cannot patch configmaps" }],
    });

    await expect(getSettings()).rejects.toThrow("cannot patch configmaps");
  });
});

describe("updateSettings", () => {
  it("applies the read data with the patch and the resourceVersion it read", async () => {
    vi.mocked(getJSON).mockResolvedValue(
      configMap({ kubernetesClusterName: "a", "custom.note": "x" }, "7"),
    );

    await updateSettings({ kubernetesClusterName: "b" });

    expect(apply).toHaveBeenCalledTimes(1);
    const change = apply.mock.calls[0][0] as Change;
    expect(change.apply).toEqual([
      expect.objectContaining({
        kind: "ConfigMap",
        metadata: { name: "fleet-manager-settings", namespace: "fleet" },
        data: { kubernetesClusterName: "b", "custom.note": "x" },
      }),
    ]);
    expect(change.preconditions).toEqual({
      "ConfigMap/fleet/fleet-manager-settings": "7",
    });
  });

  it("returns Stale from the writer and does not retry", async () => {
    vi.mocked(getJSON).mockResolvedValue(
      configMap({ kubernetesClusterName: "a" }, "7"),
    );
    apply.mockResolvedValue({
      ok: false,
      errors: [{ code: "Stale", message: "changed" }],
    });

    expect(await updateSettings({ kubernetesClusterName: "b" })).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale" })],
    });
    expect(apply).toHaveBeenCalledTimes(1);
  });

  it("an invalid patch sends no request", async () => {
    expect(await updateSettings({ "secretStore.kind": "Vault" })).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Invalid" })],
    });
    expect(getJSON).not.toHaveBeenCalled();
    expect(apply).not.toHaveBeenCalled();
  });

  it("an API server that does not answer gives Unavailable", async () => {
    vi.mocked(getJSON).mockRejectedValue(new Error("connect ECONNREFUSED"));

    expect(await updateSettings({ "git.path": "fleet" })).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Unavailable",
          message: "connect ECONNREFUSED",
        }),
      ],
    });
  });
});
