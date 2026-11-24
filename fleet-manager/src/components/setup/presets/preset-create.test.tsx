// @vitest-environment jsdom
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyPresetChange,
  previewPresetChange,
} from "@/app/setup/presets/actions";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { Resource } from "@/lib/k8s/kinds";
import type { WriteMode } from "@/lib/write/types";
import { fakeWriter } from "../../../../test/fakes/writer";
import { CREATE_EFFECT, createChange } from "./draft";
import { presetFixtures } from "./fixtures";
import { type PresetObject, presetBody, presetsOf } from "./kinds";
import { PresetCreate } from "./preset-create";
import type { PresetSources } from "./sources";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));
vi.mock("@/lib/k8s/read", () => ({ list: vi.fn(async () => []) }));
const router = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

beforeEach(() => {
  fakeWriter.reset();
  router.push.mockClear();
});
afterEach(cleanup);

const NAME_HINT =
  "Lowercase letters, digits, and hyphens. It cannot change later.";

function setup(opts: { mode?: WriteMode; sources?: PresetSources } = {}) {
  const mode = "mode" in opts ? opts.mode : fakeWriter.mode;
  const view = (sources: PresetSources) => (
    <GitPushesProvider>
      <PresetCreate
        sources={{ ...sources, errors: [] }}
        mode={mode}
        actions={{
          previewChange: previewPresetChange,
          applyChange: applyPresetChange,
        }}
      />
    </GitPushesProvider>
  );
  const { rerender } = render(view(opts.sources ?? presetFixtures()));
  return { rerender: (sources: PresetSources) => rerender(view(sources)) };
}

const user = () => userEvent.setup();
const nameField = () =>
  screen.getByRole("textbox", {
    name: "Name",
    description: (text) => text.startsWith(NAME_HINT),
  }) as HTMLInputElement;
const copyFrom = () =>
  screen.getByRole("combobox", { name: "Copy from" }) as HTMLSelectElement;
const field = (region: string, name: string) =>
  within(screen.getByRole("region", { name: region })).getByRole("group", {
    name,
  });
const spinbutton = (region: string, name: string) =>
  within(field(region, name)).getByRole("spinbutton") as HTMLInputElement;
const reviewButton = () =>
  screen.getByRole("button", { name: "Review" }) as HTMLButtonElement;

async function create(u: ReturnType<typeof user>) {
  await u.click(reviewButton());
  const confirm = await screen.findByRole("button", { name: "Create" });
  await waitFor(() =>
    expect((confirm as HTMLButtonElement).disabled).toBe(false),
  );
  await u.click(confirm);
  await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
}

