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
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyClaimChange,
  previewClaimChange,
} from "@/app/fleet/clusters/[namespace]/[name]/actions";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { withValue } from "@/lib/session";
import { FakeEventSource } from "../../../test/fakes/event-source";
import { claim, cluster, slotContext } from "../../../test/fakes/slot-context";
import { fakeWriter } from "../../../test/fakes/writer";
import type { BackupObject } from "./backups";
import { ClusterEditProvider, useClusterEdit } from "./edit/cluster-edit";
import type { ClusterSlotContext } from "./slots";
import { modeSteps, type TopologyChange } from "./topology-change";
import { TopologyChangePanel, TopologyEditor } from "./topology-editor";

vi.mock("@/lib/write/writer", () => import("../../../test/fakes/writer"));

const actions = { preview: previewClaimChange, apply: applyClaimChange };

// Other is a second editor of the page, which sets a field that is not a
// mode.
function Other() {
  const edit = useClusterEdit();
  return (
    <button
      onClick={() =>
        edit.set(
          [
            { field: "spec" },
            { field: "cluster" },
            { field: "zeebe" },
            { field: "replicas" },
          ],
          5,
        )
      }
    >
      Scale
    </button>
  );
}

function editor(
  context: ClusterSlotContext = slotContext(),
  extra?: ReactNode,
) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        <ClusterEditProvider context={context} actions={actions}>
          <TopologyEditor context={context} />
          {extra}
        </ClusterEditProvider>
      </GitPushesProvider>
    </QueryClientProvider>,
  );
}

// withSpec gives the claim and the live cluster the same cluster spec,
// with the three brokers of the fixture.
function withSpec(spec: object): ClusterSlotContext {
  return slotContext({
    cluster: cluster(spec),
    claim: claim("41", { cluster: { zeebe: { replicas: 3 }, ...spec } }),
  });
}

const running = {
  kind: "LogicalBackupRDBMS",
  metadata: { namespace: "team-a", name: "nightly" },
  spec: { clusterRef: { name: "orders" } },
  status: { phase: "Running" },
} as unknown as BackupObject;

const open = () =>
  fireEvent.click(screen.getByRole("button", { name: "Change topology" }));
const close = () =>
  fireEvent.click(
    within(editorDialog()).getByRole("button", { name: "Close" }),
  );
const editorDialog = () =>
  screen.getByRole("dialog", { name: "Change topology" });
const pick = (process: string, mode: string) =>
  fireEvent.click(
    within(screen.getByRole("group", { name: process })).getByRole("radio", {
      name: mode,
    }),
  );
const lines = () =>
  within(screen.getByRole("region", { name: "What happens" }))
    .getAllByRole("listitem")
    .map((li) => li.textContent);
const reviewButton = () =>
  within(editorDialog()).getByRole<HTMLButtonElement>("button", {
    name: "Review",
  });
const reviewDialog = () =>
  screen.getByRole("dialog", { name: "Review the change" });

