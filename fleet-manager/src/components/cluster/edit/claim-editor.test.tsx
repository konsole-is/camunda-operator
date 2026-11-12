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
import { ClaimEditor } from "./claim-editor";
import { ClusterEditProvider } from "./cluster-edit";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(window.location.search),
}));

const actions = { preview: previewClaimChange, apply: applyClaimChange };

function editor(context: ClusterSlotContext = slotContext()) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        <ClusterEditProvider context={context} actions={actions}>
          <ClaimEditor context={context} />
        </ClusterEditProvider>
      </GitPushesProvider>
    </QueryClientProvider>,
  );
}

const region = (name: string) => screen.getByRole("region", { name });
const editButton = (panel: string) =>
  screen.queryByRole("button", { name: `Edit ${panel}` });

async function save() {
  fireEvent.click(
    within(region("Unsaved changes")).getByRole("button", { name: "Review" }),
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
  fakeWriter.reset();
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
  window.history.replaceState(null, "", "?tab=claim");
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("ClaimEditor", () => {
  it("shows the read view with an Edit button on each panel that has an editor", () => {
    editor();
    expect(within(region("Basics")).getByText("orders")).toBeTruthy();
    for (const panel of [
      "Basics",
      "Release and preset",
      "Attachments",
      "Deletion policy",
    ])
      expect(
        within(region(panel)).getByRole("button", { name: `Edit ${panel}` }),
      ).toBeTruthy();
    for (const panel of ["Conditions", "Requests"])
      expect(
        within(region(panel)).queryByRole("button", { name: /^Edit/ }),
      ).toBeNull();
  });

  it("opening a panel closes the one in edit and keeps its changes", () => {
    editor();
    fireEvent.click(editButton("Basics")!);
    fireEvent.click(
      within(region("Basics")).getByRole("checkbox", {
        name: "Suspend the cluster",
      }),
    );
    fireEvent.click(editButton("Deletion policy")!);
    expect(editButton("Basics")).toBeTruthy();
    expect(
      within(region("Deletion policy")).getByRole("button", { name: "Done" }),
    ).toBeTruthy();
    expect(
      within(region("Unsaved changes")).getByText("1 unsaved change"),
    ).toBeTruthy();
  });

  it("opens the Attachments panel from the link of Change schedule", () => {
    Element.prototype.scrollIntoView = vi.fn();
    window.history.replaceState(null, "", "?tab=claim&section=attachments");
    editor();
    expect(
      within(region("Attachments")).getByRole("checkbox", {
        name: "Back up on a schedule",
      }),
    ).toBeTruthy();
  });

  it("Basics shows the name, the namespace, and the environment locked Set at creation", () => {
    editor(
      slotContext({
        claim: {
          ...claim(),
          status: { environment: "prod" },
        } as Resource<"CamundaClusterClaim">,
      }),
    );
    fireEvent.click(editButton("Basics")!);
    const basics = region("Basics");
    for (const [label, value] of [
      ["Name", "orders"],
      ["Namespace", "team-a"],
      ["Environment", "Production"],
    ]) {
      const group = within(basics).getByRole("group", { name: label });
      expect(within(group).getByText(value)).toBeTruthy();
      expect(within(group).getByText("Set at creation")).toBeTruthy();
    }
  });

  it("turning Suspend on writes spec.suspend and keeps the rest of the claim", async () => {
    editor();
    fireEvent.click(editButton("Basics")!);
    fireEvent.click(
      within(region("Basics")).getByRole("checkbox", {
        name: "Suspend the cluster",
      }),
    );
    const written = await save();
    expect(written.spec).toEqual({ ...claim().spec, suspend: true });
  });

  it("turning Suspend off again leaves nothing to review", () => {
    editor();
    fireEvent.click(editButton("Basics")!);
    const box = within(region("Basics")).getByRole("checkbox", {
      name: "Suspend the cluster",
    });
    fireEvent.click(box);
    fireEvent.click(box);
    expect(
      screen.queryByRole("region", { name: "Unsaved changes" }),
    ).toBeNull();
  });

  it("Done closes the panel and keeps its pending changes in the read view", () => {
    editor();
    fireEvent.click(editButton("Basics")!);
    fireEvent.click(
      within(region("Basics")).getByRole("checkbox", {
        name: "Suspend the cluster",
      }),
    );
    fireEvent.click(
      within(region("Basics")).getByRole("button", { name: "Done" }),
    );
    expect(editButton("Basics")).toBeTruthy();
    expect(within(region("Basics")).getByText("Yes")).toBeTruthy();
    expect(
      within(region("Unsaved changes")).getByText("1 unsaved change"),
    ).toBeTruthy();
  });

  it("opens and shows the panel that the section query parameter names", () => {
    const scroll = vi.fn();
    Element.prototype.scrollIntoView = scroll;
    window.history.replaceState(null, "", "?tab=claim&section=basics");
    editor();
    expect(
      within(region("Basics")).getByRole("button", { name: "Done" }),
    ).toBeTruthy();
    expect(scroll).toHaveBeenCalledOnce();
    expect(scroll.mock.contexts[0]).toBe(region("Basics"));
  });

  it("shows the read view without Edit buttons when the page has no session", () => {
    editor(slotContext({ writeMode: undefined }));
    expect(within(region("Basics")).getByText("orders")).toBeTruthy();
    expect(screen.queryByRole("button", { name: /^Edit/ })).toBeNull();
  });
});
