// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyClaimChange,
  previewClaimChange,
} from "@/app/fleet/clusters/[namespace]/[name]/actions";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { FakeEventSource } from "../../../../test/fakes/event-source";
import { claim, slotContext } from "../../../../test/fakes/slot-context";
import { fakeWriter } from "../../../../test/fakes/writer";
import type { ClusterSlotContext } from "../slots";
import type { ClaimPanel } from "../tabs";
import { ClaimEditorPanels } from "./claim-editor";
import { ClusterEditProvider, useClusterEdit } from "./cluster-edit";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));

const actions = { preview: previewClaimChange, apply: applyClaimChange };

const access = {
  kind: "PluginSection",
  metadata: { name: "access" },
  spec: {
    label: "Access",
    selection: "One",
    appliesTo: ["CamundaClusterClaim"],
  },
} as unknown as Resource<"PluginSection">;

const gateway = (deletionPolicies: string[]) =>
  ({
    kind: "PluginRegistration",
    metadata: { name: "gateway-access" },
    spec: { label: "Gateway", deletionPolicies },
  }) as unknown as Resource<"PluginRegistration">;

// contextOf gives the claim orders the access option Gateway, whose
// registration lists policies, and the resolved deletion policy fallback.
function contextOf(
  policies: string[],
  fallback?: "Delete" | "Orphan",
  spec: object = {},
): ClusterSlotContext {
  return slotContext({
    claim: {
      ...claim("41", {
        plugins: [{ section: "access", registration: "gateway-access" }],
        ...spec,
      }),
      status: {
        resolvedPolicy: {
          sections: { access: { allowed: ["gateway-access"] } },
          ...(fallback && { deletionPolicy: fallback }),
        },
      },
    } as unknown as Resource<"CamundaClusterClaim">,
    sections: [access],
    registrations: [gateway(policies)],
  });
}

// Suspend makes a change outside the panel, so that the change bar shows.
function Suspend() {
  const { set } = useClusterEdit();
  return (
    <button
      type="button"
      onClick={() => set([{ field: "spec" }, { field: "suspend" }], true)}
    >
      Suspend
    </button>
  );
}

// Probe makes the change of its label outside the panel.
function Probe({
  label,
  steps,
  value,
}: {
  label: string;
  steps: Step[];
  value: unknown;
}) {
  const { set } = useClusterEdit();
  return (
    <button type="button" onClick={() => set(steps, value)}>
      {label}
    </button>
  );
}

function panel(
  context: ClusterSlotContext,
  requested: ClaimPanel = "deletion",
) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        <ClusterEditProvider context={context} actions={actions}>
          <ClaimEditorPanels context={context} requested={requested} />
          <Suspend />
          <Probe
            label="Add an option"
            steps={[{ field: "spec" }, { field: "plugins" }]}
            value={[
              { section: "access", registration: "gateway-access" },
              { section: "backup", registration: "delete-only" },
            ]}
          />
          <Probe
            label="Delete"
            steps={[{ field: "spec" }, { field: "deletionPolicy" }]}
            value="Delete"
          />
        </ClusterEditProvider>
      </GitPushesProvider>
    </QueryClientProvider>,
  );
}

const deletion = () => screen.getByRole("region", { name: "Deletion policy" });
const radio = (name: RegExp) =>
  within(deletion()).getByRole<HTMLInputElement>("radio", { name });
const KEEP = /^Keep the resources/;
const DELETE = /^Delete the resources/;
const bar = () => screen.queryByRole("region", { name: "Unsaved changes" });
const barButton = (name: string) =>
  within(bar()!).getByRole<HTMLButtonElement>("button", { name });
const refusal = (registration: string, section: string) =>
  `the deletion policy is Orphan, and these registrations do not support it: PluginRegistration "${registration}" of section "${section}"`;
const REFUSED =
  "The claim keeps its resources, but an option below cannot keep them. Choose Delete, or remove that option.";

