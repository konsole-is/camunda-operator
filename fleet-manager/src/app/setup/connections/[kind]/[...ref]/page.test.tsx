// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { get } from "@/lib/k8s/read";
import ConnectionPage, { dynamic } from "./page";

vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
}));
vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async () => []),
  get: vi.fn(
    async (kind: string, namespace: string | undefined, name: string) =>
      (kind === "ObjectStorageConfig" &&
        namespace === "team-a" &&
        name === "backups") ||
      (kind === "ManagementAuthConfig" &&
        namespace === undefined &&
        name === "plane-a")
        ? { kind, metadata: { namespace, name } }
        : undefined,
  ),
}));
vi.mock("@/components/setup/connections/connection-detail", () => ({
  ConnectionDetailLive: (props: {
    target: { kind: string; namespace?: string; name: string };
  }) => (
    <p>
      detail {props.target.kind} {props.target.namespace ?? "-"}{" "}
      {props.target.name}
    </p>
  ),
}));

afterEach(cleanup);

const params = (kind: string, ref: string[]) => ({
  params: Promise.resolve({ kind, ref }),
  searchParams: Promise.resolve({}),
});

describe("ConnectionPage", () => {
  it("opens a namespaced contract from its namespace and name", async () => {
    render(
      await ConnectionPage(params("object-storage", ["team-a", "backups"])),
    );
    expect(
      screen.getByText("detail ObjectStorageConfig team-a backups"),
    ).toBeTruthy();
    expect(dynamic).toBe("force-dynamic");
  });

  it("opens a Management Identity contract from its name", async () => {
    render(await ConnectionPage(params("management-identity", ["plane-a"])));
    expect(
      screen.getByText("detail ManagementAuthConfig - plane-a"),
    ).toBeTruthy();
    expect(vi.mocked(get)).toHaveBeenCalledWith(
      "ManagementAuthConfig",
      undefined,
      "plane-a",
    );
  });

  it.each<[string, string[]]>([
    ["buckets", ["team-a", "backups"]],
    ["object-storage", ["backups"]],
    ["management-identity", ["team-a", "plane-a"]],
    ["object-storage", ["team-a", "gone"]],
  ])("is not found for the tab %j and the segments %j", async (kind, ref) => {
    await expect(ConnectionPage(params(kind, ref))).rejects.toThrow(
      "NEXT_NOT_FOUND",
    );
  });
});
