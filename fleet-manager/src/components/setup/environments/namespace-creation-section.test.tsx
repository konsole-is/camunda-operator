// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import {
  EnvironmentEditor,
  type EnvironmentActions,
} from "./environment-editor";
import { environmentFixtures } from "./fixtures";
import { NamespaceCreationSection } from "./namespace-creation-section";
import type { EnvironmentSources } from "./sources";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

function renderEditor(
  name: string,
  sources: EnvironmentSources = environmentFixtures(),
) {
  const actions = {
    previewChange: vi.fn(async () => ({ objects: [], errors: [] })),
    applyChange: vi.fn<EnvironmentActions["applyChange"]>(async () => ({
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
        sources={{ ...sources, errors: [] }}
        name={name}
        mode="direct"
        actions={actions}
      />
    </GitPushesProvider>,
  );
  return actions;
}

const fixedLabels = () =>
  screen.getByRole("group", { name: "Fixed labels of a new namespace" });

afterEach(cleanup);

describe("NamespaceCreationSection", () => {
  it("never shows the environment label in the fixed labels or the required label keys", () => {
    renderEditor("staging");
    const keys = within(fixedLabels())
      .getAllByLabelText("Key")
      .map((i) => (i as HTMLInputElement).value);
    expect(keys).toEqual(["team"]);
    expect(
      (screen.getByLabelText("Required label key 1") as HTMLInputElement).value,
    ).toBe("cost-center");
    expect(document.body.textContent).not.toContain(
      "fleet.camunda.io/environment",
    );
  });

  it("shows the error of a typed environment label", () => {
    renderEditor("staging");
    fireEvent.change(screen.getByLabelText("Required label key 1"), {
      target: { value: "fleet.camunda.io/environment" },
    });
    expect(
      screen.getAllByText(
        "The fleet manager sets fleet.camunda.io/environment itself.",
      ).length,
    ).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Review" })).toHaveProperty(
      "disabled",
      true,
    );
  });

  it("saves allowed, labels, and requiredLabelKeys", async () => {
    const { applyChange } = renderEditor("production");
    fireEvent.click(
      screen.getByLabelText("The create flow can create a namespace"),
    );
    fireEvent.click(
      within(fixedLabels()).getByRole("button", { name: "Add an entry" }),
    );
    fireEvent.change(within(fixedLabels()).getByLabelText("Key"), {
      target: { value: "team" },
    });
    fireEvent.change(within(fixedLabels()).getByLabelText("Value"), {
      target: { value: "payments" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add a key" }));
    fireEvent.change(screen.getByLabelText("Required label key 1"), {
      target: { value: "cost-center" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    const save = screen.getByRole("button", { name: "Save" });
    await waitFor(() => expect(save).toHaveProperty("disabled", false));
    fireEvent.click(save);
    await waitFor(() => expect(applyChange).toHaveBeenCalledTimes(1));
    expect(applyChange.mock.calls[0][0].apply[0].spec).toMatchObject({
      namespaceCreation: {
        allowed: true,
        labels: { team: "payments" },
        requiredLabelKeys: ["cost-center"],
      },
    });
  });

  it("describes the rows by the hint and the errors of their list", () => {
    render(
      <NamespaceCreationSection
        value={{ labels: { team: "a" }, requiredLabelKeys: ["cost-center"] }}
        errors={[
          { path: "spec.namespaceCreation.labels", message: "Too many labels" },
          {
            path: "spec.namespaceCreation.requiredLabelKeys",
            message: "Too many keys",
          },
        ]}
        onChange={() => {}}
      />,
    );
    expect(
      screen.getByRole("textbox", {
        name: "Key",
        description: "Too many labels",
      }),
    ).toBeTruthy();
    const key = screen.getByRole("textbox", {
      name: "Required label key 1",
      description:
        "The create flow asks for a value of each key. Too many keys",
    });
    expect(key.getAttribute("aria-invalid")).toBe("true");
  });
});