beforeEach(() => {
  Element.prototype.scrollIntoView = () => {};
  fakeWriter.reset();
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("the Deletion policy panel", () => {
  it("Keep the resources is disabled with the reason when an option does not support Orphan", () => {
    panel(contextOf(["Delete"]));
    expect(radio(KEEP).disabled).toBe(true);
    expect(
      within(deletion()).getByText(
        "Some options always delete what they made.",
      ),
    ).toBeTruthy();
    expect(within(deletion()).getByText("Gateway (Access)")).toBeTruthy();
  });

  it("labels the default policy of the claim Default", () => {
    panel(contextOf(["Delete", "Orphan"], "Orphan"));
    expect(radio(KEEP).checked).toBe(true);
    expect(radio(KEEP).closest("label")?.textContent).toContain("Default");
    expect(radio(DELETE).closest("label")?.textContent).not.toContain(
      "Default",
    );
  });

  it("the default that the claim recorded wins over the default of the environment", () => {
    const context = contextOf(["Delete", "Orphan"], "Delete");
    context.environment!.spec.deletionPolicy = "Orphan";
    panel(context);
    expect(radio(DELETE).checked).toBe(true);
    expect(radio(DELETE).closest("label")?.textContent).toContain("Default");
    expect(radio(KEEP).closest("label")?.textContent).not.toContain("Default");
  });

  it("a claim that keeps its resources with an option that cannot keep them shows Keep and blocks Review", () => {
    panel(contextOf(["Delete"], "Orphan"));
    expect(radio(KEEP).checked).toBe(true);
    expect(radio(KEEP).disabled).toBe(true);
    expect(radio(DELETE).checked).toBe(false);
    expect(radio(DELETE).disabled).toBe(false);
    expect(within(deletion()).getByText(REFUSED)).toBeTruthy();
    expect(bar()).toBeNull();
    expect(fakeWriter.applied).toEqual([]);
    fireEvent.click(screen.getByRole("button", { name: "Suspend" }));
    expect(barButton("Review").disabled).toBe(true);
    expect(bar()!.textContent).toContain(refusal("gateway-access", "access"));
  });

  it("choosing Delete for a refused Keep makes one change that writes Delete", async () => {
    const context = contextOf(["Delete"], "Orphan");
    panel(context);
    fireEvent.click(radio(DELETE));
    expect(within(deletion()).queryByText(REFUSED)).toBeNull();
    expect(barButton("Review").disabled).toBe(false);
    fireEvent.click(barButton("Review"));
    const save = within(
      screen.getByRole("dialog", { name: "Review the change" }),
    ).getByRole<HTMLButtonElement>("button", { name: "Save" });
    await waitFor(() => expect(save.disabled).toBe(false));
    fireEvent.click(save);
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    expect(
      (fakeWriter.applied[0].apply[0] as Resource<"CamundaClusterClaim">).spec,
    ).toEqual({ ...context.claim!.spec, deletionPolicy: "Delete" });
  });

  it("Discard returns a refused Keep to the blocked state without a write", () => {
    panel(contextOf(["Delete"], "Orphan"));
    fireEvent.click(radio(DELETE));
    fireEvent.click(barButton("Discard"));
    expect(radio(KEEP).checked).toBe(true);
    expect(within(deletion()).getByText(REFUSED)).toBeTruthy();
    expect(fakeWriter.applied).toEqual([]);
    fireEvent.click(screen.getByRole("button", { name: "Suspend" }));
    expect(barButton("Review").disabled).toBe(true);
  });

  it("with the panel closed, a change that adds an option that cannot keep its resources blocks Review", () => {
    const context = contextOf(["Delete", "Orphan"], "Orphan");
    context.claim!.status!.resolvedPolicy!.sections!.backup = {
      allowed: ["delete-only"],
    };
    context.sections = [
      ...context.sections,
      {
        kind: "PluginSection",
        metadata: { name: "backup" },
        spec: {
          label: "Backup",
          selection: "One",
          appliesTo: ["CamundaClusterClaim"],
        },
      } as unknown as Resource<"PluginSection">,
    ];
    context.registrations = [
      ...context.registrations,
      {
        kind: "PluginRegistration",
        metadata: { name: "delete-only" },
        spec: { label: "Delete only", deletionPolicies: ["Delete"] },
      } as unknown as Resource<"PluginRegistration">,
    ];
    panel(context, "release");
    expect(within(deletion()).queryByRole("radio")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Add an option" }));
    expect(barButton("Review").disabled).toBe(true);
    expect(bar()!.textContent).toContain(refusal("delete-only", "backup"));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(bar()!.textContent).not.toContain("the deletion policy is Orphan");
    expect(barButton("Review").disabled).toBe(false);
  });

  it("takes Delete as the default when the resolved policy names none", () => {
    panel(contextOf(["Delete", "Orphan"]));
    expect(radio(DELETE).checked).toBe(true);
    expect(radio(DELETE).closest("label")?.textContent).toContain("Default");
  });

  it("choosing Keep writes Orphan", async () => {
    panel(contextOf(["Delete", "Orphan"]));
    fireEvent.click(radio(KEEP));
    fireEvent.click(within(bar()!).getByRole("button", { name: "Review" }));
    const save = within(
      screen.getByRole("dialog", { name: "Review the change" }),
    ).getByRole<HTMLButtonElement>("button", { name: "Save" });
    await waitFor(() => expect(save.disabled).toBe(false));
    fireEvent.click(save);
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    expect(
      (fakeWriter.applied[0].apply[0] as Resource<"CamundaClusterClaim">).spec
        .deletionPolicy,
    ).toBe("Orphan");
  });

  it("choosing the default again drops the change", () => {
    panel(contextOf(["Delete", "Orphan"]));
    fireEvent.click(radio(KEEP));
    expect(bar()).toBeTruthy();
    fireEvent.click(radio(DELETE));
    expect(bar()).toBeNull();
  });
});
