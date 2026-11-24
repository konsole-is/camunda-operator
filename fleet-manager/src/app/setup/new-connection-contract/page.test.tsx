// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  applyContractChange,
  previewContractChange,
} from "@/app/setup/connections/actions";
import { writeMode } from "@/lib/write/writer";
import NewConnectionContractPage, { dynamic } from "./page";

vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async () => []),
}));
vi.mock("@/lib/write/writer", () => ({
  writeMode: vi.fn(async () => "direct"),
}));
vi.mock("@/app/setup/connections/actions", () => ({
  previewContractChange: vi.fn(),
  applyContractChange: vi.fn(),
}));
const form = vi.hoisted(() => ({ actions: undefined as unknown }));
vi.mock("@/components/setup/connections/contract-create", () => ({
  ContractCreateLive: (props: {
    tab: string;
    mode?: string;
    actions: unknown;
  }) => {
    form.actions = props.actions;
    return (
      <p>
        form {props.tab} {props.mode ?? "unknown"}
      </p>
    );
  },
}));

afterEach(cleanup);

const open = async (searchParams: Record<string, string> = {}) =>
  render(
    await NewConnectionContractPage({
      params: Promise.resolve({}),
      searchParams: Promise.resolve(searchParams),
    }),
  );

describe("NewConnectionContractPage", () => {
  it("passes the tab of the query", async () => {
    await open({ tab: "databases" });
    expect(screen.getByText("form databases direct")).toBeTruthy();
  });

  it.each([[{}], [{ tab: "buckets" }]])(
    "opens object storage for the query %j",
    async (query) => {
      await open(query);
      expect(screen.getByText("form object-storage direct")).toBeTruthy();
    },
  );

  it("passes the write mode of the settings", async () => {
    vi.mocked(writeMode).mockResolvedValueOnce("git");
    await open();
    expect(screen.getByText("form object-storage git")).toBeTruthy();
  });

  it("passes no write mode when the settings cannot be read", async () => {
    vi.mocked(writeMode).mockRejectedValueOnce(new Error("forbidden"));
    await open();
    expect(screen.getByText("form object-storage unknown")).toBeTruthy();
  });

  it("gives the form the server actions of the contracts", async () => {
    await open();
    expect(form.actions).toEqual({
      previewChange: previewContractChange,
      applyChange: applyContractChange,
    });
  });

  it("renders on each request", () => {
    expect(dynamic).toBe("force-dynamic");
  });
});
