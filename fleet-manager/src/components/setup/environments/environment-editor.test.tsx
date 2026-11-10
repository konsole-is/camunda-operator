// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { WriteMode, WriteResult } from "@/lib/write/types";
import {
  EnvironmentEditor,
  type EnvironmentActions,
} from "./environment-editor";
import { environmentFixtures } from "./fixtures";
import type { EnvironmentSources } from "./sources";

const router = vi.hoisted(() => ({ push: vi.fn() }));
const sync = vi.hoisted(() => ({ state: "waiting" }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));
// The editor gets its writes as props. The page module of the server
// actions loads the writer, which a component test does not need.
vi.mock("@/app/setup/environments/actions", () => ({
  previewEnvironmentChange: vi.fn(),
  applyEnvironmentChange: vi.fn(),
  previewNamespaceLabel: vi.fn(),
  applyNamespaceLabel: vi.fn(),
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => sync.state,
  useGitGone: () => sync.state,
}));

const PUSHED: WriteResult = {
  ok: true,
  mode: "git",
  commit: "4f1c3a7e9b2d8c6f0a5e1b3d4f1c3a7e9b2d8c6f",
  revisions: { "Environment//production": "abc" },
  paths: { "Environment//production": "environment/production.yaml" },
  deleted: {},
};

