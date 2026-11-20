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
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import {
  EnvironmentEditor,
  type EnvironmentActions,
} from "./environment-editor";
import { sectionFixtures } from "./fixtures";
import type { EnvironmentSources } from "./sources";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
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

const NEEDS_VALUE =
  "This parameter needs a value. A new claim that picks this option is refused until you give it a value or let a claim change it.";

// withRequiredBucket gives RustFS a required bucket without a value and an
// optional region.
function withRequiredBucket(values?: Record<string, unknown>) {
  const sources = sectionFixtures();
  const rustfs = sources.registrations.find(
    (r) => r.metadata.name === "rustfs",
  )!;
  rustfs.spec.parameters = {
    type: "object",
    required: ["bucket"],
    properties: {
      bucket: { type: "string", minLength: 1, title: "Bucket" },
      region: { type: "string", title: "Region" },
    },
  };
  if (values) rustfs.spec.parameterValues = values;
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

  describe("a required parameter without a value", () => {
    it("warns at the parameter", () => {
      renderEditor("production", withRequiredBucket());
      expect(
        within(group("RustFS", "Bucket")).getByText(NEEDS_VALUE),
      ).toBeTruthy();
      expect(
        within(group("RustFS", "Region")).queryByText(NEEDS_VALUE),
      ).toBeNull();
    });

    it("stops warning once the environment gives a value", () => {
      renderEditor("production", withRequiredBucket());
      fireEvent.change(
        within(group("RustFS", "Bucket")).getByLabelText("Bucket"),
        { target: { value: "backups" } },
      );
      expect(
        within(group("RustFS", "Bucket")).queryByText(NEEDS_VALUE),
      ).toBeNull();
    });

    it("does not warn when the registration gives a value", () => {
      renderEditor("production", withRequiredBucket({ bucket: "backups" }));
      expect(
        within(group("RustFS", "Bucket")).queryByText(NEEDS_VALUE),
      ).toBeNull();
    });

    it("stops warning once a claim can change it", () => {
      renderEditor("production", withRequiredBucket());
      fireEvent.click(
        within(group("RustFS", "Bucket")).getByRole("checkbox", {
          name: "A claim can change it",
        }),
      );
      expect(
        within(group("RustFS", "Bucket")).queryByText(NEEDS_VALUE),
      ).toBeNull();
    });

    it("does not block Review", () => {
      renderEditor("production", withRequiredBucket());
      fireEvent.change(
        within(group("RustFS", "Region")).getByLabelText("Region"),
        { target: { value: "eu" } },
      );
      expect(
        within(group("RustFS", "Bucket")).getByText(NEEDS_VALUE),
      ).toBeTruthy();
      expect(screen.getByRole("button", { name: "Review" })).toHaveProperty(
        "disabled",
        false,
      );
    });

    it("names the claim kind and the section when only some claims get no value", () => {
      const sources = sectionFixtures();
      const rustfs = sources.registrations.find(
        (r) => r.metadata.name === "rustfs",
      )!;
      rustfs.spec.parameters = {
        type: "object",
        required: ["releaseRef"],
        properties: { releaseRef: { type: "string", title: "Release" } },
      };
      sources.environments[0].spec.sections!.access = { allowed: ["rustfs"] };
      renderEditor("production", sources);
      expect(
        within(group("RustFS", "Release")).getByText(
          "This parameter needs a value for a management plane claim in Access. Such a claim is refused until you give it a value or let a claim change it.",
        ),
      ).toBeTruthy();
    });

    it("does not warn for an option that no section allows", () => {
      const sources = withRequiredBucket();
      const spec = sources.environments[0].spec;
      delete spec.sections!["backup-storage"];
      spec.parameters = {
        ...spec.parameters,
        rustfs: { region: { value: "eu" } },
      };
      renderEditor("production", sources);
      expect(screen.getByRole("region", { name: "RustFS" })).toBeTruthy();
      expect(
        within(group("RustFS", "Bucket")).queryByText(NEEDS_VALUE),
      ).toBeNull();
    });
  });

  describe("a map of strings", () => {
    function withLabels() {
      const sources = withRustfsSchema();
      const rustfs = sources.registrations.find(
        (r) => r.metadata.name === "rustfs",
      )!;
      rustfs.spec.parameters = {
        type: "object",
        properties: {
          labels: {
            type: "object",
            additionalProperties: { type: "string" },
            title: "Labels",
          },
        },
      };
      return sources;
    }
    // The rows of the map are a group of the same name inside the settings.
    const labelsGroup = () =>
      within(screen.getByRole("region", { name: "RustFS" })).getAllByRole(
        "group",
        { name: "Labels" },
      )[0];

    it("saves the rows of key and value as the environment value", async () => {
      const actions = renderEditor("production", withLabels());
      const labels = labelsGroup();
      fireEvent.click(
        within(labels).getByRole("button", { name: "Add an entry" }),
      );
      fireEvent.change(within(labels).getByRole("textbox", { name: "Key" }), {
        target: { value: "team" },
      });
      fireEvent.change(within(labels).getByRole("textbox", { name: "Value" }), {
        target: { value: "orders" },
      });
      expect(await save(actions)).toMatchObject({
        parameters: { rustfs: { labels: { value: { team: "orders" } } } },
      });
    });

    it("saves {} over a value of the registration", async () => {
      const sources = withLabels();
      sources.registrations.find(
        (r) => r.metadata.name === "rustfs",
      )!.spec.parameterValues = { replicas: 3, labels: { tier: "gold" } };
      const actions = renderEditor("production", sources);
      fireEvent.click(
        within(labelsGroup()).getByRole("button", {
          name: "Set to an empty map",
        }),
      );
      expect(
        within(labelsGroup()).getByText(
          "Empty map, overrides the inherited value",
        ),
      ).toBeTruthy();
      expect(await save(actions)).toMatchObject({
        parameters: { rustfs: { labels: { value: {} } } },
      });
    });

    it("offers no empty map without a value of the registration", () => {
      renderEditor("production", withLabels());
      expect(
        within(labelsGroup()).queryByRole("button", {
          name: "Set to an empty map",
        }),
      ).toBeNull();
    });

    it("lets an allowed choice hold {}", () => {
      const sources = withLabels();
      const environment = sources.environments.find(
        (e) => e.metadata.name === "production",
      )!;
      environment.spec.parameters = {
        rustfs: {
          labels: {
            claimEditable: true,
            allowed: [{ name: "none", label: "No labels", value: {} }],
          },
        },
      };
      renderEditor("production", sources);
      expect(
        within(screen.getByRole("group", { name: "Choice 1" })).getByText(
          "Empty map",
        ),
      ).toBeTruthy();
    });

    it("blocks Review while a key is set twice", () => {
      const { previewChange } = renderEditor("production", withLabels());
      const labels = labelsGroup();
      const add = within(labels).getByRole("button", { name: "Add an entry" });
      fireEvent.click(add);
      fireEvent.click(add);
      for (const key of within(labels).getAllByRole("textbox", {
        name: "Key",
      }))
        fireEvent.change(key, { target: { value: "team" } });
      const review = screen.getByRole("button", { name: "Review" });
      expect(review).toHaveProperty("disabled", true);
      fireEvent.click(review);
      expect(previewChange).not.toHaveBeenCalled();
    });
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
        "No Elasticsearch preset is allowed. A new claim that picks this option is refused until you check one under Presets.",
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

  it("keeps the empty member of a string enum as the environment value", async () => {
    const sources = withRustfsSchema();
    const rustfs = sources.registrations.find(
      (r) => r.metadata.name === "rustfs",
    )!;
    rustfs.spec.parameters = {
      type: "object",
      properties: {
        tier: { type: "string", enum: ["", "gold"], title: "Tier" },
      },
    };
    const actions = renderEditor("production", sources);
    fireEvent.change(within(group("RustFS", "Tier")).getByLabelText("Tier"), {
      target: { value: "0" },
    });
    expect(await save(actions)).toMatchObject({
      parameters: { rustfs: { tier: { value: "" } } },
    });
  });

  it("drops the error of a removed option, so Review opens again", () => {
    const sources = sectionFixtures();
    sources.environments[0].spec.parameters = undefined;
    renderEditor("production", sources);
    fireEvent.change(screen.getByLabelText("Display name"), {
      target: { value: "Prod" },
    });
    fireEvent.change(
      within(group("Role for each cluster", "inlinePolicy")).getByLabelText(
        "inlinePolicy",
      ),
      { target: { value: "a: [" } },
    );
    expect(screen.getByRole("button", { name: "Review" })).toHaveProperty(
      "disabled",
      true,
    );
    const permissions = within(
      screen.getByRole("region", { name: "Clusters" }),
    ).getByRole("group", { name: "Connectors permissions" });
    fireEvent.change(within(permissions).getByLabelText("Default option"), {
      target: { value: "" },
    });
    fireEvent.click(
      within(permissions).getByRole("checkbox", {
        name: "Role for each cluster",
      }),
    );
    expect(
      screen.queryByRole("region", { name: "Role for each cluster" }),
    ).toBeNull();
    expect(screen.getByRole("button", { name: "Review" })).toHaveProperty(
      "disabled",
      false,
    );
  });

  it("edits an array template parameter as YAML and saves the list", async () => {
    const sources = sectionFixtures();
    const role = sources.registrations.find(
      (r) => r.metadata.name === "aws-connectors-role",
    )!;
    role.spec.parameters = {
      type: "object",
      properties: {
        resources: {
          type: "array",
          items: { type: "string" },
          "x-fleet-template": true,
        },
      },
    };
    const actions = renderEditor("production", sources);
    fireEvent.change(
      within(group("Role for each cluster", "resources")).getByLabelText(
        "resources",
      ),
      { target: { value: "- arn:aws:sqs:*:*:{{ .namespace }}-*\n" } },
    );
    expect(await save(actions)).toMatchObject({
      parameters: {
        "aws-connectors-role": {
          resources: { value: ["arn:aws:sqs:*:*:{{ .namespace }}-*"] },
        },
      },
    });
  });

  it("shows a preset value that the environment no longer allows, and blocks Review", () => {
    const sources = withElasticsearch(["es-small", "es-large"]);
    sources.environments[0].spec.parameters = {
      elasticsearch: { preset: { value: "es-small" } },
    };
    const { previewChange } = renderEditor("production", sources);
    fireEvent.click(
      within(
        screen.getByRole("group", { name: "Elasticsearch presets" }),
      ).getByRole("checkbox", { name: "es-small" }),
    );
    const preset = within(
      group("Elasticsearch in the cluster", "Preset"),
    ).getByLabelText("Preset") as HTMLSelectElement;
    expect(preset.selectedOptions[0].text).toBe("es-small (not allowed)");
    expect(preset.getAttribute("aria-invalid")).toBe("true");
    const review = screen.getByRole("button", { name: "Review" });
    expect(review).toHaveProperty("disabled", true);
    fireEvent.click(review);
    expect(previewChange).not.toHaveBeenCalled();
  });

  it("resets a boolean to the value of the registration", async () => {
    const sources = withRustfsSchema();
    const rustfs = sources.registrations.find(
      (r) => r.metadata.name === "rustfs",
    )!;
    rustfs.spec.parameters = {
      type: "object",
      properties: { tls: { type: "boolean", title: "TLS" } },
    };
    rustfs.spec.parameterValues = { tls: true };
    sources.environments[0].spec.parameters = {
      rustfs: { tls: { value: false, claimEditable: true } },
    };
    const actions = renderEditor("production", sources);
    const tls = group("RustFS", "TLS");
    fireEvent.click(
      within(tls).getByRole("button", {
        name: "Use the value of the registration",
      }),
    );
    expect(within(tls).getByText("From the registration: true")).toBeTruthy();
    const spec = await save(actions);
    expect(spec.parameters?.rustfs?.tls).toEqual({ claimEditable: true });
  });

  it("shows the schema error of a template parameter at its field", () => {
    const sources = sectionFixtures();
    const role = sources.registrations.find(
      (r) => r.metadata.name === "aws-connectors-role",
    )!;
    role.spec.parameters = {
      type: "object",
      properties: {
        inlinePolicy: {
          type: "object",
          required: ["Statement"],
          "x-fleet-template": true,
        },
      },
    };
    sources.environments[0].spec.parameters = {
      "aws-connectors-role": { inlinePolicy: { value: { Version: "x" } } },
    };
    renderEditor("production", sources);
    const field = within(
      group("Role for each cluster", "inlinePolicy"),
    ).getByLabelText("inlinePolicy");
    expect(field.getAttribute("aria-invalid")).toBe("true");
  });

  it("keeps the settings of an option that no section allows while the environment has a policy for it", () => {
    renderEditor("production", sectionFixtures());
    const permissions = within(
      screen.getByRole("region", { name: "Clusters" }),
    ).getByRole("group", { name: "Connectors permissions" });
    fireEvent.change(within(permissions).getByLabelText("Default option"), {
      target: { value: "" },
    });
    fireEvent.click(
      within(permissions).getByRole("checkbox", {
        name: "Role for each cluster",
      }),
    );
    const role = screen.getByRole("region", { name: "Role for each cluster" });
    expect(
      within(role).getByText(
        "No section allows this option. Clear its settings to remove them from the environment.",
      ),
    ).toBeTruthy();
  });

  it("offers a chip only for a parameter that a template can read", () => {
    renderEditor("production", sectionFixtures());
    const chips = within(group("Role for each cluster", "inlinePolicy"))
      .getAllByRole("button")
      .map((b) => b.textContent);
    expect(chips).not.toContain("{{ .parameters.managedPolicyArns }}");
    expect(chips).not.toContain("{{ .parameters.extraPolicyArns }}");
  });

  it("keeps the last value of a parameter while its YAML does not parse", () => {
    const sources = sectionFixtures();
    sources.environments[0].spec.sections!["connectors-permissions"] = {};
    sources.environments[0].spec.parameters = {
      "aws-connectors-role": {
        inlinePolicy: { value: { Version: "2012-10-17" } },
      },
    };
    renderEditor("production", sources);
    fireEvent.change(
      within(group("Role for each cluster", "inlinePolicy")).getByLabelText(
        "inlinePolicy",
      ),
      { target: { value: "a: [" } },
    );
    expect(
      screen.getByRole("region", { name: "Role for each cluster" }),
    ).toBeTruthy();
    expect(
      within(group("Role for each cluster", "inlinePolicy"))
        .getByLabelText("inlinePolicy")
        .getAttribute("aria-invalid"),
    ).toBe("true");
  });

  it("names the settings of an option that does not exist and removes them", async () => {
    const sources = sectionFixtures();
    sources.environments[0].spec.parameters = {
      ...sources.environments[0].spec.parameters,
      gone: { region: { value: "eu-west-1", claimEditable: true } },
    };
    const actions = renderEditor("production", sources);
    const note = screen.getByRole("group", {
      name: "Settings of options that do not exist",
    });
    fireEvent.click(within(note).getByRole("button", { name: "Remove gone" }));
    const spec = await save(actions);
    expect(Object.keys(spec.parameters ?? {})).toEqual(["aws-connectors-role"]);
  });

  describe("the preview of a template parameter that reads the release", () => {
    // withReleaseTemplate gives registration the template parameter
    // roleName, with the value {{ .claim }}-{{ .parameters.releaseRef }} in
    // the environment production, and a parameter releaseRef when declared.
    function withReleaseTemplate(registration: string, declared: boolean) {
      const sources = sectionFixtures();
      const target = sources.registrations.find(
        (r) => r.metadata.name === registration,
      )!;
      target.spec.parameters = {
        type: "object",
        properties: {
          ...(declared && { releaseRef: { type: "string" } }),
          roleName: { type: "string", "x-fleet-template": true },
        },
      };
      const spec = sources.environments[0].spec;
      spec.parameters = {
        ...spec.parameters,
        [registration]: {
          roleName: { value: "{{ .claim }}-{{ .parameters.releaseRef }}" },
        },
      };
      return sources;
    }

    const preview = (option: string) => group(option, "roleName");

    it("renders the release of a cluster claim that names one", () => {
      renderEditor(
        "production",
        withReleaseTemplate("aws-connectors-role", true),
      );
      fireEvent.change(
        within(preview("Role for each cluster")).getByLabelText("Preview for"),
        { target: { value: "1" } },
      );
      expect(
        preview("Role for each cluster").querySelector("pre")?.textContent,
      ).toBe("orders-8.9.22");
    });

    it("renders the default release of the environment for a cluster claim without one", () => {
      const sources = withReleaseTemplate("aws-connectors-role", true);
      sources.clusterClaims[1].spec.releaseRef = undefined;
      sources.environments[0].spec.releases!.default = "8.9.22";
      renderEditor("production", sources);
      expect(
        preview("Role for each cluster").querySelector("pre")?.textContent,
      ).toBe("billing-8.9.22");
    });

    it("renders the default release of the environment for the example claim", () => {
      const sources = withReleaseTemplate("aws-connectors-role", true);
      sources.clusterClaims = [];
      renderEditor("production", sources);
      expect(
        preview("Role for each cluster").querySelector("pre")?.textContent,
      ).toBe("example-8.9.23");
    });

    it("gives no release to a claim of an environment without a default release", () => {
      const sources = withReleaseTemplate("aws-connectors-role", true);
      sources.clusterClaims = [];
      sources.environments[0].spec.releases = undefined;
      renderEditor("production", sources);
      expect(
        within(preview("Role for each cluster")).getByRole("alert").textContent,
      ).toBe('parameter "releaseRef" has no value');
    });

    it("gives no release to a registration that does not declare releaseRef", () => {
      renderEditor(
        "production",
        withReleaseTemplate("aws-connectors-role", false),
      );
      expect(
        within(preview("Role for each cluster")).getByRole("alert").textContent,
      ).toBe('parameter "releaseRef" has no value');
    });

    it("gives no release to a management plane claim", () => {
      const sources = withReleaseTemplate("gateway-public", true);
      sources.environments[0].spec.sections!.access = {
        allowed: ["gateway-public"],
      };
      sources.namespaces.find(
        (n) => n.metadata?.name === "platform",
      )!.metadata!.labels = { [ENVIRONMENT_LABEL]: "production" };
      renderEditor("production", sources);
      expect(
        within(preview("Public gateway")).getByRole("option", {
          selected: true,
        }).textContent,
      ).toBe("plane-a in platform");
      expect(
        within(preview("Public gateway")).getByRole("alert").textContent,
      ).toBe('parameter "releaseRef" has no value');
    });
  });
});
