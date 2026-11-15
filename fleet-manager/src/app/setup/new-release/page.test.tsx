// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import NewReleasePage, { dynamic } from "./page";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async (kind: string) =>
    kind === "CamundaRelease"
      ? [
          {
            apiVersion: "core.camunda.io/v1",
            kind: "CamundaRelease",
            metadata: { name: "camunda-8-9-0" },
            spec: { version: "8.9.0" },
          },
        ]
      : [],
  ),
}));
vi.mock("@/lib/write/writer", () => ({
  writeMode: vi.fn(async () => "direct"),
}));
vi.mock("@/lib/bundled", () => ({
  getBundledData: vi.fn(async () => ({
    imagesTable: { schemaVersion: 1, minors: {} },
  })),
}));
vi.mock("@/app/setup/releases/actions", () => ({
  previewReleaseChange: vi.fn(),
  applyReleaseChange: vi.fn(),
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (_kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
  }),
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

afterEach(cleanup);

const show = async (from?: string | string[]) =>
  render(
    await NewReleasePage({
      params: Promise.resolve({}),
      searchParams: Promise.resolve(from === undefined ? {} : { from }),
    }),
    { wrapper: GitPushesProvider },
  );

describe("NewReleasePage", () => {
  it("copies the release of the from query", async () => {
    await show(["camunda-8-9-0", "x"]);
    expect(
      screen.getByRole("heading", { level: 1, name: "New release" }),
    ).toBeTruthy();
    expect(
      (screen.getByLabelText("Copy from") as HTMLSelectElement).value,
    ).toBe("camunda-8-9-0");
    expect(
      (screen.getByLabelText("Camunda version") as HTMLInputElement).value,
    ).toBe("8.9.0");
    expect(dynamic).toBe("force-dynamic");
  });

  it("starts with an empty Copy from choice without a query", async () => {
    await show();
    expect(
      (screen.getByLabelText("Copy from") as HTMLSelectElement).value,
    ).toBe("");
    expect(
      (screen.getByLabelText("Camunda version") as HTMLInputElement).value,
    ).toBe("");
  });
});
