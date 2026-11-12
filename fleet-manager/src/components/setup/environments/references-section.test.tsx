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
import {
  EnvironmentEditor,
  type EnvironmentActions,
} from "./environment-editor";
import { environmentFixtures } from "./fixtures";
import type { EnvironmentSources } from "./sources";

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

function renderEditor(
  name: string,
  sources: EnvironmentSources = environmentFixtures(),
) {
  const actions = {
    previewChange: vi.fn<EnvironmentActions["previewChange"]>(async () => ({
      objects: [],
      errors: [],
    })),
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

const checkbox = (name: string) =>
  screen.getByRole("checkbox", {
    name: new RegExp(`^${name.replace(/\./g, "\\.")}`),
  });

afterEach(cleanup);

describe("ReferencesSection", () => {
  it("refuses a default release that is not allowed before the API does", () => {
    const { previewChange } = renderEditor("production");
    fireEvent.click(checkbox("8.9.23"));
    expect(
      screen.getByLabelText("Default release").getAttribute("aria-invalid"),
    ).toBe("true");
    expect(
      screen.getAllByText(
        "The default release must be one of the allowed releases.",
      ),
    ).toHaveLength(2);
    const review = screen.getByRole("button", { name: "Review" });
    expect(review).toHaveProperty("disabled", true);
    fireEvent.click(review);
    expect(previewChange).not.toHaveBeenCalled();
  });

  it("shows a listed release that does not exist as Not found and lets the admin clear it", async () => {
    const sources = environmentFixtures();
    sources.environments[0].spec.releases = {
      allowed: ["8.9.23", "8.9.9"],
      default: "8.9.23",
    };
    const { applyChange } = renderEditor("production", sources);
    const missing = checkbox("8.9.9");
    expect(missing).toHaveProperty("checked", true);
    expect(missing.closest("label")?.textContent).toContain("Not found");
    fireEvent.click(missing);
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    const save = screen.getByRole("button", { name: "Save" });
    await waitFor(() => expect(save).toHaveProperty("disabled", false));
    fireEvent.click(save);
    await waitFor(() => expect(applyChange).toHaveBeenCalledTimes(1));
    expect(applyChange.mock.calls[0][0].apply[0].spec).toMatchObject({
      releases: { allowed: ["8.9.23"], default: "8.9.23" },
    });
  });

  it("offers the platform configs with None, and saves the choice", async () => {
    const { applyChange } = renderEditor("staging");
    const select = screen.getByLabelText(
      "Platform config",
    ) as HTMLSelectElement;
    expect(Array.from(select.options).map((o) => o.text)).toEqual([
      "None",
      "shared",
    ]);
    fireEvent.change(select, { target: { value: "shared" } });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    const save = screen.getByRole("button", { name: "Save" });
    await waitFor(() => expect(save).toHaveProperty("disabled", false));
    fireEvent.click(save);
    await waitFor(() => expect(applyChange).toHaveBeenCalledTimes(1));
    expect(applyChange.mock.calls[0][0].apply[0].spec).toMatchObject({
      platformConfigRef: "shared",
    });
  });

  it("shows the error of too many allowed releases beside the list", () => {
    const sources = environmentFixtures();
    sources.environments[0].spec.releases = {
      allowed: Array.from({ length: 65 }, (_, i) => `r-${i}`),
    };
    renderEditor("production", sources);
    const list = screen.getByRole("group", { name: "Allowed releases" });
    expect(list.textContent).toContain("At most 64 allowed releases.");
  });

  it("says that an empty list allows no Elasticsearch and no PostgreSQL preset", () => {
    renderEditor("production");
    expect(
      screen.getByText(
        "No Elasticsearch preset is allowed, so the built-in Elasticsearch of a claim can use none.",
      ),
    ).toBeTruthy();
    expect(
      screen.getByText(
        "No PostgreSQL preset is allowed, so the built-in PostgreSQL of a claim can use none.",
      ),
    ).toBeTruthy();
  });

  it("says that an empty list allows no release and no cluster preset", () => {
    const sources = environmentFixtures();
    sources.environments[0].spec.releases = undefined;
    sources.environments[0].spec.presets = undefined;
    renderEditor("production", sources);
    expect(
      screen.getByText("No release is allowed, so a claim can use none."),
    ).toBeTruthy();
    expect(
      screen.getByText(
        "No cluster preset is allowed, so a cluster claim can use none.",
      ),
    ).toBeTruthy();
  });
});
