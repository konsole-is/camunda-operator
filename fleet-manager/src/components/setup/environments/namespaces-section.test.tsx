// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { WriteMode } from "@/lib/write/types";
import { EnvironmentEditor } from "./environment-editor";
import { environmentFixtures } from "./fixtures";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
// The editor gets its writes as props. The page module of the server
// actions loads the writer, which a component test does not need.
vi.mock("@/app/setup/environments/actions", () => ({
  previewEnvironmentChange: vi.fn(),
  applyEnvironmentChange: vi.fn(),
  previewNamespaceLabel: vi.fn(),
  applyNamespaceLabel: vi.fn(),
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

function renderEditor(name: string | undefined, mode: WriteMode = "direct") {
  const actions = {
    previewChange: vi.fn(async () => ({ objects: [], errors: [] })),
    applyChange: vi.fn(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
    previewLabel: vi.fn(async () => ({ objects: [], errors: [] })),
    applyLabel: vi.fn(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
  };
  render(
    <GitPushesProvider>
      <EnvironmentEditor
        sources={{ ...environmentFixtures(), errors: [] }}
        name={name}
        mode={mode}
        actions={actions}
      />
    </GitPushesProvider>,
  );
  return actions;
}

function addNamespace(name: string) {
  fireEvent.change(screen.getByLabelText("Add a namespace"), {
    target: { value: name },
  });
  fireEvent.click(screen.getByRole("button", { name: "Add" }));
}

afterEach(cleanup);

describe("NamespacesSection", () => {
  it("lists the namespaces of the environment with their claims", () => {
    renderEditor("production");
    const list = screen.getByRole("list", {
      name: "Namespaces of the environment",
    });
    expect(list.textContent).toContain("team-a2 clusters");
    expect(list.textContent).toContain("team-b0 clusters");
  });

  it("offers only namespaces without an environment label", () => {
    renderEditor("production");
    const select = screen.getByLabelText(
      "Add a namespace",
    ) as HTMLSelectElement;
    expect(Array.from(select.options).map((o) => o.value)).toEqual([
      "",
      "legacy",
      "sandbox",
    ]);
  });

  it("Add a namespace previews and writes the label change with directOnly", async () => {
    const { previewLabel, applyLabel } = renderEditor("production");
    addNamespace("sandbox");
    const confirm = screen.getByRole("button", { name: "Add the namespace" });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    fireEvent.click(confirm);
    await waitFor(() => expect(applyLabel).toHaveBeenCalledTimes(1));
    const expected = {
      intent: "Add the namespace sandbox to the environment production",
      apply: [
        {
          apiVersion: "v1",
          kind: "Namespace",
          metadata: {
            name: "sandbox",
            labels: { "fleet.camunda.io/environment": "production" },
          },
        },
      ],
      preconditions: { "Namespace//sandbox": "7" },
      directOnly: true,
    };
    expect(previewLabel).toHaveBeenCalledWith(expected);
    expect(applyLabel).toHaveBeenCalledWith(expected);
    await waitFor(() =>
      expect(screen.getByRole("status").textContent).toContain(
        "The namespace sandbox is in the environment production.",
      ),
    );
  });

  it("in Git mode, says that the label goes straight to the cluster, and the button does not read Push to Git", async () => {
    renderEditor("production", "git");
    addNamespace("legacy");
    expect(
      screen.getByText(
        "The label goes straight to the cluster, also in Git mode. It is not in the Git repository.",
      ),
    ).toBeTruthy();
    expect(
      screen.getByRole("button", { name: "Add the namespace" }),
    ).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Push to Git" })).toBeNull();
  });

  it("asks to save a new environment before it adds namespaces", () => {
    renderEditor(undefined);
    expect(
      screen.getByText("Save the environment, then add namespaces."),
    ).toBeTruthy();
    expect(screen.queryByLabelText("Add a namespace")).toBeNull();
  });
});
