// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { secretRefsOf } from "@/components/fleet/secret-ref-picker";
import { clusterFields, type Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { claimAfter } from "@/lib/session";
import { foreignFields } from "@/lib/write/ownership";
import type { KubeObject } from "@/lib/write/types";
import {
  claim,
  cluster,
  slotContext,
} from "../../../../test/fakes/slot-context";
import { clusterFieldsInputOf } from "../settings";
import type { ClusterSlotContext } from "../slots";
import {
  type ClaimActions,
  ClusterEditProvider,
  useClusterEdit,
} from "./cluster-edit";
import { ComponentSettingsEditor } from "./settings-slot";

const SHRINK = "Volumes cannot shrink. The operator keeps the current size.";

const live = vi.hoisted(() => ({ secrets: [] as unknown[] }));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string) => ({
    items: kind === "Secret" ? live.secrets : [],
  }),
  useGitSync: () => "pending",
  useGitGone: () => false,
}));

beforeEach(() => {
  live.secrets = [];
});
afterEach(cleanup);

const actions: ClaimActions = {
  preview: () => Promise.reject(new Error("no preview in this test")),
  apply: () => Promise.reject(new Error("no write in this test")),
};

const attachment = {
  managementAuthConfigRef: "keycloak",
  importer: { replicas: 1 },
};

const volumeEncryption = {
  kind: "PluginSection",
  metadata: { name: "volume-encryption" },
  spec: {
    label: "Volume encryption",
    slot: "volumeEncryption",
    appliesTo: ["CamundaClusterClaim"],
    selection: "One",
  },
} as unknown as Resource<"PluginSection">;

// contextOf returns the slot context of the managed cluster orders, whose
// gateway runs standalone and hosts Tasklist, with an Optimize attachment.
function contextOf(over: Partial<ClusterSlotContext> = {}): ClusterSlotContext {
  const spec = { zeebe: { replicas: 3 }, gateway: { mode: "Standalone" } };
  const base = slotContext({
    cluster: cluster(spec),
    claim: claim("41", {
      cluster: spec,
      attachments: { optimize: attachment },
    }),
    optimize: {
      kind: "CamundaOptimize",
      metadata: { namespace: "team-a", name: "orders" },
      spec: {
        clusterRef: { name: "orders" },
        managementAuthRef: "keycloak",
        importer: { replicas: 1 },
      },
    } as unknown as Resource<"CamundaOptimize">,
    ...over,
  });
  return { ...base, fields: clusterFields(clusterFieldsInputOf(base)) };
}

// Probe shows the claim after the pending changes.
function Probe() {
  const { session } = useClusterEdit();
  return (
    <p data-testid="claim">
      {session ? JSON.stringify(claimAfter(session).spec) : ""}
    </p>
  );
}

// Pend makes a pending change of value at steps on a click on its button,
// as another editor of the page, such as the Claim tab, does.
function Pend({ steps, value }: { steps: Step[]; value: unknown }) {
  const { set } = useClusterEdit();
  return (
    <button type="button" onClick={() => set(steps, value)}>
      Pend
    </button>
  );
}

// renderSlot renders the slot of context. update gives the page a newer
// context, as when the claim updates. pend adds a Pend button.
function renderSlot(
  context: ClusterSlotContext,
  pend?: { steps: Step[]; value: unknown },
) {
  const client = new QueryClient();
  const tree = (c: ClusterSlotContext) => (
    <QueryClientProvider client={client}>
      <GitPushesProvider>
        <ClusterEditProvider context={c} actions={actions}>
          <ComponentSettingsEditor ctx={c} />
          <Probe />
          {pend && <Pend {...pend} />}
        </ClusterEditProvider>
      </GitPushesProvider>
    </QueryClientProvider>
  );
  const result = render(tree(context));
  return {
    ...result,
    update: (c: ClusterSlotContext) => result.rerender(tree(c)),
  };
}

function claimSpec() {
  return JSON.parse(screen.getByTestId("claim").textContent ?? "{}");
}

function group(label: string): HTMLElement {
  return screen.getByRole("region", { name: `${label} settings` });
}

function settingRow(within_: HTMLElement, label: string): HTMLElement {
  return within(within_).getAllByRole("group", { name: label })[0];
}

async function openEditors() {
  await userEvent.click(screen.getByRole("button", { name: "Edit settings" }));
}