describe("PresetCreate", () => {
  it("creates an empty cluster preset with one field and a description", async () => {
    const u = user();
    const { rerender } = setup();
    await u.type(nameField(), "small-two");
    await u.type(screen.getByLabelText("Description"), "For small labs.");
    await u.type(spinbutton("Brokers settings", "replicas"), "12");
    await u.click(reviewButton());
    expect(
      within(
        screen.getByRole("region", { name: "Review the preset" }),
      ).getByText(CREATE_EFFECT),
    ).toBeTruthy();
    const confirm = screen.getByRole("button", { name: "Create" });
    await waitFor(() =>
      expect((confirm as HTMLButtonElement).disabled).toBe(false),
    );
    await u.click(confirm);
    await waitFor(() =>
      expect(fakeWriter.applied).toEqual([
        createChange("CamundaClusterPreset", "small-two", {
          body: { zeebe: { replicas: 12 } },
          description: "For small labs.",
        }),
      ]),
    );
    expect(
      await screen.findByText("The preset small-two is created."),
    ).toBeTruthy();

    const sources = presetFixtures();
    rerender({
      ...sources,
      clusterPresets: [
        ...sources.clusterPresets,
        {
          ...sources.clusterPresets[1],
          metadata: { name: "small-two", resourceVersion: "40" },
        },
      ],
    });
    await waitFor(() =>
      expect(router.push).toHaveBeenCalledWith(
        "/setup/presets/cluster/small-two",
      ),
    );
  });

  it("creates an empty Elasticsearch preset under spec.cluster", async () => {
    const u = user();
    setup();
    await u.click(screen.getByRole("radio", { name: "Elasticsearch" }));
    expect(
      screen.getByRole("region", { name: "Elasticsearch settings" }),
    ).toBeTruthy();
    expect(
      screen.queryByRole("region", { name: "Brokers settings" }),
    ).toBeNull();
    expect(
      within(
        screen.getByRole("list", { name: "Fields a preset cannot set" }),
      ).getByText("secondaryStorageConfig"),
    ).toBeTruthy();
    await u.type(nameField(), "search-two");
    await create(u);
    expect(fakeWriter.applied[0].apply).toEqual([
      {
        apiVersion: "core.camunda.io/v1",
        kind: "ElasticsearchClusterPreset",
        metadata: { name: "search-two" },
        spec: { cluster: {} },
      },
    ]);
    expect(fakeWriter.applied[0].create).toEqual([
      "ElasticsearchClusterPreset//search-two",
    ]);
  });

  it("creates an empty PostgreSQL preset under spec.server", async () => {
    const u = user();
    setup();
    await u.click(screen.getByRole("radio", { name: "PostgreSQL" }));
    expect(
      within(
        screen.getByRole("list", { name: "Fields a preset cannot set" }),
      ).getByText("databaseServerConfig"),
    ).toBeTruthy();
    await u.type(nameField(), "db-two");
    await u.type(spinbutton("PostgreSQL settings", "instances"), "15");
    await create(u);
    expect(fakeWriter.applied[0].apply[0].spec).toEqual({
      server: { instances: 15 },
    });
  });

  it.each([
    ["Cluster", "CamundaClusterPreset", "medium"],
    ["Elasticsearch", "ElasticsearchClusterPreset", "standard"],
    ["PostgreSQL", "DatabaseServerPreset", "standard"],
  ] as const)(
    "creates a copy of a %s preset with the body of its source only",
    async (label, kind, source) => {
      const u = user();
      setup();
      await u.click(screen.getByRole("radio", { name: label }));
      await u.selectOptions(copyFrom(), source);
      await u.type(nameField(), `${source}-copy`);
      await create(u);
      const preset = presetsOf(kind, presetFixtures()).find(
        (p) => p.metadata.name === source,
      ) as PresetObject;
      expect(fakeWriter.applied).toEqual([
        createChange(kind, `${source}-copy`, {
          body: presetBody(preset) ?? {},
          description: "",
        }),
      ]);
    },
  );

  it("keeps an empty string, an empty block, and a ConfigMap env entry of the copied body", async () => {
    const sources = presetFixtures();
    const body = {
      operate: {},
      zeebe: {
        storageClassName: "",
        resources: {},
        extraEnv: [
          {
            name: "LOG_LEVEL",
            valueFrom: { configMapKeyRef: { name: "app", key: "level" } },
          },
        ],
      },
    };
    const shapes = {
      ...sources.clusterPresets[1],
      metadata: { name: "shapes", resourceVersion: "41" },
      spec: { cluster: body },
    } as Resource<"CamundaClusterPreset">;
    const u = user();
    setup({
      sources: {
        ...sources,
        clusterPresets: [...sources.clusterPresets, shapes],
      },
    });
    await u.selectOptions(copyFrom(), "shapes");
    await u.type(nameField(), "shapes-copy");
    await u.type(spinbutton("Brokers settings", "replicas"), "12");
    await create(u);
    expect(fakeWriter.applied[0].apply[0].spec).toEqual({
      cluster: { ...body, zeebe: { ...body.zeebe, replicas: 12 } },
    });
  });

  it("keeps the typed name and description and replaces the body when the copy source changes", async () => {
    const u = user();
    setup();
    await u.click(screen.getByRole("radio", { name: "PostgreSQL" }));
    await u.type(nameField(), "next-db");
    await u.type(screen.getByLabelText("Description"), "For databases.");
    await u.selectOptions(copyFrom(), "small");
    expect(spinbutton("PostgreSQL settings", "instances").value).toBe("1");
    await u.selectOptions(copyFrom(), "standard");
    expect(spinbutton("PostgreSQL settings", "instances").value).toBe("2");
    expect(
      (
        within(field("PostgreSQL settings", "walStorageSize")).getByRole(
          "textbox",
        ) as HTMLInputElement
      ).value,
    ).toBe("5Gi");
    expect((nameField() as HTMLInputElement).value).toBe("next-db");
    expect(
      (screen.getByLabelText("Description") as HTMLInputElement).value,
    ).toBe("For databases.");
  });

  it("starts empty when the kind changes after a copy", async () => {
    const u = user();
    setup();
    await u.click(screen.getByRole("radio", { name: "Elasticsearch" }));
    await u.selectOptions(copyFrom(), "standard");
    await u.click(screen.getByRole("radio", { name: "PostgreSQL" }));
    expect(copyFrom().value).toBe("");
    expect([...copyFrom().options].map((o) => o.text)).toEqual([
      "Nothing, start empty",
      "small",
      "standard",
    ]);
    expect(spinbutton("PostgreSQL settings", "instances").value).toBe("");
    await u.type(nameField(), "db-three");
    await create(u);
    expect(fakeWriter.applied[0].apply[0].spec).toEqual({ server: {} });
  });

  it("checks the name against the presets of the chosen kind", async () => {
    const u = user();
    setup();
    await u.click(screen.getByRole("radio", { name: "Elasticsearch" }));
    await u.type(nameField(), "standard");
    expect(
      screen.getAllByText("A preset of this kind with this name exists.")
        .length,
    ).toBeGreaterThan(0);
    expect(reviewButton().disabled).toBe(true);
    await u.click(screen.getByRole("radio", { name: "Cluster" }));
    expect(
      screen.queryByText("A preset of this kind with this name exists."),
    ).toBeNull();
    expect(reviewButton().disabled).toBe(false);
    await u.clear(nameField());
    await u.type(nameField(), "team.a");
    expect(
      screen.getAllByText(
        "The name has only lowercase letters, digits, and hyphens, and at most 253 characters.",
      ).length,
    ).toBeGreaterThan(0);
    expect(reviewButton().disabled).toBe(true);
  });

  it("blocks the create while a field shows an error, and drops the error with the kind", async () => {
    const u = user();
    setup();
    await u.click(screen.getByRole("radio", { name: "Elasticsearch" }));
    await u.type(nameField(), "search-three");
    await u.type(spinbutton("Elasticsearch settings", "replicas"), "-3");
    expect(screen.getAllByText("Enter 1 or more").length).toBeGreaterThan(0);
    expect(reviewButton().disabled).toBe(true);
    await u.click(screen.getByRole("radio", { name: "PostgreSQL" }));
    expect(screen.queryByText("Enter 1 or more")).toBeNull();
    expect(reviewButton().disabled).toBe(false);
    await u.type(spinbutton("PostgreSQL settings", "instances"), "15");
    expect(reviewButton().disabled).toBe(false);
  });

  it("stays on the page with the name when the create fails", async () => {
    fakeWriter.results = [
      {
        ok: false,
        errors: [
          {
            code: "Stale",
            message:
              "The DatabaseServerPreset db-four exists already. Pick another name.",
          },
        ],
      },
    ];
    const u = user();
    setup();
    await u.click(screen.getByRole("radio", { name: "PostgreSQL" }));
    await u.type(nameField(), "db-four");
    await create(u);
    expect(
      await screen.findByText(
        "The DatabaseServerPreset db-four exists already. Pick another name.",
        { exact: false },
      ),
    ).toBeTruthy();
    expect(router.push).not.toHaveBeenCalled();
    await u.click(screen.getByRole("button", { name: "Back" }));
    expect(nameField().matches(":disabled")).toBe(false);
    expect(nameField().value).toBe("db-four");
  });

  it("disables the form while changes are off", () => {
    setup({ mode: undefined });
    expect(screen.getByText(CHANGES_OFF)).toBeTruthy();
    for (const label of ["Cluster", "Elasticsearch", "PostgreSQL"])
      expect(
        screen.getByRole("radio", { name: label }).matches(":disabled"),
      ).toBe(true);
    expect(copyFrom().matches(":disabled")).toBe(true);
    expect(nameField().matches(":disabled")).toBe(true);
    expect(screen.queryByRole("button", { name: "Review" })).toBeNull();
  });
});
