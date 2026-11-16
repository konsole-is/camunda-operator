// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  applyPresetChange,
  previewPresetChange,
} from "@/app/setup/presets/actions";
import { get } from "@/lib/k8s/read";
import { writeMode } from "@/lib/write/writer";
import PresetPage, { dynamic } from "./page";

vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
}));
vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async () => []),
  get: vi.fn(async (kind: string, _ns: unknown, name: string) =>
    kind === "ElasticsearchClusterPreset" && name === "standard"
      ? { kind, metadata: { name } }
      : undefined,
  ),
}));
vi.mock("@/lib/write/writer", () => ({
  writeMode: vi.fn(async () => "direct"),
}));
vi.mock("@/app/setup/presets/actions", () => ({
  previewPresetChange: vi.fn(),
  applyPresetChange: vi.fn(),
}));
const detail = vi.hoisted(() => ({ actions: undefined as unknown }));
vi.mock("@/components/setup/presets/preset-detail", () => ({
  PresetDetailLive: (props: {
    kind: string;
    name: string;
    mode?: string;
    actions: unknown;
  }) => {
    detail.actions = props.actions;
    return (
      <p>
        detail {props.kind} {props.name} {props.mode ?? "unknown"}
      </p>
    );
  },
}));

afterEach(cleanup);

const params = (kind: string, name: string) => ({
  params: Promise.resolve({ kind, name }),
  searchParams: Promise.resolve({}),
});

describe("PresetPage", () => {
  it("is not found for an unknown kind segment", async () => {
    await expect(PresetPage(params("redis", "x"))).rejects.toThrow(
      "NEXT_NOT_FOUND",
    );
  });

  it("is not found for a preset that does not exist", async () => {
    await expect(PresetPage(params("postgresql", "standard"))).rejects.toThrow(
      "NEXT_NOT_FOUND",
    );
    expect(vi.mocked(get)).toHaveBeenCalledWith(
      "DatabaseServerPreset",
      undefined,
      "standard",
    );
  });

  it("opens the detail of the preset of the kind of the path", async () => {
    vi.mocked(writeMode).mockResolvedValueOnce("git");
    render(await PresetPage(params("elasticsearch", "standard")));
    expect(
      screen.getByText("detail ElasticsearchClusterPreset standard git"),
    ).toBeTruthy();
    expect(dynamic).toBe("force-dynamic");
  });

  it("passes no write mode when the settings cannot be read", async () => {
    vi.mocked(writeMode).mockRejectedValueOnce(new Error("forbidden"));
    render(await PresetPage(params("elasticsearch", "standard")));
    expect(
      screen.getByText("detail ElasticsearchClusterPreset standard unknown"),
    ).toBeTruthy();
  });

  it("gives the detail the server actions of the presets", async () => {
    render(await PresetPage(params("elasticsearch", "standard")));
    expect(detail.actions).toEqual({
      previewChange: previewPresetChange,
      applyChange: applyPresetChange,
    });
  });
});
