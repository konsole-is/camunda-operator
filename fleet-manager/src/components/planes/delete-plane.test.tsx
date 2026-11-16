// @vitest-environment jsdom
import { createHash } from "node:crypto";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyPlaneClaimChange,
  applyPlaneClaimDelete,
  previewPlaneClaimDelete,
} from "@/app/fleet/planes/[namespace]/[name]/actions";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { Resource } from "@/lib/k8s/kinds";
import { type WatchSpec, watchId } from "@/lib/k8s/shared-watch";
import { ENVIRONMENT_LABEL, REVISION_ANNOTATION } from "@/lib/labels";
import type { WriteMode } from "@/lib/write/types";
import { FakeEventSource } from "../../../test/fakes/event-source";
import { fakeWriter } from "../../../test/fakes/writer";
import { DeletePlane } from "./delete-plane";
import type { DeletePlaneData } from "./delete-plane-model";
import { DATABASE_DATA } from "./delete-plane-view";

vi.mock("@/lib/write/writer", () => import("../../../test/fakes/writer"));

const KEY = "CamundaManagementClusterClaim/platform/plane-a";
const ref = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaManagementClusterClaim",
  namespace: "platform",
  name: "plane-a",
};

type ClaimOver = {
  resourceVersion?: string;
  annotations?: Record<string, string>;
  deletionPolicy?: "Delete" | "Orphan";
};
const claim = (over: ClaimOver = {}) => {
  const { deletionPolicy, ...metadata } = over;
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: {
      namespace: "platform",
      name: "plane-a",
      resourceVersion: "7",
      uid: "uid-1",
      ...metadata,
    },
    spec: {
      environments: ["platform"],
      plugins: [
        {
          section: "management-database",
          registration: "existing-database-configs",
        },
      ],
      ...(deletionPolicy && { deletionPolicy }),
    },
    status: {
      resolvedPolicy: { deletionPolicy: "Delete" },
      managementCluster: { name: "plane-a" },
      requests: [
        {
          section: "management-database",
          registration: "existing-database-configs",
          name: "plane-a-management-database",
        },
      ],
    },
  } as unknown as Resource<"CamundaManagementClusterClaim">;
};

const controlled = { ownerReferences: [{ controller: true, uid: "uid-1" }] };

const data = (
  deletionPolicies = ["Delete", "Orphan"],
  over: Partial<DeletePlaneData> = {},
): DeletePlaneData =>
  ({
    claim: claim(),
    plane: {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaManagementCluster",
      metadata: { namespace: "platform", name: "plane-a", ...controlled },
      spec: {},
      status: {
        managementAuthConfig: "plane-a",
        clusters: [
          { namespace: "team-orders", name: "orders", attached: true },
        ],
      },
    },
    namespace: {
      metadata: {
        name: "platform",
        labels: { [ENVIRONMENT_LABEL]: "platform" },
      },
    },
    environments: [],
    sections: [
      {
        metadata: { name: "management-database" },
        spec: {
          label: "Management database",
          selection: "One",
          appliesTo: ["CamundaManagementClusterClaim"],
        },
      },
    ],
    registrations: [
      {
        metadata: { name: "existing-database-configs" },
        spec: {
          label: "Existing DatabaseConfigs",
          sections: ["management-database"],
          deletionPolicies,
        },
      },
    ],
    requests: [
      {
        metadata: {
          namespace: "platform",
          name: "plane-a-management-database",
          ...controlled,
        },
        spec: { section: "management-database" },
        status: {
          resources: [{ kind: "Database", name: "identity", onOrphan: "Kept" }],
        },
      },
    ],
    optimizes: [
      {
        kind: "CamundaOptimize",
        metadata: { namespace: "team-orders", name: "orders" },
        spec: { managementAuthRef: "plane-a" },
      },
    ],
    errors: [],
    ...over,
  }) as unknown as DeletePlaneData;

const actions = {
  preview: previewPlaneClaimDelete,
  remove: applyPlaneClaimDelete,
  applyPolicy: applyPlaneClaimChange,
};

function page(initial: DeletePlaneData = data(), mode: WriteMode = "direct") {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        <DeletePlane initial={initial} writeMode={mode} actions={actions} />
      </GitPushesProvider>
    </QueryClientProvider>,
  );
}

async function confirm(label: string) {
  fireEvent.change(await screen.findByLabelText("Type plane-a to confirm"), {
    target: { value: "plane-a" },
  });
  const button = screen.getByRole<HTMLButtonElement>("button", {
    name: label,
  });
  await waitFor(() => expect(button.disabled).toBe(false));
  fireEvent.click(button);
}

async function watched(spec: WatchSpec) {
  const watch = watchId(spec);
  await waitFor(() =>
    expect(FakeEventSource.instances.at(-1)?.ids()).toContain(watch),
  );
  return { source: FakeEventSource.instances.at(-1)!, watch };
}

async function snapshot(spec: WatchSpec, objects: object[]) {
  const { source, watch } = await watched(spec);
  source.emit("snapshot", { watch, kind: spec.kind, objects });
  await act(async () => {});
}

const claimWatch: WatchSpec = {
  kind: "CamundaManagementClusterClaim",
  namespace: "platform",
  name: "plane-a",
};
const planeWatch: WatchSpec = {
  kind: "CamundaManagementCluster",
  namespace: "platform",
  name: "plane-a",
};