describe("ComponentSettingsEditor", () => {
  it("opens the editor of the Claim tab for a section", async () => {
    const plugins = [{ section: "volume-encryption", registration: "aws-ebs" }];
    renderSlot(
      contextOf({
        openId: "section:volume-encryption",
        claim: claim("41", { plugins }),
        sections: [volumeEncryption],
        registrations: [
          {
            kind: "PluginRegistration",
            metadata: { name: "aws-ebs" },
            spec: {
              label: "Amazon EBS",
              sections: ["volume-encryption"],
              mode: "Optional",
              deletionPolicies: ["Delete"],
              serviceAccount: { name: "aws-ebs", namespace: "plugins" },
            },
          } as unknown as Resource<"PluginRegistration">,
        ],
      }),
    );
    await openEditors();
    const option = screen.getByRole("group", { name: "Option" });
    expect(option.textContent).toContain("Amazon EBS");
  });

  it("renders nothing for a section that the claim no longer selects", () => {
    renderSlot(contextOf({ openId: "section:backup-storage" }));
    expect(screen.queryByRole("button", { name: "Edit settings" })).toBeNull();
  });

  it("an embedded Tasklist field writes to spec.cluster.tasklist", async () => {
    renderSlot(contextOf({ openId: "gateway" }));
    await openEditors();
    const tasklist = group("Tasklist");
    expect(
      within(tasklist).getByText(
        "Applies when Tasklist runs standalone. The exception is extraEnv and extraEnvFrom: they apply to the host now.",
      ),
    ).toBeDefined();
    await userEvent.type(
      within(settingRow(tasklist, "replicas")).getByRole("spinbutton"),
      "2",
    );
    expect(claimSpec().cluster.tasklist.replicas).toBe(2);
  });

  it("an Optimize field writes to spec.attachments.optimize", async () => {
    renderSlot(contextOf({ openId: "optimize" }));
    await openEditors();
    const replicas = within(
      settingRow(group("Optimize"), "importer.replicas"),
    ).getByRole("spinbutton");
    await userEvent.clear(replicas);
    await userEvent.type(replicas, "0");
    expect(claimSpec().attachments.optimize).toEqual({
      managementAuthConfigRef: "keycloak",
      importer: { replicas: 0 },
    });
  });

  it("refuses an Optimize request typed above its limit", async () => {
    const optimize = {
      managementAuthConfigRef: "keycloak",
      importer: {
        replicas: 1,
        resources: { requests: { cpu: "2" }, limits: { cpu: "3" } },
      },
    };
    const spec = { zeebe: { replicas: 3 }, gateway: { mode: "Standalone" } };
    renderSlot(
      contextOf({
        openId: "optimize",
        claim: claim("41", {
          cluster: spec,
          attachments: { optimize },
        }),
      }),
    );
    await openEditors();
    const request = within(
      within(settingRow(group("Optimize"), "importer.resources")).getByRole(
        "group",
        { name: "CPU request" },
      ),
    ).getByRole("textbox", { name: "Amount" });
    await userEvent.type(request, "4");
    expect(screen.getByText("The limit is below the request")).toBeTruthy();
    expect(claimSpec().attachments.optimize).toEqual(optimize);
  });

  it("a new Optimize field shows that the claim sets it", async () => {
    renderSlot(contextOf({ openId: "optimize" }));
    await openEditors();
    const row = settingRow(group("Optimize"), "webapp.replicas");
    expect(within(row).getByText("Not set")).toBeDefined();
    await userEvent.type(within(row).getByRole("spinbutton"), "2");
    expect(within(row).getByText("Changed in the claim")).toBeDefined();
    expect(within(row).getByRole("button", { name: /^Reset/ })).toBeDefined();
  });

  it("a locked field shows its reason and no editor", async () => {
    renderSlot(
      contextOf({
        openId: "zeebe",
        sections: [volumeEncryption],
        claim: claim("41", {
          cluster: { zeebe: { replicas: 3 }, gateway: { mode: "Standalone" } },
          plugins: [
            { section: "volume-encryption", registration: "aws-ebs-kms" },
          ],
        }),
      }),
    );
    await openEditors();
    const row = settingRow(group("Brokers"), "storageClassName");
    expect(within(row).getByText("Filled by Volume encryption")).toBeDefined();
    expect(within(row).queryByRole("textbox")).toBeNull();
  });

  it("a Secret value never reaches the browser", async () => {
    const secret = {
      metadata: { name: "orders-db" },
      keys: ["password"],
      data: { password: "czNjcjN0" },
      stringData: { password: "s3cr3t" },
    };
    live.secrets = [secret];
    renderSlot(contextOf({ openId: "zeebe" }));
    await openEditors();
    const env = settingRow(group("Brokers"), "extraEnv");
    await userEvent.click(
      within(env).getByRole("button", { name: "Add a variable" }),
    );
    await userEvent.selectOptions(
      within(env).getByRole("combobox", { name: "Source" }),
      "secret",
    );
    const secretPicker = within(env).getByRole("combobox", { name: "Secret" });
    await userEvent.selectOptions(secretPicker, "orders-db");
    expect(
      within(env)
        .getAllByRole("option")
        .map((o) => o.textContent),
    ).toEqual(expect.arrayContaining(["orders-db", "password"]));
    expect(document.body.textContent).not.toContain("s3cr3t");
    expect(document.body.textContent).not.toContain("czNjcjN0");
    expect(document.body.innerHTML).not.toContain("s3cr3t");
    expect(secretRefsOf([secret])).toEqual([
      { name: "orders-db", keys: ["password"] },
    ]);
  });

  it("Discard shows the live values again", async () => {
    renderSlot(contextOf({ openId: "zeebe" }));
    await openEditors();
    const input = () =>
      within(settingRow(group("Brokers"), "replicas")).getByRole("spinbutton");
    await userEvent.clear(input());
    await userEvent.type(input(), "2");
    expect(claimSpec().cluster.zeebe.replicas).toBe(2);
    await userEvent.click(screen.getByRole("button", { name: "Discard" }));
    expect(claimSpec().cluster.zeebe.replicas).toBe(3);
    expect(input()).toHaveProperty("value", "3");
  });

  it("a value with an error after a valid one blocks Review and names the field until the error goes", async () => {
    renderSlot(contextOf({ openId: "gateway" }));
    await openEditors();
    const input = within(settingRow(group("Tasklist"), "replicas")).getByRole(
      "spinbutton",
    );
    const bar = () => screen.getByRole("region", { name: "Unsaved changes" });
    const review = () =>
      within(bar()).getByRole<HTMLButtonElement>("button", { name: "Review" });
    await userEvent.type(input, "2");
    expect(review().disabled).toBe(false);
    await userEvent.type(input, ".5");
    expect(claimSpec().cluster.tasklist.replicas).toBe(2);
    expect(review().disabled).toBe(true);
    expect(
      within(bar()).getByText("replicas: Enter a whole number"),
    ).toBeTruthy();
    await userEvent.type(input, "{Backspace}{Backspace}");
    expect(review().disabled).toBe(false);
    expect(within(bar()).queryByText(/Enter a whole number/)).toBeNull();
  });

  it("a cleared limit that leaves the limit of the preset below the request blocks Review and names the preset", async () => {
    const spec = {
      zeebe: {
        replicas: 3,
        resources: { requests: { cpu: "2" }, limits: { cpu: "3" } },
      },
      gateway: { mode: "Standalone" },
    };
    renderSlot(
      contextOf({
        openId: "zeebe",
        cluster: cluster(spec),
        claim: claim("41", { cluster: spec }),
        presets: [
          {
            kind: "CamundaClusterPreset",
            metadata: { name: "medium" },
            spec: {
              cluster: { zeebe: { resources: { limits: { cpu: "1" } } } },
            },
          } as unknown as Resource<"CamundaClusterPreset">,
        ],
      }),
    );
    await openEditors();
    const resources = () => settingRow(group("Brokers"), "resources");
    await userEvent.clear(
      within(
        within(resources()).getByRole("group", { name: "CPU limit" }),
      ).getByRole("textbox", { name: "Amount" }),
    );
    const message =
      "The CPU limit of 1 comes from the preset and is below the CPU request of 2.";
    expect(within(resources()).getByRole("alert").textContent).toBe(message);
    const bar = screen.getByRole("region", { name: "Unsaved changes" });
    expect(
      within(bar).getByRole<HTMLButtonElement>("button", { name: "Review" })
        .disabled,
    ).toBe(true);
    expect(within(bar).getByText(`zeebe.resources: ${message}`)).toBeTruthy();
    expect(within(resources()).getAllByRole("alert")).toHaveLength(1);
  });

  it("keeps Review off for a merged error after the editors close", async () => {
    const spec = {
      zeebe: {
        replicas: 3,
        resources: { requests: { cpu: "2" }, limits: { cpu: "3" } },
      },
      gateway: { mode: "Standalone" },
    };
    renderSlot(
      contextOf({
        openId: "zeebe",
        cluster: cluster(spec),
        claim: claim("41", { cluster: spec }),
        presets: [
          {
            kind: "CamundaClusterPreset",
            metadata: { name: "medium" },
            spec: {
              cluster: { zeebe: { resources: { limits: { cpu: "1" } } } },
            },
          } as unknown as Resource<"CamundaClusterPreset">,
        ],
      }),
    );
    await openEditors();
    await userEvent.clear(
      within(
        within(settingRow(group("Brokers"), "resources")).getByRole("group", {
          name: "CPU limit",
        }),
      ).getByRole("textbox", { name: "Amount" }),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Close the editors" }),
    );
    const bar = screen.getByRole("region", { name: "Unsaved changes" });
    expect(
      within(bar).getByRole<HTMLButtonElement>("button", { name: "Review" })
        .disabled,
    ).toBe(true);
    expect(
      within(bar).getByText(
        "zeebe.resources: The CPU limit of 1 comes from the preset and is below the CPU request of 2.",
      ),
    ).toBeTruthy();
  });

  // brokerSlot renders the brokers of a cluster whose claim and live
  // cluster hold resources, under the preset medium with presetResources.
  function brokerSlot(resources: object, presetResources: object) {
    const spec = {
      zeebe: { replicas: 3, resources },
      gateway: { mode: "Standalone" },
    };
    const preset = {
      kind: "CamundaClusterPreset",
      metadata: { name: "medium" },
      spec: { cluster: { zeebe: { resources: presetResources } } },
    } as unknown as Resource<"CamundaClusterPreset">;
    const of = (s: object) =>
      contextOf({
        openId: "zeebe",
        cluster: cluster(s),
        claim: claim("41", { cluster: s }),
        presets: [preset],
      });
    return { view: renderSlot(of(spec)), of, spec };
  }
  const brokerAmount = (label: string) =>
    within(
      within(settingRow(group("Brokers"), "resources")).getByRole("group", {
        name: label,
      }),
    ).getByRole("textbox", { name: "Amount" });

  it("leaves no edit of a longer value after a reset to the preset and a backspace", async () => {
    brokerSlot({ requests: { cpu: "2" } }, { requests: { cpu: "1" } });
    await openEditors();
    await userEvent.click(
      within(settingRow(group("Brokers"), "resources")).getByRole("button", {
        name: "Reset to preset",
      }),
    );
    await userEvent.type(brokerAmount("CPU request"), "2");
    expect(claimSpec().cluster.zeebe.resources).toEqual({
      requests: { cpu: "12" },
    });
    await userEvent.type(brokerAmount("CPU request"), "{Backspace}");
    expect(claimSpec().cluster.zeebe.resources).toEqual({
      requests: { cpu: "1" },
    });
  });

  it("ends with no change after a delete and a retype of the value of the claim", async () => {
    brokerSlot({ requests: { cpu: "1" } }, { requests: { cpu: "1" } });
    await openEditors();
    await userEvent.clear(brokerAmount("CPU request"));
    expect(
      screen.getByRole("region", { name: "Unsaved changes" }).textContent,
    ).toContain("1 unsaved change");
    await userEvent.type(brokerAmount("CPU request"), "1");
    expect(
      screen.queryByRole("region", { name: "Unsaved changes" }),
    ).toBeNull();
  });

  describe("an edit of one amount of the resources", () => {
    const PRESET = {
      requests: { cpu: "1", memory: "1Gi" },
      limits: { cpu: "2", memory: "2Gi" },
    };
    const unsaved = () =>
      screen.queryByRole("region", { name: "Unsaved changes" })?.textContent;
    const retype = async (label: string, text: string) => {
      await userEvent.clear(brokerAmount(label));
      await userEvent.type(brokerAmount(label), text);
    };

    it("writes only that amount over the preset, as one change", async () => {
      brokerSlot({}, PRESET);
      await openEditors();
      await retype("Memory request", "2");
      expect(claimSpec().cluster.zeebe.resources).toEqual({
        requests: { memory: "2Gi" },
      });
      expect(unsaved()).toContain("1 unsaved change");
    });

    it("keeps a number of the claim when another amount changes", async () => {
      brokerSlot({ requests: { cpu: 1 } }, PRESET);
      await openEditors();
      await retype("Memory limit", "4");
      expect(claimSpec().cluster.zeebe.resources).toEqual({
        requests: { cpu: 1 },
        limits: { memory: "4Gi" },
      });
    });

    it("leaves no change after a retype of the claim amount once the editors open again", async () => {
      brokerSlot({ requests: { cpu: "3" } }, PRESET);
      await openEditors();
      await retype("CPU request", "5");
      await userEvent.click(
        screen.getByRole("button", { name: "Close the editors" }),
      );
      await openEditors();
      expect(brokerAmount("CPU request")).toHaveProperty("value", "5");
      await retype("Memory request", "4");
      expect(claimSpec().cluster.zeebe.resources).toEqual({
        requests: { cpu: "5", memory: "4Gi" },
      });
      await retype("CPU request", "3");
      await retype("Memory request", "1");
      expect(unsaved()).toBeUndefined();
    });

    it("leaves no change after a retype of the claim amount once a reset of another setting rebuilt the rows", async () => {
      brokerSlot({ requests: { cpu: "3" } }, PRESET);
      await openEditors();
      await retype("CPU request", "5");
      await userEvent.click(
        within(settingRow(group("Brokers"), "replicas")).getByRole("button", {
          name: "Reset to preset",
        }),
      );
      expect(unsaved()).toContain("2 unsaved changes");
      await retype("CPU request", "3");
      expect(unsaved()).toContain("1 unsaved change");
      expect(claimSpec().cluster.zeebe.resources).toEqual({
        requests: { cpu: "3" },
      });
    });

    it("keeps the other amounts on the preset after a reset of the block", async () => {
      brokerSlot({ requests: { cpu: "3", memory: "3Gi" } }, PRESET);
      await openEditors();
      await userEvent.click(
        within(settingRow(group("Brokers"), "resources")).getByRole("button", {
          name: "Reset to preset",
        }),
      );
      await retype("Memory request", "4");
      expect(claimSpec().cluster.zeebe.resources).toEqual({
        requests: { memory: "4Gi" },
      });
      expect(unsaved()).toContain("1 unsaved change");
      await retype("Memory request", "3");
      expect(claimSpec().cluster.zeebe.resources).toEqual({
        requests: { memory: "3Gi" },
      });
      await retype("CPU request", "3");
      expect(unsaved()).toBeUndefined();
    });

    it("keeps the amount of another writer that the editor did not load", async () => {
      const { view, of, spec } = brokerSlot(
        { requests: { cpu: "1" }, limits: { cpu: "2" } },
        PRESET,
      );
      await openEditors();
      await userEvent.type(brokerAmount("CPU request"), "0");
      const newer = {
        ...spec,
        zeebe: {
          ...spec.zeebe,
          resources: { requests: { cpu: "1" }, limits: { cpu: "40" } },
        },
      };
      view.update({ ...of(newer), claim: claim("42", { cluster: newer }) });
      expect(brokerAmount("CPU limit")).toHaveProperty("value", "2");
      await userEvent.type(brokerAmount("CPU request"), "0");
      expect(claimSpec().cluster.zeebe.resources).toEqual({
        requests: { cpu: "100" },
        limits: { cpu: "40" },
      });
    });

    it("changes no field of another manager that the user did not touch", async () => {
      const { spec } = brokerSlot({ limits: { cpu: 2 } }, PRESET);
      await openEditors();
      await retype("Memory request", "2");
      const live = {
        ...claim("41", { cluster: spec }),
        metadata: {
          ...claim("41").metadata,
          managedFields: [
            {
              manager: "kubectl",
              operation: "Update",
              fieldsV1: {
                "f:spec": {
                  "f:cluster": {
                    "f:zeebe": {
                      "f:resources": { "f:limits": { "f:cpu": {} } },
                    },
                  },
                },
              },
            },
          ],
        },
      } as unknown as KubeObject;
      const applied = { ...live, spec: claimSpec() } as KubeObject;
      expect(foreignFields(live, applied)).toEqual({
        changed: [],
        cleared: [],
      });
    });
  });

  describe("what an edit of an amount writes while the claim or the preset changes", () => {
    const resourcesAt = (
      version: string,
      resources: object,
      presets: Resource<"CamundaClusterPreset">[],
      presetRef = "medium",
    ) => {
      const spec = {
        zeebe: { replicas: 3, resources },
        gateway: { mode: "Standalone" },
      };
      return contextOf({
        openId: "zeebe",
        cluster: cluster(spec),
        claim: claim(version, { cluster: spec, presetRef }),
        presets,
      });
    };
    const preset = (name: string, resources: object) =>
      ({
        kind: "CamundaClusterPreset",
        metadata: { name },
        spec: { cluster: { zeebe: { resources } } },
      }) as unknown as Resource<"CamundaClusterPreset">;
    const medium = preset("medium", { requests: { memory: "1Gi" } });
    const large = preset("large", { requests: { memory: "4Gi" } });
    const unsaved = () =>
      screen.queryByRole("region", { name: "Unsaved changes" })?.textContent;
    const retype = async (label: string, text: string) => {
      await userEvent.clear(brokerAmount(label));
      if (text) await userEvent.type(brokerAmount(label), text);
    };
    const steps = (...names: string[]): Step[] =>
      names.map((field) => ({ field }));
    const resourcesSteps = steps("spec", "cluster", "zeebe", "resources");

    it("saves the newer amount of another writer without a second keystroke", async () => {
      const { update } = renderSlot(
        resourcesAt("41", { requests: { cpu: "1" } }, [medium]),
      );
      await openEditors();
      await retype("Memory request", "2");
      update(resourcesAt("42", { requests: { cpu: "4" } }, [medium]));
      expect(claimSpec().cluster.zeebe.resources).toEqual({
        requests: { cpu: "4", memory: "2Gi" },
      });
    });

    it("keeps the newer amount of another writer after the editors open again", async () => {
      const { update } = renderSlot(
        resourcesAt("41", { requests: { cpu: "1" } }, [medium]),
      );
      await openEditors();
      await retype("Memory request", "2");
      await userEvent.click(
        screen.getByRole("button", { name: "Close the editors" }),
      );
      update(resourcesAt("42", { requests: { cpu: "4" } }, [medium]));
      await openEditors();
      await retype("Memory request", "3");
      expect(claimSpec().cluster.zeebe.resources).toEqual({
        requests: { cpu: "4", memory: "3Gi" },
      });
    });

    it("leaves a key of another writer as a reset of the block left it after a keystroke", async () => {
      const { update } = renderSlot(
        resourcesAt("41", { requests: { cpu: "1" } }, [medium]),
      );
      await openEditors();
      await userEvent.click(
        within(settingRow(group("Brokers"), "resources")).getByRole("button", {
          name: "Reset to preset",
        }),
      );
      update(
        resourcesAt(
          "42",
          { requests: { cpu: "1" }, limits: { memory: "3Gi" } },
          [medium],
        ),
      );
      const before = claimSpec().cluster.zeebe.resources?.limits;
      await retype("Memory request", "2");
      expect(claimSpec().cluster.zeebe.resources?.limits).toEqual(before);
      expect(claimSpec().cluster.zeebe.resources?.requests).toEqual({
        memory: "2Gi",
      });
    });

    it("writes an amount that equals the live preset when a new preset is pending", async () => {
      renderSlot(resourcesAt("41", {}, [medium, large]), {
        steps: steps("spec", "presetRef"),
        value: "large",
      });
      await userEvent.click(screen.getByRole("button", { name: "Pend" }));
      await openEditors();
      expect(brokerAmount("Memory request")).toHaveProperty("value", "4");
      await retype("Memory request", "1");
      expect(claimSpec().cluster.zeebe.resources).toEqual({
        requests: { memory: "1Gi" },
      });
    });

    it("writes nothing for an amount that equals the pending preset", async () => {
      renderSlot(resourcesAt("41", {}, [medium, large]), {
        steps: steps("spec", "presetRef"),
        value: "large",
      });
      await userEvent.click(screen.getByRole("button", { name: "Pend" }));
      await openEditors();
      await retype("Memory request", "4");
      expect(claimSpec().cluster.zeebe.resources).toEqual({});
      expect(unsaved()).toContain("1 unsaved change");
    });

    it("leaves no change after a type and a clear beside an empty requests of the claim", async () => {
      renderSlot(resourcesAt("41", { requests: {} }, [preset("medium", {})]));
      await openEditors();
      await userEvent.type(brokerAmount("CPU limit"), "2");
      expect(unsaved()).toContain("1 unsaved change");
      await retype("CPU limit", "");
      expect(unsaved()).toBeUndefined();
    });

    it("writes every amount typed while an amount was not valid once it is valid", async () => {
      renderSlot(resourcesAt("41", {}, [medium]));
      await openEditors();
      await userEvent.type(brokerAmount("CPU request"), "x");
      await retype("Memory request", "2");
      expect(unsaved()).toBeUndefined();
      await retype("CPU request", "3");
      expect(claimSpec().cluster.zeebe.resources).toEqual({
        requests: { cpu: "3", memory: "2Gi" },
      });
    });

    it("writes a key into a pending change of the whole block from another editor", async () => {
      renderSlot(resourcesAt("41", { requests: { cpu: "1" } }, [medium]), {
        steps: resourcesSteps,
        value: {
          requests: { cpu: "5" },
          limits: { "ephemeral-storage": "1Gi" },
        },
      });
      await userEvent.click(screen.getByRole("button", { name: "Pend" }));
      await openEditors();
      await retype("Memory request", "2");
      expect(claimSpec().cluster.zeebe.resources).toEqual({
        requests: { cpu: "5", memory: "2Gi" },
        limits: { "ephemeral-storage": "1Gi" },
      });
      expect(unsaved()).toContain("1 unsaved change");
    });

    it("counts two changed amounts of one block as two changes", async () => {
      renderSlot(resourcesAt("41", {}, [medium]));
      await openEditors();
      await retype("Memory request", "2");
      await retype("CPU limit", "3");
      expect(unsaved()).toContain("2 unsaved changes");
    });

    it("removes the block, not a copy of it, when the last amount of the claim goes", async () => {
      renderSlot(resourcesAt("41", { requests: { cpu: "1" } }, [medium]));
      await openEditors();
      await retype("CPU request", "");
      expect(claimSpec().cluster.zeebe.resources).toBeUndefined();
      expect(unsaved()).toContain("1 unsaved change");
    });
  });

  it("closing the editors clears the error that one of them shows", async () => {
    renderSlot(contextOf({ openId: "gateway" }));
    await openEditors();
    const input = within(settingRow(group("Tasklist"), "replicas")).getByRole(
      "spinbutton",
    );
    await userEvent.type(input, "2.5");
    const bar = screen.getByRole("region", { name: "Unsaved changes" });
    const review = within(bar).getByRole<HTMLButtonElement>("button", {
      name: "Review",
    });
    expect(review.disabled).toBe(true);
    await userEvent.click(
      screen.getByRole("button", { name: "Close the editors" }),
    );
    expect(review.disabled).toBe(false);
  });

  it("an edit that the user takes back keeps the editor and its focus", async () => {
    renderSlot(contextOf({ openId: "gateway" }));
    await openEditors();
    const input = within(settingRow(group("Tasklist"), "replicas")).getByRole(
      "spinbutton",
    );
    await userEvent.type(input, "2");
    expect(claimSpec().cluster.tasklist.replicas).toBe(2);
    await userEvent.type(input, "{Backspace}");
    expect(claimSpec().cluster.tasklist).toBeUndefined();
    expect(input.isConnected).toBe(true);
    expect(document.activeElement).toBe(input);
    expect(input).toHaveProperty("value", "");
  });

  describe("a claim update with an unrelated pending change", () => {
    // brokers gives the brokers of orders the CPU limit limit, in the claim
    // of resourceVersion version.
    const brokers = (version: string, limit: string) => {
      const spec = {
        zeebe: {
          replicas: 3,
          resources: { requests: { cpu: "1" }, limits: { cpu: limit } },
        },
        gateway: { mode: "Standalone" },
      };
      return contextOf({
        openId: "zeebe",
        cluster: cluster(spec),
        claim: claim(version, { cluster: spec }),
      });
    };
    const amount = (label: string) =>
      within(
        within(settingRow(group("Brokers"), "resources")).getByRole("group", {
          name: label,
        }),
      ).getByRole("textbox", { name: "Amount" });
    const replicas = () =>
      within(settingRow(group("Brokers"), "replicas")).getByRole("spinbutton");

    it("shows the new value in an editor that the user did not touch", async () => {
      const { update } = renderSlot(brokers("41", "2"));
      await openEditors();
      await userEvent.clear(replicas());
      await userEvent.type(replicas(), "5");
      update(brokers("42", "4"));
      expect(amount("CPU limit")).toHaveProperty("value", "4");
    });

    it("an edit of one field of a block keeps the new value of the others", async () => {
      const { update } = renderSlot(brokers("41", "2"));
      await openEditors();
      await userEvent.clear(replicas());
      await userEvent.type(replicas(), "5");
      update(brokers("42", "4"));
      await userEvent.clear(amount("CPU request"));
      await userEvent.type(amount("CPU request"), "2");
      expect(claimSpec().cluster.zeebe).toEqual({
        replicas: 5,
        resources: { requests: { cpu: "2" }, limits: { cpu: "4" } },
      });
    });

    it("keeps the text, the editor, and the focus of an editor that the user touched", async () => {
      const { update } = renderSlot(brokers("41", "2"));
      await openEditors();
      const input = replicas();
      await userEvent.clear(input);
      await userEvent.type(input, "5");
      update(brokers("42", "4"));
      expect(input.isConnected).toBe(true);
      expect(document.activeElement).toBe(input);
      expect(input).toHaveProperty("value", "5");
    });
  });

  it("a claim update keeps the text and the focus of a YAML editor after an edit that keeps its value", async () => {
    const affinityOf = (pool: string) => ({
      zeebe: {
        replicas: 3,
        scheduling: {
          nodeAffinity: {
            requiredDuringSchedulingIgnoredDuringExecution: {
              nodeSelectorTerms: [
                {
                  matchExpressions: [
                    { key: "pool", operator: "In", values: [pool] },
                  ],
                },
              ],
            },
          },
        },
      },
      gateway: { mode: "Standalone" },
    });
    const contextAt = (version: string, pool: string) =>
      contextOf({
        openId: "zeebe",
        cluster: cluster(affinityOf(pool)),
        claim: claim(version, { cluster: affinityOf(pool) }),
      });
    const { update } = renderSlot(contextAt("41", "camunda"));
    await openEditors();
    const replicas = within(settingRow(group("Brokers"), "replicas")).getByRole(
      "spinbutton",
    );
    await userEvent.clear(replicas);
    await userEvent.type(replicas, "5");
    const affinity = within(
      settingRow(group("Brokers"), "scheduling.nodeAffinity"),
    ).getByRole("textbox");
    await userEvent.type(affinity, "{Enter}# a note");
    update(contextAt("42", "batch"));
    expect(affinity.isConnected).toBe(true);
    expect(document.activeElement).toBe(affinity);
    expect(affinity).toHaveProperty(
      "value",
      expect.stringContaining("# a note"),
    );
  });

  it("a claim update keeps a variable that the user added and that is not valid yet", async () => {
    const envOf = (env: object[]) => ({
      zeebe: { replicas: 3, extraEnv: env },
      gateway: { mode: "Standalone" },
    });
    const contextAt = (version: string, env: object[]) =>
      contextOf({
        openId: "zeebe",
        cluster: cluster(envOf(env)),
        claim: claim(version, { cluster: envOf(env) }),
      });
    const { update } = renderSlot(
      contextAt("41", [{ name: "LOG_LEVEL", value: "info" }]),
    );
    await openEditors();
    const replicas = within(settingRow(group("Brokers"), "replicas")).getByRole(
      "spinbutton",
    );
    await userEvent.clear(replicas);
    await userEvent.type(replicas, "5");
    const env = () => settingRow(group("Brokers"), "extraEnv");
    await userEvent.click(
      within(env()).getByRole("button", { name: "Add a variable" }),
    );
    update(contextAt("42", [{ name: "LOG_LEVEL", value: "debug" }]));
    expect(
      within(env()).getByRole("group", { name: "Variable 2" }),
    ).toBeTruthy();
    expect(
      within(
        within(env()).getByRole("group", { name: "Variable 1" }),
      ).getByRole("textbox", { name: "Value" }),
    ).toHaveProperty("value", "info");
  });

  it("a smaller pending size still says volumes cannot shrink when the editors open again", async () => {
    const spec = {
      zeebe: { replicas: 3, storageSize: "32Gi" },
      gateway: { mode: "Standalone" },
    };
    renderSlot(
      contextOf({
        openId: "zeebe",
        cluster: cluster(spec),
        claim: claim("41", { cluster: spec }),
      }),
    );
    await openEditors();
    const amount = () =>
      within(settingRow(group("Brokers"), "storageSize")).getByRole("textbox", {
        name: "Amount",
      });
    await userEvent.clear(amount());
    await userEvent.type(amount(), "16");
    expect(screen.getByText(SHRINK)).toBeDefined();
    await userEvent.click(
      screen.getByRole("button", { name: "Close the editors" }),
    );
    await openEditors();
    expect(amount()).toHaveProperty("value", "16");
    expect(screen.getByText(SHRINK)).toBeDefined();
  });

  it("a reset of a block shows the preset in each editor of the block", async () => {
    const spec = {
      zeebe: {
        replicas: 3,
        scheduling: {
          nodeAffinity: {
            requiredDuringSchedulingIgnoredDuringExecution: {
              nodeSelectorTerms: [
                {
                  matchExpressions: [
                    { key: "pool", operator: "In", values: ["camunda"] },
                  ],
                },
              ],
            },
          },
          tolerations: [{ key: "dedicated", operator: "Exists" }],
        },
      },
      gateway: { mode: "Standalone" },
    };
    renderSlot(
      contextOf({
        openId: "zeebe",
        cluster: cluster(spec),
        claim: claim("41", { cluster: spec }),
      }),
    );
    await openEditors();
    const affinity = () =>
      within(settingRow(group("Brokers"), "scheduling.nodeAffinity")).getByRole(
        "textbox",
      );
    expect(affinity()).toHaveProperty("value", expect.stringContaining("pool"));
    await userEvent.click(
      within(settingRow(group("Brokers"), "scheduling.tolerations")).getByRole(
        "button",
        { name: "Reset scheduling to preset" },
      ),
    );
    expect(claimSpec().cluster.zeebe.scheduling).toBeUndefined();
    expect(affinity()).toHaveProperty("value", "");
  });

  it("shows nothing without an open component", () => {
    renderSlot(contextOf({ openId: undefined }));
    expect(screen.queryByRole("button", { name: "Edit settings" })).toBeNull();
  });
});
