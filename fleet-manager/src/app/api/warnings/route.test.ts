import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fleetPodWarnings } from "@/lib/observe/warnings";
import type { PodWarnings } from "@/lib/warnings";

vi.mock("@/lib/observe/warnings", () => ({ fleetPodWarnings: vi.fn() }));

const warnings: PodWarnings = {
  clusters: {
    "team-a/orders": [
      {
        kind: "oom",
        severity: "Error",
        label: "Out of memory",
        detail: "Container zeebe of pod orders-zeebe-0 ran out of memory",
      },
    ],
  },
  missing: ["prometheus"],
};

// The route keeps its shared answer in the module, so each test loads a
// fresh copy.
async function route() {
  vi.resetModules();
  return import("./route");
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-11-10T12:00:00Z"));
});

afterEach(() => vi.useRealTimers());

describe("GET /api/warnings", () => {
  it("answers the warnings of the fleet", async () => {
    vi.mocked(fleetPodWarnings).mockResolvedValue(warnings);
    const { GET } = await route();

    const response = await GET();

    expect(response.status).toBe(200);
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(await response.json()).toEqual({ warnings });
    expect(fleetPodWarnings).toHaveBeenCalledWith(
      new Date("2026-11-10T12:00:00Z"),
    );
  });

  it("answers 502 with a fixed text when the read fails", async () => {
    vi.mocked(fleetPodWarnings).mockRejectedValue(
      new Error("pods is forbidden: User system:serviceaccount"),
    );
    const { GET } = await route();

    const response = await GET();

    expect(response.status).toBe(502);
    expect(await response.text()).toBe(
      "Could not read the warnings of the pods",
    );
  });

  it("shares one answer between two calls within 10 seconds", async () => {
    vi.mocked(fleetPodWarnings).mockResolvedValue(warnings);
    const { GET } = await route();

    await GET();
    vi.advanceTimersByTime(9_999);
    await GET();
    expect(fleetPodWarnings).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(1);
    await GET();
    expect(fleetPodWarnings).toHaveBeenCalledTimes(2);
  });

  it("reads again after a failed read", async () => {
    vi.mocked(fleetPodWarnings)
      .mockRejectedValueOnce(new Error("timeout"))
      .mockResolvedValue(warnings);
    const { GET } = await route();

    expect((await GET()).status).toBe(502);
    const response = await GET();

    expect(response.status).toBe(200);
    expect(fleetPodWarnings).toHaveBeenCalledTimes(2);
  });
});
