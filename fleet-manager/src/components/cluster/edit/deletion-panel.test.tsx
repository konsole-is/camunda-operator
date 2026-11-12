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
import type { Resource } from "@/lib/k8s/kinds";
import { FakeEventSource } from "../../../../test/fakes/event-source";
import { claim, slotContext } from "../../../../test/fakes/slot-context";
import { fakeWriter } from "../../../../test/fakes/writer";
import type { ClusterSlotContext } from "../slots";
import { ClaimEditorPanels } from "./claim-editor";
import { ClusterEditProvider } from "./cluster-edit";

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

function panel(context: ClusterSlotContext) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        <ClusterEditProvider context={context} actions={actions}>
          <ClaimEditorPanels context={context} requested="deletion" />
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
