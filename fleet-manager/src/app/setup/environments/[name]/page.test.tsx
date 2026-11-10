// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { writeMode } from "@/lib/write/writer";
import EnvironmentPage, { dynamic } from "./page";

const staging = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "staging" },
  spec: {},
};

vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
}));
vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async () => []),
  get: vi.fn(async (_kind: string, _ns: unknown, name: string) =>
    name === "staging" ? staging : undefined,
  ),
}));
vi.mock("@/lib/write/writer", () => ({
  writeMode: vi.fn(async () => "direct"),
}));
vi.mock("@/components/setup/environments/environment-editor", () => ({
  EnvironmentEditorLive: (props: { name?: string; mode: string }) => (
    <p>
      editor {props.name} {props.mode}
    </p>
  ),
}));

afterEach(cleanup);

const params = (name: string) => ({
  params: Promise.resolve({ name }),
  searchParams: Promise.resolve({}),
});

describe("EnvironmentPage", () => {
  it("calls notFound for an environment that is not in the cache", async () => {
    await expect(EnvironmentPage(params("gone"))).rejects.toThrow(
      "NEXT_NOT_FOUND",
    );
  });

  it("passes the write mode of the settings", async () => {
    vi.mocked(writeMode).mockResolvedValueOnce("git");
    render(await EnvironmentPage(params("staging")));
    expect(screen.getByText("editor staging git")).toBeTruthy();
    expect(dynamic).toBe("force-dynamic");
  });

  it("takes the direct mode when the settings cannot be read", async () => {
    vi.mocked(writeMode).mockRejectedValueOnce(new Error("forbidden"));
    render(await EnvironmentPage(params("staging")));
    expect(screen.getByText("editor staging direct")).toBeTruthy();
  });
});
