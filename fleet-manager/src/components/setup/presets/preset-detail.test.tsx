// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import type { ReadError } from "@/lib/k8s/unread";
import type { PresetKind } from "@/lib/usage";
import { foreignFields } from "@/lib/write/ownership";
import type { Change, KubeObject, Preview, WriteMode } from "@/lib/write/types";
import { presetDeleteChange } from "./delete";
import { presetDraftOf, presetUpdateChange } from "./draft";
import { presetFixtures } from "./fixtures";
import { PresetDetail } from "./preset-detail";
import type { PresetActions, PresetSources } from "./sources";

const router = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

beforeEach(() => router.push.mockClear());
afterEach(cleanup);

function setup(
  opts: {
    kind?: PresetKind;
    name?: string;
    mode?: WriteMode;
    sources?: PresetSources;
    errors?: ReadError[];
    pending?: KindName[];
    preview?: (change: Change) => Preview;
  } = {},
) {
  const {
    kind = "CamundaClusterPreset",
    name = "medium",
    sources = presetFixtures(),
    errors = [],
    pending = [],
  } = opts;
  const mode = "mode" in opts ? opts.mode : "direct";
  const actions = {
    previewChange: vi.fn<PresetActions["previewChange"]>(async (change) =>
      opts.preview ? opts.preview(change) : { objects: [], errors: [] },
    ),
    applyChange: vi.fn<PresetActions["applyChange"]>(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
  };
  const view = (s: PresetSources, e: ReadError[]) => (
    <GitPushesProvider>
      <PresetDetail
        sources={{ ...s, errors: e, pending }}
        kind={kind}
        name={name}
        secrets={{}}
        secretsUnread={[]}
        mode={mode}
        actions={actions}
      />
    </GitPushesProvider>
  );
  const { rerender } = render(view(sources, errors));
  return {
    actions,
    rerender: (s: PresetSources, e: ReadError[] = errors) =>
      rerender(view(s, e)),
  };
}

const openDelete = (name: string) => {
  fireEvent.change(screen.getByLabelText(`Type ${name} to confirm`), {
    target: { value: name },
  });
  fireEvent.click(screen.getByRole("button", { name: "Review the delete" }));
};

describe("PresetDetail", () => {
  it("shows the kind, the description, the facts, Allowed in, and Used by in the header", () => {
    setup();
    const header = screen.getByRole("banner");
    expect(
      within(header).getByRole("heading", { level: 1, name: "medium" }),
    ).toBeTruthy();
    const text = header.textContent ?? "";
    for (const part of [
      "Cluster",
      "For production workloads with three brokers and replication.",
      "Brokers",
      "32Gi",
      "production",
      "1 cluster, 1 claim",
      "team-orders/orders",
      "2026-11-14 09:30 UTC",
    ])
      expect(text).toContain(part);
  });

  it("shows the settings, the fields it cannot set, and the inheriting resources", () => {
    setup();
    expect(
      screen.getByRole("region", { name: "Brokers settings" }),
    ).toBeTruthy();
    expect(
      screen.getByRole("list", { name: "Fields a preset cannot set" }),
    ).toBeTruthy();
    expect(
      screen.getByRole("region", { name: "Inheriting resources" }).textContent,
    ).toContain("team-orders");
  });

  it("offers no delete for a used preset and says why", () => {
    setup();
    expect(
      screen.getByText(
        "Delete is possible when no resource, claim, or request uses the preset.",
      ),
    ).toBeTruthy();
    expect(screen.queryByLabelText("Type medium to confirm")).toBeNull();
  });

  it("deletes an unused preset after the typed confirm and the review", async () => {
    const { actions } = setup({ name: "spare" });
    openDelete("spare");
    const review = screen.getByRole("region", { name: "Review the delete" });
    expect(review.textContent).toContain("The preset spare is deleted.");
    const confirm = within(review).getByRole("button", {
      name: "Delete the preset",
    });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    fireEvent.click(confirm);
    await waitFor(() =>
      expect(actions.applyChange).toHaveBeenCalledWith(
        presetDeleteChange([presetFixtures().clusterPresets[1]]),
      ),
    );
    await waitFor(() =>
      expect(router.push).toHaveBeenCalledWith("/setup/presets"),
    );
  });

  it("reads Push the delete to Git in Git mode", () => {
    setup({ name: "spare", mode: "git" });
    openDelete("spare");
    const review = screen.getByRole("region", { name: "Review the delete" });
    expect(
      within(review).getByRole("button", { name: "Push the delete to Git" }),
    ).toBeTruthy();
    expect(review.textContent).toContain("GitOps must prune the presets");
  });

  it("keeps two presets of one name and two kinds apart", async () => {
    const { actions } = setup({
      kind: "DatabaseServerPreset",
      name: "standard",
    });
    expect(screen.getByRole("banner").textContent).toContain("PostgreSQL");
    openDelete("standard");
    const confirm = within(
      screen.getByRole("region", { name: "Review the delete" }),
    ).getByRole("button", { name: "Delete the preset" });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    fireEvent.click(confirm);
    await waitFor(() =>
      expect(actions.applyChange).toHaveBeenCalledWith(
        presetDeleteChange([presetFixtures().databasePresets[1]]),
      ),
    );
  });

  it("closes the delete review when an environment allows the preset", () => {
    const { actions, rerender } = setup({ name: "spare" });
    openDelete("spare");
    expect(
      screen.getByRole("region", { name: "Review the delete" }),
    ).toBeTruthy();
    const sources = presetFixtures();
    sources.environments[0].spec.presets = { camundaCluster: ["spare"] };
    rerender(sources);
    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
    expect(
      screen.getByText(
        "Delete is possible when no environment allows the preset.",
      ),
    ).toBeTruthy();
    expect(actions.applyChange).not.toHaveBeenCalled();
  });

  it("offers no delete while a list of users cannot be read", () => {
    setup({
      name: "spare",
      errors: [{ kind: "CamundaClusterClaim", message: "forbidden" }],
    });
    expect(
      screen.getByText(
        "Delete is possible when the fleet manager can read CamundaClusterClaim.",
      ),
    ).toBeTruthy();
    expect(
      screen.getByText("Could not read CamundaClusterClaim: forbidden"),
    ).toBeTruthy();
  });

  it("closes the delete review when a list of users cannot be read", () => {
    const { actions, rerender } = setup({ name: "spare" });
    openDelete("spare");
    rerender(presetFixtures(), [
      { kind: "CamundaCluster", message: "forbidden" },
    ]);
    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
    expect(
      screen.getByText(
        "Delete is possible when the fleet manager can read CamundaCluster.",
      ),
    ).toBeTruthy();
    expect(actions.applyChange).not.toHaveBeenCalled();
  });

  it("offers no delete until the lists of users are current", () => {
    setup({ name: "spare", pending: ["CamundaCluster"] });
    expect(
      screen.getByText(
        "Delete is possible when the fleet manager has the current lists.",
      ),
    ).toBeTruthy();
    expect(screen.queryByLabelText("Type spare to confirm")).toBeNull();
  });

  it("says that the inheriting resources are not known while their kind cannot be read", () => {
    setup({
      name: "spare",
      errors: [{ kind: "CamundaCluster", message: "forbidden" }],
    });
    expect(
      screen.getByText(
        "The inheriting resources are not known while the fleet manager cannot read CamundaCluster.",
      ),
    ).toBeTruthy();
    expect(screen.queryByText("No resource inherits this preset.")).toBeNull();
  });

  it("says Unknown for Used by and Allowed in while their lists cannot be read", () => {
    setup({
      errors: [
        { kind: "Environment", message: "forbidden" },
        { kind: "CamundaClusterClaim", message: "forbidden" },
      ],
    });
    const header = within(screen.getByRole("banner"));
    expect(header.getAllByText("Unknown")).toHaveLength(2);
    expect(header.queryByText("1 cluster, 1 claim")).toBeNull();
    expect(header.queryByText("production")).toBeNull();
  });

  it("pages through the inheriting resources without a write mode", () => {
    const sources = presetFixtures();
    sources.clusters = Array.from(
      { length: 30 },
      (_, i) =>
        ({
          ...sources.clusters[0],
          metadata: { namespace: "team-orders", name: `orders-${i}` },
        }) as Resource<"CamundaCluster">,
    );
    setup({ sources, mode: undefined });
    const next = screen.getByRole("button", { name: "Next page" });
    expect(next.matches(":disabled")).toBe(false);
  });

  it("says that the preset was deleted when it leaves the live list", () => {
    const { rerender } = setup({ name: "spare" });
    const sources = presetFixtures();
    sources.clusterPresets = sources.clusterPresets.slice(0, 1);
    rerender(sources);
    expect(screen.getByText("The preset spare was deleted.")).toBeTruthy();
    expect(screen.queryByLabelText("Type spare to confirm")).toBeNull();
  });

  it("disables the delete without a write mode", () => {
    setup({ name: "spare", mode: undefined });
    expect(screen.getByText(CHANGES_OFF)).toBeTruthy();
    expect(
      screen.getByRole("button", { name: "Review the delete" }),
    ).toHaveProperty("disabled", true);
  });

  it("saves an edit through the writer after the impact preview", async () => {
    const { actions } = setup({ preview: dryRun() });
    editReplicas("5");
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    const review = screen.getByRole("region", { name: "Review the preset" });
    await waitFor(() =>
      expect(review.textContent).toContain("team-orders/orders"),
    );
    expect(review.textContent).toContain("Pods roll");
    expect(review.textContent).toContain(
      "The brokers restart one at a time. The other processes restart.",
    );
    const save = within(review).getByRole("button", { name: "Save" });
    await waitFor(() => expect(save).toHaveProperty("disabled", false));
    fireEvent.click(save);
    await waitFor(() =>
      expect(actions.applyChange).toHaveBeenCalledWith(
        presetUpdateChange(medium, withReplicas(5)),
      ),
    );
  });

  it("a save that changes a field of another tool shows the field and asks", async () => {
    const ref = {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaClusterPreset",
      name: "medium",
    };
    const ownership = [
      {
        ref,
        changed: [
          {
            path: "spec.cluster.zeebe.replicas",
            steps: ["spec", "cluster", "zeebe", "replicas"].map((field) => ({
              field,
            })),
            managers: [
              {
                name: "kubectl-client-side-apply",
                operation: "Update",
                kind: "other" as const,
                known: true,
              },
            ],
          },
        ],
        cleared: [],
      },
    ];
    const { actions } = setup({
      preview: (change) =>
        change.takeOver
          ? { ...dryRun()(change), ownership }
          : {
              objects: [],
              errors: [
                {
                  code: "Conflict",
                  message: 'conflict with "kubectl-client-side-apply"',
                  path: "spec.cluster.zeebe.replicas",
                  ref,
                },
              ],
              ownership,
            },
    });
    editReplicas("5");
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    const review = screen.getByRole("region", { name: "Review the preset" });
    const choice = await within(review).findByRole("checkbox", {
      name: "Take over these fields",
    });
    expect(
      within(review).getByRole("group", {
        name: "Fields that another tool set",
      }).textContent,
    ).toContain(
      "spec.cluster.zeebe.replicas: kubectl-client-side-apply set it. The save changes it.",
    );
    const save = within(review).getByRole("button", { name: "Save" });
    expect(save).toHaveProperty("disabled", true);

    fireEvent.click(choice);
    await waitFor(() => expect(save).toHaveProperty("disabled", false));
    fireEvent.click(save);
    await waitFor(() => expect(actions.applyChange).toHaveBeenCalledTimes(1));
    expect(actions.applyChange.mock.calls[0][0]).toMatchObject({
      takeOver: {
        "CamundaClusterPreset//medium": ["spec.cluster.zeebe.replicas"],
      },
    });
    expect(actions.applyChange.mock.calls[0][0].force).toBeUndefined();
  });

  it("shows no row for an empty field of another tool that the edit typed into and emptied again", async () => {
    const sources = presetFixtures();
    const preset = sources.clusterPresets[0];
    const zeebe = preset.spec.cluster!.zeebe!;
    zeebe.storageClassName = "";
    preset.metadata.managedFields = [
      {
        manager: "kubectl-client-side-apply",
        operation: "Update",
        fieldsV1: {
          "f:spec": {
            "f:cluster": {
              "f:zeebe": { "f:replicas": {}, "f:storageClassName": {} },
            },
          },
        },
      },
    ];
    const { actions } = setup({ sources });
    const brokers = screen.getByRole("region", { name: "Brokers settings" });
    const storageClass = within(
      within(brokers).getByRole("group", { name: "storageClassName" }),
    ).getByRole("textbox");
    fireEvent.change(storageClass, { target: { value: "h" } });
    fireEvent.change(storageClass, { target: { value: "" } });
    editReplicas("5");
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    await waitFor(() => expect(actions.previewChange).toHaveBeenCalled());
    const change = actions.previewChange.mock.calls.at(-1)![0];
    const { changed, cleared } = foreignFields(
      preset as unknown as KubeObject,
      change.apply[0],
      change.keep?.["CamundaClusterPreset//medium"],
    );
    expect(changed.map((f) => f.path)).toEqual(["spec.cluster.zeebe.replicas"]);
    expect(cleared).toEqual([]);
  });

  it("reads Push to Git in Git mode", async () => {
    setup({ mode: "git", preview: dryRun() });
    editReplicas("5");
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    const review = screen.getByRole("region", { name: "Review the preset" });
    expect(
      within(review).getByRole("button", { name: "Push to Git" }),
    ).toBeTruthy();
  });

  it("computes the impact from the spec that the dry run keeps", async () => {
    // Another field manager keeps zeebe.replicas 3, so the save changes
    // nothing that the clusters inherit.
    setup({
      preview: (change) => ({
        ...dryRun()(change),
        objects: [
          {
            ref: {
              apiVersion: "core.camunda.io/v1",
              kind: "CamundaClusterPreset",
              name: "medium",
            },
            before: medium as unknown as KubeObject,
            after: medium as unknown as KubeObject,
            diff: [{ op: "added", text: "x" }],
          },
        ],
      }),
    });
    editReplicas("5");
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    const review = screen.getByRole("region", { name: "Review the preset" });
    await waitFor(() => expect(review.textContent).toContain("Unchanged"));
    expect(review.textContent).not.toContain("Pods roll");
  });

  it("holds the save and says why while the users of the impact cannot be read", async () => {
    setup({ preview: dryRun(), pending: ["CamundaCluster"] });
    editReplicas("5");
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    const review = screen.getByRole("region", { name: "Review the preset" });
    await waitFor(() =>
      expect(review.textContent).toContain(
        "The impact is not known while the fleet manager cannot read CamundaCluster.",
      ),
    );
    expect(review.textContent).not.toContain("Unchanged");
    expect(review.textContent).not.toContain(
      "No resource inherits this preset.",
    );
    expect(within(review).getByRole("button", { name: "Save" })).toHaveProperty(
      "disabled",
      true,
    );
  });

  it("refuses a misspelled field of the YAML in the review and never writes it", async () => {
    const { actions } = setup({
      kind: "ElasticsearchClusterPreset",
      name: "standard",
      preview: () => ({
        objects: [],
        errors: [
          {
            code: "Invalid",
            message:
              ".spec.cluster.storgeClassName: field not declared in schema",
          },
        ],
      }),
    });
    fireEvent.change(screen.getByLabelText("spec.cluster"), {
      target: { value: "replicas: 3\nstorgeClassName: fast\n" },
    });
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Review" })).toBeTruthy(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    const review = screen.getByRole("region", { name: "Review the preset" });
    await waitFor(() =>
      expect(review.textContent).toContain("field not declared in schema"),
    );
    const save = within(review).getByRole("button", { name: "Save" });
    expect(save).toHaveProperty("disabled", true);
    fireEvent.click(save);
    expect(actions.applyChange).not.toHaveBeenCalled();
  });

  it("disables the rotation while the form has changes", () => {
    setup();
    const rotate = screen.getByRole("button", {
      name: "Rotate the admin password of 1 cluster",
    });
    expect(rotate).toHaveProperty("disabled", false);
    editReplicas("5");
    expect(rotate).toHaveProperty("disabled", true);
  });

  it("disables the rotation while the form shows an error", async () => {
    setup();
    editReplicas("-1");
    await waitFor(() =>
      expect(
        screen.getByRole("button", {
          name: "Rotate the admin password of 1 cluster",
        }),
      ).toHaveProperty("disabled", true),
    );
  });

  it("locks the form while the rotation review is open", async () => {
    setup();
    fireEvent.click(
      screen.getByRole("button", {
        name: "Rotate the admin password of 1 cluster",
      }),
    );
    await waitFor(() =>
      expect(replicasInput().matches(":disabled")).toBe(true),
    );
  });

  it("offers no rotation for an Elasticsearch preset", () => {
    setup({ kind: "ElasticsearchClusterPreset", name: "standard" });
    expect(screen.queryByRole("button", { name: /^Rotate/ })).toBeNull();
  });

  it("keeps the inheriting resources and the delete section", () => {
    setup();
    editReplicas("5");
    expect(
      screen.getByRole("region", { name: "Inheriting resources" }).textContent,
    ).toContain("team-orders");
    expect(screen.getByText("Delete the preset")).toBeTruthy();
  });
});

const medium = presetFixtures().clusterPresets[0];

function withReplicas(n: number) {
  const draft = presetDraftOf(medium);
  return {
    ...draft,
    body: {
      ...draft.body,
      zeebe: { ...(draft.body.zeebe as object), replicas: n },
    },
  };
}

function replicasInput() {
  const brokers = screen.getByRole("region", { name: "Brokers settings" });
  return within(
    within(brokers).getByRole("group", { name: "replicas" }),
  ).getByRole("spinbutton");
}

function editReplicas(value: string) {
  fireEvent.change(replicasInput(), { target: { value } });
}

// dryRun returns a preview that keeps exactly the applied spec.
function dryRun(): (change: Change) => Preview {
  return (change) => ({
    objects: [
      {
        ref: {
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaClusterPreset",
          name: "medium",
        },
        before: medium as unknown as KubeObject,
        after: change.apply[0],
        diff: [{ op: "added", text: "replicas: 5" }],
      },
    ],
    errors: [],
  });
}
