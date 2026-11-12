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
import type { Environment } from "@/generated/crds/environment";
import {
  EnvironmentEditor,
  type EnvironmentActions,
} from "./environment-editor";
import { sectionFixtures } from "./fixtures";
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

function renderEditor(name: string, sources: EnvironmentSources) {
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

// group returns the settings of a parameter in the settings of an option.
const group = (option: string, parameter: string) =>
  within(screen.getByRole("region", { name: option })).getByRole("group", {
    name: parameter,
  });

async function save(actions: ReturnType<typeof renderEditor>) {
  fireEvent.click(screen.getByRole("button", { name: "Review" }));
  const confirm = screen.getByRole("button", { name: "Save" });
  await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
  fireEvent.click(confirm);
  await waitFor(() => expect(actions.applyChange).toHaveBeenCalledTimes(1));
  return actions.applyChange.mock.calls[0][0].apply[0]
    .spec as Environment["spec"];
}

afterEach(cleanup);

function withRustfsSchema() {
  const sources = sectionFixtures();
  const rustfs = sources.registrations.find(
    (r) => r.metadata.name === "rustfs",
  )!;
  rustfs.spec.parameters = {
    type: "object",
    properties: {
      replicas: { type: "integer", minimum: 1, maximum: 9, title: "Replicas" },
      tier: { type: "string", enum: ["gold", "silver"], title: "Tier" },
    },
  };
  rustfs.spec.parameterValues = { replicas: 3 };
  return sources;
}

function withElasticsearch(listed: string[]) {
  const sources = sectionFixtures();
  sources.sections.push({
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name: "secondary-storage" },
    spec: {
      label: "Secondary storage",
      appliesTo: ["CamundaClusterClaim"],
      selection: "One",
    },
  });
  sources.registrations.push({
    ...sources.registrations[0],
    metadata: { name: "elasticsearch" },
    spec: {
      ...sources.registrations[0].spec,
      label: "Elasticsearch in the cluster",
      sections: ["secondary-storage"],
      controller: { name: "camunda-operator" },
      parameters: {
        type: "object",
        properties: {
          preset: { type: "string", title: "Preset" },
          releaseRef: { type: "string" },
        },
      },
    },
  } as (typeof sources.registrations)[number]);
  sources.elasticsearchPresets = ["es-small", "es-large"].map(
    (name) =>
      ({
        apiVersion: "core.camunda.io/v1",
        kind: "ElasticsearchClusterPreset",
        metadata: { name },
        spec: { cluster: {} },
      }) as unknown as (typeof sources.elasticsearchPresets)[number],
  );
  const spec = sources.environments[0].spec;
  spec.sections = {
    ...spec.sections,
    "secondary-storage": { allowed: ["elasticsearch"] },
  };
  spec.presets = { ...spec.presets, elasticsearchCluster: listed };
  return sources;
}