// readAll sends a snapshot of the objects of initial on each watch of the
// page, so that every list is current and the review opens.
async function readAll(initial: DeletePlaneData = data()) {
  const lists: [WatchSpec, object[]][] = [
    [claimWatch, [initial.claim]],
    [planeWatch, initial.plane ? [initial.plane] : []],
    [
      { kind: "Namespace", name: "platform" },
      initial.namespace ? [initial.namespace] : [],
    ],
    [{ kind: "PluginRequest", namespace: "platform" }, initial.requests],
    [{ kind: "CamundaOptimize" }, initial.optimizes],
    [{ kind: "Environment" }, initial.environments],
    [{ kind: "PluginSection" }, initial.sections],
    [{ kind: "PluginRegistration" }, initial.registrations],
  ];
  for (const [spec, objects] of lists) await snapshot(spec, objects);
}

beforeEach(() => {
  fakeWriter.reset();
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const remove = (resourceVersion = "7") => ({
  intent: "Delete the management plane plane-a",
  apply: [],
  delete: [ref],
  uids: { [KEY]: "uid-1" },
  preconditions: { [KEY]: resourceVersion },
});

const radio = (name: RegExp) =>
  screen.getByRole<HTMLInputElement>("radio", { name });

describe("DeletePlane", () => {
  it("preselects the policy that the claim recorded, and turns Keep off when an option cannot keep", async () => {
    page(data(["Delete"]));
    await readAll(data(["Delete"]));
    expect(radio(/Delete the resources/).checked).toBe(true);
    expect(radio(/Keep the resources/).disabled).toBe(true);
  });

  it("preselects Keep for a claim whose policy is Orphan", async () => {
    const initial = data(undefined, {
      claim: claim({ deletionPolicy: "Orphan" }),
    });
    page(initial);
    await readAll(initial);
    expect(radio(/Keep the resources/).checked).toBe(true);
  });

  it("the confirm stays off until the name is typed", async () => {
    page();
    await readAll();
    const button = await screen.findByRole<HTMLButtonElement>("button", {
      name: "Delete the management plane",
    });
    expect(button.disabled).toBe(true);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("direct mode: the policy of the claim sends only the delete, with the claim UID and resourceVersion", async () => {
    page();
    await readAll();
    await confirm("Delete the management plane");
    await waitFor(() => expect(fakeWriter.applied).toEqual([remove()]));
  });

  it("direct mode: another policy is written first, and the delete follows once the claim has it", async () => {
    page();
    await readAll();
    fireEvent.click(radio(/Keep the resources/));
    await confirm("Delete the management plane");
    expect(
      await screen.findByText(
        /Waiting for the operator to read the deletion policy\./,
      ),
    ).toBeTruthy();
    expect(fakeWriter.applied).toHaveLength(1);
    expect(
      (fakeWriter.applied[0].apply[0].spec as { deletionPolicy?: string })
        .deletionPolicy,
    ).toBe("Orphan");
    await snapshot(claimWatch, [
      claim({ deletionPolicy: "Orphan", resourceVersion: "8" }),
    ]);
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
    expect(fakeWriter.applied[1]).toEqual(remove("8"));
  });

  it("Git mode: sends the delete only after the policy revision is live", async () => {
    fakeWriter.reset("git");
    page(data(), "git");
    await readAll();
    fireEvent.click(radio(/Keep the resources/));
    await confirm("Push the delete to Git");
    await screen.findByText(
      /Waiting for GitOps to apply the deletion policy\./,
    );
    expect(fakeWriter.applied).toHaveLength(1);
    const revision = createHash("sha1")
      .update(JSON.stringify(fakeWriter.applied[0].apply[0]))
      .digest("hex");
    await snapshot(claimWatch, [
      claim({
        resourceVersion: "8",
        deletionPolicy: "Orphan",
        annotations: { [REVISION_ANNOTATION]: revision },
      }),
    ]);
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
    expect(fakeWriter.applied[1]).toEqual(remove("8"));
  });

  it("lists what goes and what stays for the chosen policy", async () => {
    page();
    await readAll();
    await screen.findByRole("button", { name: "Delete the management plane" });
    expect(
      screen.getByText(
        "This applies to the resources of the database and access options.",
      ),
    ).toBeTruthy();
    expect(screen.getByText(DATABASE_DATA)).toBeTruthy();
    expect(
      screen.getByText(/These clusters stay and keep running\./),
    ).toBeTruthy();
    expect(screen.getByText("team-orders/orders")).toBeTruthy();
    expect(
      screen.getByText(
        /Each CamundaOptimize that reads it then reports InvalidReference: team-orders\/orders/,
      ),
    ).toBeTruthy();
    expect(screen.getAllByText("Deleted").length).toBeGreaterThan(0);
    fireEvent.click(radio(/Keep the resources/));
    expect(screen.queryByText(DATABASE_DATA)).toBeNull();
    expect(screen.getAllByText("Kept").length).toBeGreaterThan(0);
  });

  it("a failed watch of the plane closes the review, names the kind, and states nothing about the served clusters", async () => {
    page();
    await readAll();
    await screen.findByRole("button", { name: "Delete the management plane" });
    const { source, watch } = await watched(planeWatch);
    act(() =>
      source.emit("failure", {
        watch,
        kind: "CamundaManagementCluster",
        message: "lost",
      }),
    );
    expect(
      await screen.findByText("Could not read CamundaManagementCluster: lost"),
    ).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "Delete the management plane" }),
    ).toBeNull();
    expect(
      screen.getByText(
        "Not known: the page could not read CamundaManagementCluster.",
      ),
    ).toBeTruthy();
    expect(screen.queryByText(/These clusters stay/)).toBeNull();
  });

  it("a failed watch of the requests leaves the fates not known", async () => {
    page();
    await readAll();
    const { source, watch } = await watched({
      kind: "PluginRequest",
      namespace: "platform",
    });
    act(() =>
      source.emit("failure", { watch, kind: "PluginRequest", message: "lost" }),
    );
    expect(await screen.findByText("Not known")).toBeTruthy();
    expect(screen.queryByText("Deleted")).toBeNull();
  });
});
