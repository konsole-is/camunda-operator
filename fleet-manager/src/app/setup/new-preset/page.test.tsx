// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  applyPresetChange,
  previewPresetChange,
} from "@/app/setup/presets/actions";
import { writeMode } from "@/lib/write/writer";
import NewPresetPage, { dynamic } from "./page";

vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async () => []),
}));
vi.mock("@/lib/write/writer", () => ({
  writeMode: vi.fn(async () => "direct"),
}));
vi.mock("@/app/setup/presets/actions", () => ({
  previewPresetChange: vi.fn(),
  applyPresetChange: vi.fn(),
}));
const form = vi.hoisted(() => ({ actions: undefined as unknown }));
vi.mock("@/components/setup/presets/preset-create", () => ({
  PresetCreateLive: (props: { mode?: string; actions: unknown }) => {
    form.actions = props.actions;
    return <p>form {props.mode ?? "unknown"}</p>;
  },
}));

afterEach(cleanup);

describe("NewPresetPage", () => {
  it("passes the write mode of the settings", async () => {
    vi.mocked(writeMode).mockResolvedValueOnce("git");
    render(await NewPresetPage());
    expect(screen.getByText("form git")).toBeTruthy();
  });

  it("passes no write mode when the settings cannot be read", async () => {
    vi.mocked(writeMode).mockRejectedValueOnce(new Error("forbidden"));
    render(await NewPresetPage());
    expect(screen.getByText("form unknown")).toBeTruthy();
  });

  it("gives the form the server actions of the presets", async () => {
    render(await NewPresetPage());
    expect(form.actions).toEqual({
      previewChange: previewPresetChange,
      applyChange: applyPresetChange,
    });
  });

  it("renders on each request", () => {
    expect(dynamic).toBe("force-dynamic");
  });
});
