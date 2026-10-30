import { beforeEach, describe, expect, it, vi } from "vitest";
import { KubernetesRequestError, getJSON } from "@/lib/k8s/client";
import type { Change, Writer } from "@/lib/write/types";
import { directWriter, getWriter } from "@/lib/write/writer";
import { getSettings, readSettings, updateSettings } from "./store";

vi.mock("@/lib/k8s/client", async (real) => ({
  ...(await real<typeof import("@/lib/k8s/client")>()),
  getJSON: vi.fn(),
  requestJSON: vi.fn(),
}));
vi.mock("@/lib/write/writer", async (real) => ({
  ...(await real<typeof import("@/lib/write/writer")>()),
  directWriter: vi.fn(),
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
  vi.mocked(directWriter).mockReturnValue(writer);
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

  it("a first create does not take over keys that an admin set at the same time", async () => {
    vi.mocked(getJSON).mockRejectedValue(
      new KubernetesRequestError(404, "not found"),
    );
    apply.mockImplementation(async (change) =>
      change.force
        ? { ok: true, mode: "direct" }
        : {
            ok: false,
            errors: [
              {
                code: "Conflict",
                message: 'conflict with "kubectl-create"',
                manager: "kubectl-create",
              },
            ],
          },
    );

    await expect(getSettings()).rejects.toThrow(
      'conflict with "kubectl-create"',
    );
  });

  it("reads wait for no write", async () => {
    vi.mocked(getJSON).mockResolvedValue(configMap({ "git.path": "a" }, "1"));
    let release = () => {};
    apply.mockImplementation(
      () =>
        new Promise((resolve) => {
          release = () => resolve({ ok: true, mode: "direct" });
        }),
    );

    const update = updateSettings({ "git.path": "b" });
    await new Promise((resolve) => setTimeout(resolve, 5));

    expect((await getSettings())["git.path"]).toBe("a");
    release();
    await update;
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

describe("reads of the ConfigMap", () => {
  it("each read has a deadline, so a stalled answer cannot hold the settings", async () => {
    vi.mocked(getJSON)
      .mockRejectedValueOnce(new KubernetesRequestError(404, "not found"))
      .mockResolvedValue(configMap({}, "1"));

    await getSettings();

    expect(getJSON).toHaveBeenCalledTimes(2);
    for (const call of vi.mocked(getJSON).mock.calls) {
      expect(call[2]).toBeGreaterThan(0);
    }
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

describe("a first read beside an update", () => {
  it("a slow first create does not undo an update", async () => {
    let live: Record<string, string> | undefined;
    vi.mocked(getJSON).mockImplementation(async () => {
      if (!live) throw new KubernetesRequestError(404, "not found");
      return configMap(live, "1");
    });
    let release = () => {};
    const slow = new Promise<void>((resolve) => (release = resolve));
    apply.mockImplementation(async (change) => {
      if (apply.mock.calls.length === 1) await slow;
      live = change.apply[0].data as Record<string, string>;
      return { ok: true, mode: "direct" };
    });

    const first = getSettings();
    const update = updateSettings({ "git.path": "fleet" });
    await new Promise((resolve) => setTimeout(resolve, 10));
    release();
    await Promise.all([first, update]);

    expect(live?.["git.path"]).toBe("fleet");
  });
});

// Contract 6.1: the settings never go to Git.
describe("in Git mode", () => {
  it("the first create and each update go to the API, not to Git", async () => {
    const missing = new KubernetesRequestError(404, "not found");
    vi.mocked(getJSON)
      .mockRejectedValueOnce(missing)
      .mockRejectedValueOnce(missing)
      .mockResolvedValue(configMap({}, "1"));

    await getSettings();
    await updateSettings({ "git.path": "fleet" });

    expect(apply).toHaveBeenCalledTimes(2);
    for (const [change] of apply.mock.calls) {
      expect(change.directOnly).toBe(true);
    }
    expect(getWriter).not.toHaveBeenCalled();
  });
});

describe("readSettings", () => {
  it("returns the settings with the resourceVersion of the ConfigMap", async () => {
    vi.mocked(getJSON).mockResolvedValue(
      configMap({ kubernetesClusterName: "a" }, "7"),
    );

    const read = await readSettings();

    expect(read.resourceVersion).toBe("7");
    expect(read.settings.kubernetesClusterName).toBe("a");
  });
});

describe("updateSettings with the version that the page read", () => {
  it("a changed ConfigMap gives Stale and sends no write", async () => {
    vi.mocked(getJSON).mockResolvedValue(
      configMap({ kubernetesClusterName: "a" }, "8"),
    );

    expect(await updateSettings({ kubernetesClusterName: "b" }, "7")).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Stale",
          message:
            "The ConfigMap fleet-manager-settings changed since you opened it.",
        }),
      ],
    });
    expect(apply).not.toHaveBeenCalled();
  });

  it("a deleted ConfigMap gives Stale and is not created again", async () => {
    vi.mocked(getJSON).mockRejectedValue(
      new KubernetesRequestError(404, "not found"),
    );

    expect(await updateSettings({ kubernetesClusterName: "b" }, "7")).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale" })],
    });
    expect(apply).not.toHaveBeenCalled();
  });

  it("the same version is the precondition of the write", async () => {
    vi.mocked(getJSON).mockResolvedValue(
      configMap({ kubernetesClusterName: "a" }, "7"),
    );

    await updateSettings({ kubernetesClusterName: "b" }, "7");

    expect((apply.mock.calls[0][0] as Change).preconditions).toEqual({
      "ConfigMap/fleet/fleet-manager-settings": "7",
    });
  });
});
