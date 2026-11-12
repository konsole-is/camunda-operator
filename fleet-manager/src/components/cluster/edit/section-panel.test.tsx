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
import userEvent from "@testing-library/user-event";
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

const harness = {
  kind: "PluginRegistration",
  metadata: { name: "fleet-harness" },
  spec: {
    label: "Fleet harness",
    deletionPolicies: ["Delete", "Orphan"],
    parameters: {
      type: "object",
      properties: {
        hostPrefix: { type: "string" },
        zone: { type: "string" },
        tier: { type: "string" },
      },
    },
  },
} as unknown as Resource<"PluginRegistration">;

// contextOf gives the claim orders the access option fleet-harness, whose
// hostPrefix the claim sets and may change, whose zone the admin sets, and
// whose tier is a choice of the environment.
function contextOf(): ClusterSlotContext {
  const plugins = [
    {
      section: "access",
      registration: "fleet-harness",
      parameters: { hostPrefix: "orders", tier: "gold" },
    },
  ];
  return slotContext({
    claim: {
      ...claim("41", { plugins }),
      status: {
        resolvedPolicy: {
          sections: { access: { allowed: ["fleet-harness"] } },
          parameters: {
            "fleet-harness": {
              hostPrefix: { claimEditable: true, value: "default" },
              zone: { value: "eu-west-1a" },
              tier: {
                claimEditable: true,
                allowed: [
                  { name: "silver", label: "Silver", value: { size: 1 } },
                  { name: "gold", label: "Gold", value: { size: 2 } },
                ],
              },
            },
          },
        },
      },
    } as unknown as Resource<"CamundaClusterClaim">,
    sections: [access],
    registrations: [harness],
  });
}

function panel(context: ClusterSlotContext = contextOf()) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        <ClusterEditProvider context={context} actions={actions}>
          <ClaimEditorPanels context={context} requested="section:access" />
        </ClusterEditProvider>
      </GitPushesProvider>
    </QueryClientProvider>,
  );
}

const sectionPanel = () =>
  screen.getByRole("region", { name: "Access: Fleet harness" });
const row = (label: string) =>
  within(sectionPanel()).getAllByRole("group", { name: label })[0];

async function save() {
  fireEvent.click(
    within(screen.getByRole("region", { name: "Unsaved changes" })).getByRole(
      "button",
      { name: "Review" },
    ),
  );
  const button = within(
    screen.getByRole("dialog", { name: "Review the change" }),
  ).getByRole<HTMLButtonElement>("button", { name: "Save" });
  await waitFor(() => expect(button.disabled).toBe(false));
  fireEvent.click(button);
  await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
  return fakeWriter.applied[0].apply[0] as Resource<"CamundaClusterClaim">;
}

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

describe("a section panel", () => {
  it("shows the option locked Chosen at creation", () => {
    panel();
    const option = row("Option");
    expect(within(option).getByText("Fleet harness")).toBeTruthy();
    expect(within(option).getByText("Chosen at creation")).toBeTruthy();
  });

  it("a parameter change writes only the parameter", async () => {
    panel();
    await userEvent.type(within(row("hostPrefix")).getByRole("textbox"), "-eu");
    const written = await save();
    const before = contextOf().claim!.spec;
    expect(written.spec).toEqual({
      ...before,
      plugins: [
        {
          ...before.plugins![0],
          parameters: { hostPrefix: "orders-eu", tier: "gold" },
        },
      ],
    });
  });

  it("Discard shows the values of the claim again in the parameter editors", async () => {
    panel();
    const input = () =>
      within(row("hostPrefix")).getByRole<HTMLInputElement>("textbox");
    await userEvent.type(input(), "-eu");
    fireEvent.click(
      within(screen.getByRole("region", { name: "Unsaved changes" })).getByRole(
        "button",
        { name: "Discard" },
      ),
    );
    expect(input().value).toBe("orders");
  });

  it("an edit that the user takes back keeps the editor and its focus", async () => {
    panel();
    const input = within(row("hostPrefix")).getByRole<HTMLInputElement>(
      "textbox",
    );
    await userEvent.type(input, "x");
    await userEvent.type(input, "{Backspace}");
    expect(
      screen.queryByRole("region", { name: "Unsaved changes" }),
    ).toBeNull();
    expect(input.isConnected).toBe(true);
    expect(document.activeElement).toBe(input);
  });

  it("a parameter that the admin set stays locked", () => {
    panel();
    const zone = row("zone");
    expect(within(zone).getByText("eu-west-1a")).toBeTruthy();
    expect(within(zone).getByText("Set by your admin")).toBeTruthy();
    expect(within(zone).queryByRole("textbox")).toBeNull();
  });

  it("a parameter with choices picks a choice by its name", async () => {
    panel();
    const select = within(row("tier")).getByRole<HTMLSelectElement>("combobox");
    expect(select.value).toBe("gold");
    fireEvent.change(select, { target: { value: "silver" } });
    const written = await save();
    expect(written.spec.plugins![0].parameters).toEqual({
      hostPrefix: "orders",
      tier: "silver",
    });
  });

  it("says when no parameter can change", () => {
    const context = contextOf();
    context.claim!.status!.resolvedPolicy!.parameters!["fleet-harness"] = {
      zone: { value: "eu-west-1a" },
    };
    context.claim!.spec.plugins![0].parameters = {};
    panel(context);
    expect(
      within(sectionPanel()).getByText("No parameters you can change."),
    ).toBeTruthy();
    expect(within(sectionPanel()).queryByRole("textbox")).toBeNull();
  });
});
