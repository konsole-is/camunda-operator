// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  applyPlatformConfigChange,
  previewPlatformConfigChange,
} from "@/app/setup/platform/actions";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { PlatformCreateLive } from "@/components/setup/platform/platform-create";
import NewPlatformConfigPage, { dynamic } from "./page";

const settings = vi.hoisted(() => ({ failing: false }));
const seen = vi.hoisted(() => ({
  props: undefined as Parameters<typeof PlatformCreateLive>[0] | undefined,
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async (kind: string) =>
    kind === "CamundaPlatformConfig"
      ? [
          {
            apiVersion: "core.camunda.io/v1",
            kind: "CamundaPlatformConfig",
            metadata: { name: "basic-lab" },
            spec: {},
          },
        ]
      : [],
  ),
}));
vi.mock("@/lib/write/writer", () => ({
  writeMode: vi.fn(async () => {
    if (settings.failing) throw new Error("no settings");
    return "git";
  }),
}));
vi.mock("@/lib/bundled", () => ({
  getBundledData: vi.fn(async () => ({
    imagesTable: { schemaVersion: 1, minors: { "8.9": {} } },
  })),
}));
vi.mock("@/app/setup/platform/actions", () => ({
  previewPlatformConfigChange: vi.fn(),
  applyPlatformConfigChange: vi.fn(),
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (_kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
  }),
  useWatchesFresh: (lists: unknown[]) => lists.map(() => true),
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));
vi.mock(
  "@/components/setup/platform/platform-create",
  async (importOriginal) => {
    const actual =
      await importOriginal<
        typeof import("@/components/setup/platform/platform-create")
      >();
    return {
      ...actual,
      PlatformCreateLive: (
        props: Parameters<typeof actual.PlatformCreateLive>[0],
      ) => {
        seen.props = props;
        return <actual.PlatformCreateLive {...props} />;
      },
    };
  },
);

afterEach(() => {
  cleanup();
  settings.failing = false;
  seen.props = undefined;
});

const show = async () =>
  render(await NewPlatformConfigPage(), { wrapper: GitPushesProvider });

describe("NewPlatformConfigPage", () => {
  it("renders the create form with the configs of the list", async () => {
    await show();

    expect(
      screen.getByRole("heading", { level: 1, name: "New platform config" }),
    ).toBeTruthy();
    expect(screen.getByRole("option", { name: "basic-lab" })).toBeTruthy();
  });

  it("passes the write mode of the settings", async () => {
    await show();
    expect(seen.props?.mode).toBe("git");
  });

  it("passes no write mode when the settings cannot be read", async () => {
    settings.failing = true;
    await show();
    expect(seen.props?.mode).toBeUndefined();
  });

  it("gives the form the server actions of the platform configs", async () => {
    await show();
    expect(seen.props?.actions).toEqual({
      previewChange: previewPlatformConfigChange,
      applyChange: applyPlatformConfigChange,
    });
  });

  it("passes the images table", async () => {
    await show();
    expect(seen.props?.imagesTable).toEqual({
      schemaVersion: 1,
      minors: { "8.9": {} },
    });
  });

  it("renders on each request", () => {
    expect(dynamic).toBe("force-dynamic");
  });
});