function setup({
  name = "production",
  mode = "direct",
  sources = environmentFixtures(),
  result = { ok: true, mode: "direct" },
}: {
  name?: string;
  mode?: WriteMode;
  sources?: EnvironmentSources;
  result?: WriteResult;
} = {}) {
  const actions = {
    previewChange: vi.fn<EnvironmentActions["previewChange"]>(async () => ({
      objects: [],
      errors: [],
    })),
    applyChange: vi.fn<EnvironmentActions["applyChange"]>(async () => result),
    previewLabel: vi.fn(async () => ({ objects: [], errors: [] })),
    applyLabel: vi.fn(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
  };
  const view = (s: EnvironmentSources) => (
    <GitPushesProvider>
      <EnvironmentEditor
        sources={{ ...s, errors: [] }}
        name={name === "" ? undefined : name}
        mode={mode}
        actions={actions}
      />
    </GitPushesProvider>
  );
  const { rerender } = render(view(sources));
  return { actions, rerender: (s: EnvironmentSources) => rerender(view(s)) };
}

const displayName = () =>
  screen.getByLabelText("Display name") as HTMLInputElement;

async function saveDisplayName(value: string, confirm = "Save") {
  fireEvent.change(displayName(), { target: { value } });
  fireEvent.click(screen.getByRole("button", { name: "Review" }));
  const button = screen.getByRole("button", { name: confirm });
  await waitFor(() => expect(button).toHaveProperty("disabled", false));
  fireEvent.click(button);
}

beforeEach(() => {
  router.push.mockClear();
  sync.state = "waiting";
});
afterEach(cleanup);

describe("EnvironmentEditor", () => {
  it("shows the message of Ready=False", () => {
    const sources = environmentFixtures();
    sources.environments[0].status = {
      conditions: [
        {
          type: "Ready",
          status: "False",
          reason: "InvalidReference",
          message:
            "The spec names objects that do not exist: CamundaRelease 8.9.9",
          lastTransitionTime: "2026-11-10T10:00:00Z",
        },
      ],
    };
    setup({ sources });
    expect(screen.getByRole("alert").textContent).toBe(
      "The spec names objects that do not exist: CamundaRelease 8.9.9",
    );
    expect(document.body.textContent).toContain("Not ready");
  });

  it("offers Delete only while no namespace carries the label", () => {
    setup();
    expect(
      screen.getByText(
        "Delete is possible when no namespace carries the label of this environment.",
      ),
    ).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "Delete the environment" }),
    ).toBeNull();
    cleanup();
    const sources = environmentFixtures();
    sources.namespaces = sources.namespaces.filter(
      (n) => n.metadata?.name !== "team-c",
    );
    setup({ name: "staging", sources });
    expect(
      screen.getByRole("button", { name: "Delete the environment" }),
    ).toBeTruthy();
  });

  it("a confirmed delete writes the delete change", async () => {
    const sources = environmentFixtures();
    sources.namespaces = [];
    const { actions } = setup({ sources });
    fireEvent.change(screen.getByLabelText(/to confirm/), {
      target: { value: "production" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Delete the environment" }),
    );
    await waitFor(() =>
      expect(router.push).toHaveBeenCalledWith("/setup/environments"),
    );
    expect(actions.applyChange).toHaveBeenCalledWith({
      intent: "Delete the environment production",
      apply: [],
      delete: [
        {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "Environment",
          name: "production",
        },
      ],
      preconditions: { "Environment//production": "12" },
    });
  });

  it("saves an edit with the resourceVersion that the form read and force", async () => {
    const { actions } = setup();
    await saveDisplayName("Prod");
    await waitFor(() => expect(actions.applyChange).toHaveBeenCalledTimes(1));
    expect(actions.applyChange.mock.calls[0][0]).toMatchObject({
      intent: "Change the environment production",
      preconditions: { "Environment//production": "12" },
      force: true,
      apply: [{ spec: { displayName: "Prod" } }],
    });
  });

  it("counts the changed fields and blocks Review with the first error", () => {
    setup();
    expect(
      screen.queryByRole("region", { name: "Pending changes" }),
    ).toBeNull();
    fireEvent.change(displayName(), { target: { value: "x".repeat(65) } });
    fireEvent.change(screen.getByLabelText("Color"), {
      target: { value: "#000000" },
    });
    expect(screen.getByText("2 changes")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Review" })).toHaveProperty(
      "disabled",
      true,
    );
  });

  it("Discard loads the live copy", () => {
    setup();
    fireEvent.change(displayName(), { target: { value: "Prod" } });
    fireEvent.click(screen.getByRole("button", { name: "Discard" }));
    expect(displayName().value).toBe("Production");
  });

  it("reads Push to Git in Git mode", async () => {
    setup({ mode: "git", result: PUSHED });
    fireEvent.change(displayName(), { target: { value: "Prod" } });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Push to Git" }),
      ).toHaveProperty("disabled", false),
    );
  });

  it("keeps the form disabled while a push waits for GitOps", async () => {
    const { rerender } = setup({ mode: "git", result: PUSHED });
    await saveDisplayName("Prod", "Push to Git");
    await waitFor(() =>
      expect(screen.getByText("Waiting for GitOps to apply")).toBeTruthy(),
    );
    expect(displayName().matches(":disabled")).toBe(true);
    expect(displayName().value).toBe("Prod");

    sync.state = "applied";
    const applied = environmentFixtures();
    applied.environments[0].spec.displayName = "Prod";
    applied.environments[0].metadata.resourceVersion = "13";
    rerender(applied);
    await waitFor(() => expect(displayName().matches(":disabled")).toBe(false));
    expect(displayName().value).toBe("Prod");
    expect(
      screen.queryByRole("region", { name: "Pending changes" }),
    ).toBeNull();
  });

  it("creates an environment and opens its editor once it shows in the live list", async () => {
    const { actions, rerender } = setup({ name: "" });
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "lab" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    const save = screen.getByRole("button", { name: "Save" });
    await waitFor(() => expect(save).toHaveProperty("disabled", false));
    fireEvent.click(save);
    await waitFor(() => expect(actions.applyChange).toHaveBeenCalledTimes(1));
    expect(actions.applyChange.mock.calls[0][0]).toEqual({
      intent: "Create the environment lab",
      apply: [
        {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "Environment",
          metadata: { name: "lab" },
          spec: { deletionPolicy: "Delete" },
        },
      ],
    });
    expect(router.push).not.toHaveBeenCalled();
    const sources = environmentFixtures();
    sources.environments.push({
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: { name: "lab" },
      spec: {},
    });
    rerender(sources);
    await waitFor(() =>
      expect(router.push).toHaveBeenCalledWith("/setup/environments/lab"),
    );
  });

  it("refuses the name of an environment that exists", () => {
    setup({ name: "" });
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "staging" },
    });
    expect(
      screen.getAllByText("An environment with this name exists.").length,
    ).toBeGreaterThan(0);
  });
});