beforeEach(() => {
  fakeWriter.reset();
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("TopologyEditor", () => {
  it("lists the gateway and the web apps with a mode and Connectors without one", () => {
    editor(withSpec({ connectors: { enabled: true } }));
    open();
    for (const name of ["Gateway", "Operate", "Tasklist", "Admin"])
      expect(
        within(screen.getByRole("group", { name })).getAllByRole("radio"),
      ).toHaveLength(2);
    expect(screen.queryByRole("group", { name: "Connectors" })).toBeNull();
    for (const column of ["Now", "After the change"])
      expect(
        within(screen.getByRole("list", { name: column })).getByText(
          "Connectors",
        ),
      ).toBeTruthy();
  });

  it("shows the impact lines of the picked switch, with the count of prepared settings", () => {
    editor(
      withSpec({
        operate: { replicas: 2, resources: { requests: { cpu: "1" } } },
      }),
    );
    open();
    expect(screen.getByText("Nothing changes yet.")).toBeTruthy();
    pick("Operate", "Standalone");
    expect(lines()).toEqual([
      "Creates Deployment orders-operate and Service orders-operate.",
      "Operate starts with the settings prepared in the claim (2 fields).",
      "Gateway pods restart.",
    ]);
  });

  it("writes only the mode fields that changed", async () => {
    editor();
    open();
    pick("Gateway", "Embedded");
    pick("Operate", "Standalone");
    pick("Operate", "Embedded");
    fireEvent.click(reviewButton());
    const save = within(reviewDialog()).getByRole<HTMLButtonElement>("button", {
      name: "Save",
    });
    await waitFor(() => expect(save.disabled).toBe(false));
    fireEvent.click(save);
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    expect(fakeWriter.applied[0].apply).toEqual([
      withValue(claim(), modeSteps("gateway"), "Embedded"),
    ]);
    expect(fakeWriter.applied[0].preconditions).toEqual({
      "CamundaClusterClaim/team-a/orders": "41",
    });
  });

  it("picking a mode and back leaves nothing to review", () => {
    editor();
    open();
    pick("Tasklist", "Standalone");
    pick("Tasklist", "Embedded");
    expect(screen.getByText("Nothing changes yet.")).toBeTruthy();
    expect(reviewButton().disabled).toBe(true);
  });

  it("keeps the picks as pending changes of the page when it closes", () => {
    editor();
    open();
    pick("Gateway", "Embedded");
    close();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(
      screen.getByRole("region", { name: "Unsaved changes" }).textContent,
    ).toContain("1 unsaved change");
    open();
    expect(
      within(
        screen.getByRole("group", { name: "Gateway" }),
      ).getByRole<HTMLInputElement>("radio", { name: "Embedded" }).checked,
    ).toBe(true);
  });

  it("says when the lines include other pending changes", () => {
    const note =
      "These lines include the other pending changes of this cluster.";
    editor(slotContext(), <Other />);
    open();
    pick("Gateway", "Embedded");
    expect(screen.queryByText(note)).toBeNull();
    close();
    fireEvent.click(screen.getByRole("button", { name: "Scale" }));
    open();
    expect(screen.getByText(note)).toBeTruthy();
  });

  it("shows the backup caution in the review of a mode switch during a running backup", () => {
    editor(slotContext({ backups: [running] }));
    open();
    pick("Gateway", "Embedded");
    fireEvent.click(reviewButton());
    expect(
      within(reviewDialog()).getByRole("note", { name: "Caution" }).textContent,
    ).toContain("Wait until backup nightly finishes.");
  });

  it("shows no caution in the review of a mode switch that keeps the brokers during a running backup", () => {
    editor(slotContext({ backups: [running] }));
    open();
    pick("Operate", "Standalone");
    fireEvent.click(reviewButton());
    expect(
      within(reviewDialog()).queryByRole("note", { name: "Caution" }),
    ).toBeNull();
    expect(
      within(reviewDialog()).getByText(
        /This change does not roll the brokers, so it does not affect the backup\./,
      ),
    ).toBeTruthy();
  });

  it("renders nothing without a session", () => {
    editor(slotContext({ claim: undefined }));
    expect(
      screen.queryByRole("button", { name: "Change topology" }),
    ).toBeNull();
  });
});

describe("TopologyChangePanel", () => {
  const change: TopologyChange = {
    before: [],
    after: [],
    marks: { before: {}, after: {} },
    modes: [
      {
        process: "operate",
        label: "Operate",
        before: "Embedded",
        after: "Embedded",
        host: "gateway",
        lock: { reason: "admin" },
      },
      {
        process: "tasklist",
        label: "Tasklist",
        before: "Embedded",
        after: "Standalone",
        host: "gateway",
      },
    ],
  };

  it("shows a locked mode with its reason and no choice", () => {
    render(
      <TopologyChangePanel
        change={change}
        impacts={[]}
        others={false}
        onPick={() => {}}
      />,
    );
    const operate = screen.getByRole("group", { name: "Operate" });
    expect(within(operate).queryByRole("radio")).toBeNull();
    expect(within(operate).getByText("Set by your admin")).toBeTruthy();
  });

  it("gives the hint of the picked mode", () => {
    render(
      <TopologyChangePanel
        change={{
          ...change,
          modes: [
            { ...change.modes[1], after: "Embedded" },
            {
              ...change.modes[1],
              process: "admin",
              label: "Admin",
              host: "zeebe",
            },
          ],
        }}
        impacts={[]}
        others={false}
        onPick={() => {}}
      />,
    );
    expect(
      within(screen.getByRole("group", { name: "Tasklist" })).getByText(
        "Runs in the gateway.",
      ),
    ).toBeTruthy();
    expect(
      within(screen.getByRole("group", { name: "Admin" })).getByText(
        "Own Deployment and Service.",
      ),
    ).toBeTruthy();
  });

  it("calls onPick with the process and the mode", () => {
    const onPick = vi.fn();
    render(
      <TopologyChangePanel
        change={change}
        impacts={[]}
        others={false}
        onPick={onPick}
      />,
    );
    pick("Tasklist", "Embedded");
    expect(onPick).toHaveBeenCalledWith("tasklist", "Embedded");
  });

  it("shows a refused pick with the reason of its lock", () => {
    render(
      <TopologyChangePanel
        change={change}
        impacts={[]}
        others={false}
        refused={{ process: "tasklist", lock: { reason: "release" } }}
        onPick={() => {}}
      />,
    );
    expect(screen.getByRole("alert").textContent).toContain(
      "Set by the release",
    );
  });
});
