// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  applyEnvironmentChange,
  applyNamespaceLabel,
  previewEnvironmentChange,
  previewNamespaceLabel,
} from "@/app/setup/environments/actions";
import { writeMode } from "@/lib/write/writer";
import NewEnvironmentPage from "./page";

vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async () => []),
}));
vi.mock("@/lib/write/writer", () => ({
  writeMode: vi.fn(async () => "direct"),
}));
vi.mock("@/app/setup/environments/actions", () => ({
  previewEnvironmentChange: vi.fn(),
  applyEnvironmentChange: vi.fn(),
  previewNamespaceLabel: vi.fn(),
  applyNamespaceLabel: vi.fn(),
}));
const editor = vi.hoisted(() => ({ actions: undefined as unknown }));
vi.mock("@/components/setup/environments/environment-editor", () => ({
  EnvironmentEditorLive: (props: { mode?: string; actions: unknown }) => {
    editor.actions = props.actions;
    return <p>editor {props.mode ?? "unknown"}</p>;
  },
}));

afterEach(cleanup);

describe("NewEnvironmentPage", () => {
  it("passes the write mode of the settings", async () => {
    vi.mocked(writeMode).mockResolvedValueOnce("git");
    render(await NewEnvironmentPage());
    expect(screen.getByText("editor git")).toBeTruthy();
  });

  it("passes no write mode when the settings cannot be read", async () => {
    vi.mocked(writeMode).mockRejectedValueOnce(new Error("forbidden"));
    render(await NewEnvironmentPage());
    expect(screen.getByText("editor unknown")).toBeTruthy();
  });

  it("gives the editor the server actions of the Environments", async () => {
    render(await NewEnvironmentPage());
    expect(editor.actions).toEqual({
      previewChange: previewEnvironmentChange,
      applyChange: applyEnvironmentChange,
      previewLabel: previewNamespaceLabel,
      applyLabel: applyNamespaceLabel,
    });
  });
});