describe("OptionSettings", () => {
  it("shows a number parameter with its range and an enum parameter as a select", () => {
    renderEditor("production", withRustfsSchema());
    const replicas = within(group("RustFS", "Replicas")).getByLabelText(
      "Replicas",
    );
    expect(replicas).toHaveProperty("type", "number");
    expect(
      within(group("RustFS", "Replicas")).getByText("From 1 to 9"),
    ).toBeTruthy();
    expect(within(group("RustFS", "Tier")).getByLabelText("Tier").tagName).toBe(
      "SELECT",
    );
  });

  it("shows the value of the registration while the environment value is empty", () => {
    renderEditor("production", withRustfsSchema());
    expect(
      within(group("RustFS", "Replicas")).getByText("From the registration: 3"),
    ).toBeTruthy();
    fireEvent.change(
      within(group("RustFS", "Replicas")).getByLabelText("Replicas"),
      { target: { value: "5" } },
    );
    expect(
      within(group("RustFS", "Replicas")).queryByText(
        "From the registration: 3",
      ),
    ).toBeNull();
  });

  it("saves the environment value and A claim can change it", async () => {
    const actions = renderEditor("production", withRustfsSchema());
    const replicas = group("RustFS", "Replicas");
    fireEvent.change(within(replicas).getByLabelText("Replicas"), {
      target: { value: "5" },
    });
    fireEvent.click(
      within(replicas).getByRole("checkbox", { name: "A claim can change it" }),
    );
    expect(await save(actions)).toMatchObject({
      parameters: { rustfs: { replicas: { value: 5, claimEditable: true } } },
    });
  });

  it("refuses a value that the schema refuses before the API does", () => {
    const { previewChange } = renderEditor("production", withRustfsSchema());
    fireEvent.change(
      within(group("RustFS", "Replicas")).getByLabelText("Replicas"),
      { target: { value: "12" } },
    );
    expect(
      within(group("RustFS", "Replicas"))
        .getByLabelText("Replicas")
        .getAttribute("aria-invalid"),
    ).toBe("true");
    const review = screen.getByRole("button", { name: "Review" });
    expect(review).toHaveProperty("disabled", true);
    fireEvent.click(review);
    expect(previewChange).not.toHaveBeenCalled();
  });

  it("shows the limits by label with (limit set at install)", () => {
    renderEditor("production", sectionFixtures());
    const role = screen.getByRole("region", { name: "Role for each cluster" });
    expect(
      within(role).getByText("Permissions boundary (limit set at install)"),
    ).toBeTruthy();
    expect(
      within(role).getByText(
        "arn:aws:iam::123456789012:policy/camunda-boundary",
      ),
    ).toBeTruthy();
  });

  it("shows the hostname pattern with a rendered host", () => {
    const sources = sectionFixtures();
    sources.environments[0].spec.sections!.access = {
      allowed: ["gateway-public"],
    };
    renderEditor("production", sources);
    expect(
      within(group("Public gateway", "Hostname pattern")).getByText(
        "billing in team-a gets billing.team-a.example.com",
      ),
    ).toBeTruthy();
  });

  it("offers the presets that the environment allows for a preset parameter", () => {
    renderEditor("production", withElasticsearch(["es-small"]));
    const preset = within(
      group("Elasticsearch in the cluster", "Preset"),
    ).getByLabelText("Preset") as HTMLSelectElement;
    expect(Array.from(preset.options).map((o) => o.text)).toEqual([
      "Choose one",
      "es-small",
    ]);
  });

  it("says that an empty preset list allows no preset", () => {
    renderEditor("production", withElasticsearch([]));
    expect(
      within(group("Elasticsearch in the cluster", "Preset")).getByText(
        "No Elasticsearch preset is allowed. A claim that picks this option is refused until you check one under Presets.",
      ),
    ).toBeTruthy();
  });

  it("hides the parameters that the built-in table hides", () => {
    renderEditor("production", withElasticsearch(["es-small"]));
    expect(
      within(
        screen.getByRole("region", { name: "Elasticsearch in the cluster" }),
      ).queryByRole("group", { name: "releaseRef" }),
    ).toBeNull();
  });

  it("shows a template parameter with its editor and the preview", () => {
    renderEditor("production", sectionFixtures());
    const inline = group("Role for each cluster", "inlinePolicy");
    expect(
      within(inline).getByRole("button", { name: "{{ .serviceAccount }}" }),
    ).toBeTruthy();
    expect(inline.querySelector("pre")?.textContent).toContain(
      "Resource: arn:aws:sqs:*:*:team-a-*",
    );
  });

  it("blocks Review while a template has an unknown placeholder", () => {
    const { previewChange } = renderEditor("production", sectionFixtures());
    fireEvent.change(
      within(group("Role for each cluster", "inlinePolicy")).getByLabelText(
        "inlinePolicy",
      ),
      { target: { value: "Resource: '{{ .claimName }}'\n" } },
    );
    const review = screen.getByRole("button", { name: "Review" });
    expect(review).toHaveProperty("disabled", true);
    fireEvent.click(review);
    expect(previewChange).not.toHaveBeenCalled();
  });
});
