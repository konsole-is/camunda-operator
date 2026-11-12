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

function panel(context: ClusterSlotContext = slotContext()) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        <ClusterEditProvider context={context} actions={actions}>
          <ClaimEditorPanels context={context} requested="release" />
        </ClusterEditProvider>
      </GitPushesProvider>
    </QueryClientProvider>,
  );
}

const releasePanel = () =>
  screen.getByRole("region", { name: "Release and preset" });
const select = (label: string) =>
  within(releasePanel()).getByLabelText<HTMLSelectElement>(label);
const choose = (label: string, value: string) =>
  fireEvent.change(select(label), { target: { value } });
const bar = () => screen.queryByRole("region", { name: "Unsaved changes" });
const dialog = () => screen.getByRole("dialog", { name: "Review the change" });

function openReview() {
  fireEvent.click(within(bar()!).getByRole("button", { name: "Review" }));
}

async function confirm(label: string) {
  const button = within(dialog()).getByRole<HTMLButtonElement>("button", {
    name: label,
  });
  await waitFor(() => expect(button.disabled).toBe(false));
  fireEvent.click(button);
  await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
  return (fakeWriter.applied[0].apply[0] as Resource<"CamundaClusterClaim">)
    .spec;
}

const preset = (name: string, replicas: number) =>
  ({
    kind: "CamundaClusterPreset",
    metadata: { name },
    spec: { cluster: { tasklist: { replicas } } },
  }) as unknown as Resource<"CamundaClusterPreset">;

const platformConfig = (name: string) =>
  ({
    kind: "CamundaPlatformConfig",
    metadata: { name },
    spec: {},
  }) as unknown as Resource<"CamundaPlatformConfig">;

beforeEach(() => {
  // jsdom has no layout, so it has no scrollIntoView.
  Element.prototype.scrollIntoView = () => {};
  fakeWriter.reset();
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("the Release and preset panel", () => {
  it("a downgrade says so and writes releaseRef and allowVersionDowngrade", async () => {
    panel();
    choose("Release", "camunda-8-9-0");
    expect(
      within(releasePanel()).getByText(
        "8.9.1 to 8.9.0 is a downgrade. The review asks you to confirm it.",
      ),
    ).toBeTruthy();
    openReview();
    const spec = await confirm("Downgrade");
    expect(spec.releaseRef).toBe("camunda-8-9-0");
    expect(spec.allowVersionDowngrade).toBe("8.9.0");
  });

  it("moving off a downgrade drops allowVersionDowngrade again", () => {
    panel();
    choose("Release", "camunda-8-9-0");
    choose("Release", "camunda-8-9-1");
    expect(bar()).toBeNull();
  });

  it("marks the default release and the downgrade in the options", () => {
    panel();
    const labels = [...select("Release").options].map((o) => o.textContent);
    expect(labels).toEqual([
      "8.9.1 (camunda-8-9-1), Default",
      "8.9.0 (camunda-8-9-0), downgrade",
    ]);
  });

  it("a preset change shows the merged fields that change", () => {
    const presets = [preset("medium", 1), preset("large", 2)];
    const context = slotContext({ presets });
    context.environment!.spec.presets = { camundaCluster: ["medium", "large"] };
    panel(context);
    choose("Preset", "large");
    openReview();
    expect(dialog().textContent).toContain("spec.tasklist.replicas: 1 to 2.");
    expect(
      within(dialog()).getByRole("figure", { name: "Changes to the cluster" }),
    ).toBeTruthy();
  });

  it("offers only the presets that the environment allows, and the current one", () => {
    const presets = [preset("medium", 1), preset("large", 2), preset("xl", 3)];
    const context = slotContext({ presets });
    context.environment!.spec.presets = { camundaCluster: ["large"] };
    panel(context);
    expect([...select("Preset").options].map((o) => o.value)).toEqual([
      "large",
      "medium",
    ]);
  });

  it("a platform config change writes platformConfigRef, and the default option resets it", async () => {
    const context = slotContext({
      platformConfigs: [platformConfig("base"), platformConfig("strict")],
    });
    context.environment!.spec.platformConfigRef = "base";
    panel(context);
    expect(
      [...select("Platform config").options].map((o) => o.textContent),
    ).toEqual(["Default of the environment", "base", "strict"]);
    choose("Platform config", "strict");
    choose("Platform config", "");
    expect(bar()).toBeNull();
    choose("Platform config", "strict");
    openReview();
    expect((await confirm("Save")).platformConfigRef).toBe("strict");
  });

  it("Clear writes the reset of allowVersionDowngrade when nothing else is pending", async () => {
    panel(
      slotContext({ claim: claim("41", { allowVersionDowngrade: "8.9.1" }) }),
    );
    expect(
      within(releasePanel()).getByText(
        "The cluster runs 8.9.1, so allowVersionDowngrade can go.",
      ),
    ).toBeTruthy();
    fireEvent.click(
      within(releasePanel()).getByRole("button", { name: "Clear" }),
    );
    const spec = await confirm("Save");
    expect(spec.allowVersionDowngrade).toBeUndefined();
    expect(spec.releaseRef).toBe("camunda-8-9-1");
  });

  it("offers no Clear while another change is pending", () => {
    panel(
      slotContext({ claim: claim("41", { allowVersionDowngrade: "8.9.1" }) }),
    );
    choose("Preset", "medium");
    choose("Release", "camunda-8-9-0");
    expect(
      within(releasePanel()).queryByRole("button", { name: "Clear" }),
    ).toBeNull();
  });

  it("a release error shows under the release select", async () => {
    fakeWriter.results = [
      {
        ok: false,
        errors: [
          {
            code: "Invalid",
            message: "the release is refused",
            path: "spec.releaseRef",
          },
        ],
      },
    ];
    panel();
    choose("Release", "camunda-8-9-0");
    openReview();
    const button = within(dialog()).getByRole<HTMLButtonElement>("button", {
      name: "Downgrade",
    });
    await waitFor(() => expect(button.disabled).toBe(false));
    fireEvent.click(button);
    await waitFor(() =>
      expect(within(dialog()).getByRole("alert").textContent).toContain(
        "the release is refused",
      ),
    );
    fireEvent.click(within(dialog()).getByRole("button", { name: "Back" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(select("Release").getAttribute("aria-invalid")).toBe("true");
    expect(
      within(releasePanel()).getByText("the release is refused"),
    ).toBeTruthy();
  });
});
