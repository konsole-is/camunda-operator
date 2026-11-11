// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { secretRefsOf } from "@/components/fleet/secret-ref-picker";
import { clusterFields } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { claimAfter } from "@/lib/session";
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

function renderSlot(context: ClusterSlotContext) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        <ClusterEditProvider context={context} actions={actions}>
          <ComponentSettingsEditor ctx={context} />
          <Probe />
        </ClusterEditProvider>
      </GitPushesProvider>
    </QueryClientProvider>,
  );
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
  it("an embedded Tasklist field writes to spec.cluster.tasklist", async () => {
    renderSlot(contextOf({ openId: "gateway" }));
    await openEditors();
    const tasklist = group("Tasklist");
    expect(
      within(tasklist).getByText("Applies when Tasklist runs standalone."),
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

  it("shows nothing without an open component", () => {
    renderSlot(contextOf({ openId: undefined }));
    expect(screen.queryByRole("button", { name: "Edit settings" })).toBeNull();
  });
});
