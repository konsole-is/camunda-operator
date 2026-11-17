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
import type { KubeObject, WriteMode } from "@/lib/write/types";
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
      generation: 1,
      finalizers: ["fleet.camunda.io/cleanup"],
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
      observedGeneration: 1,
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
  fakeWriter.objects = [initial.claim as unknown as KubeObject];
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

// removed says that the delete of the reviewed claim, by its UID, is
// the one write of applied. The claim delete seam owns the precondition
// that keeps it off a newer copy, so the test does not name it.
const removed = (applied: unknown) =>
  expect(applied).toMatchObject({
    intent: "Delete the management plane plane-a",
    apply: [],
    delete: [ref],
    uids: { [KEY]: "uid-1" },
    generations: { [KEY]: 1 },
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

  it("direct mode: the policy of the claim sends only the delete, once, for the reviewed claim", async () => {
    page();
    await readAll();
    await confirm("Delete the management plane");
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    removed(fakeWriter.applied[0]);
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
    const written = claim({ deletionPolicy: "Orphan", resourceVersion: "8" });
    fakeWriter.objects = [written as unknown as KubeObject];
    // The cleanup finalizer gives the policy to the request of the option
    // on delete, so the request keeps the old policy here.
    await snapshot(claimWatch, [written]);
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
    removed(fakeWriter.applied[1]);
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
    const written = claim({
      resourceVersion: "8",
      deletionPolicy: "Orphan",
      annotations: { [REVISION_ANNOTATION]: revision },
    });
    fakeWriter.objects = [written as unknown as KubeObject];
    await snapshot(claimWatch, [written]);
    expect(fakeWriter.applied).toHaveLength(1);
    // In Git mode the delete waits until the request has the policy.
    await snapshot(
      { kind: "PluginRequest", namespace: "platform" },
      data().requests.map((r) => ({
        ...r,
        spec: { ...r.spec, deletionPolicy: "Orphan" },
      })),
    );
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
    removed(fakeWriter.applied[1]);
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

  const planeWith = (
    spec: object,
    clusters: object[] = [
      { namespace: "team-orders", name: "orders", attached: true },
    ],
    meta: object = controlled,
  ) =>
    ({
      ...data(),
      plane: {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaManagementCluster",
        metadata: { namespace: "platform", name: "plane-a", ...meta },
        spec,
        status: { managementAuthConfig: "plane-a", clusters },
      },
    }) as unknown as DeletePlaneData;

  const KEYCLOAK_SIGN_IN =
    "Each cluster that uses the Keycloak of this plane as its OIDC issuer loses sign-in: its users and API clients can no longer sign in or get tokens.";
  const REALM_ERASED =
    "When a DatabaseServer of an option holds the Keycloak database, the delete erases the users and clients of the realm.";

  it("the review of a plane with its own Keycloak warns that the served OIDC clusters cannot sign in or get tokens", async () => {
    const initial = planeWith({ identityProvider: { keycloak: {} } });
    page(initial);
    await readAll(initial);
    await screen.findByRole("button", { name: "Delete the management plane" });
    const caution = await screen.findByText(
      /This plane still serves 1 cluster\./,
    );
    expect(caution.textContent).toContain(KEYCLOAK_SIGN_IN);
    expect(caution.textContent).toContain(REALM_ERASED);
    expect(
      screen.getByText(/These clusters stay and keep running\./).textContent,
    ).toContain(KEYCLOAK_SIGN_IN);
    fireEvent.click(radio(/Keep the resources/));
    await waitFor(() =>
      expect(
        screen.getByText(/This plane still serves 1 cluster\./).textContent,
      ).not.toContain(REALM_ERASED),
    );
  });

  it("the review of an OIDC plane warns about Console, Web Modeler, and Optimize only", async () => {
    const initial = planeWith({ identityProvider: { oidc: {} } });
    page(initial);
    await readAll(initial);
    const caution = await screen.findByText(
      /This plane still serves 1 cluster\./,
    );
    expect(caution.textContent).toBe(
      "This plane still serves 1 cluster. Sign-in to Console, Web Modeler, and Optimize stops for them.",
    );
    expect(screen.queryByText(/Keycloak of this plane/)).toBeNull();
  });

  it("the review of a plane with its own Keycloak and no attached cluster still warns about the clusters that sign in at that Keycloak", async () => {
    const initial = planeWith({ identityProvider: { keycloak: {} } }, [
      {
        namespace: "team-orders",
        name: "orders",
        attached: false,
        reason: "NotReady",
      },
    ]);
    page(initial);
    await readAll(initial);
    await screen.findByRole("button", { name: "Delete the management plane" });
    expect(screen.queryByText("The plane serves no cluster now.")).toBeNull();
    expect(
      screen.getByText(`No cluster is attached now. ${KEYCLOAK_SIGN_IN}`),
    ).toBeTruthy();
    expect(screen.queryByText(/still serves/)).toBeNull();
    expect(
      await screen.findByText(`${KEYCLOAK_SIGN_IN} ${REALM_ERASED}`),
    ).toBeTruthy();
  });

  it("the review of an OIDC plane that serves no cluster has no caution", async () => {
    const initial = planeWith({ identityProvider: { oidc: {} } }, []);
    page(initial);
    await readAll(initial);
    await screen.findByRole("button", { name: "Delete the management plane" });
    expect(screen.getByText("The plane serves no cluster now.")).toBeTruthy();
    expect(screen.queryByText(/still serves/)).toBeNull();
    expect(screen.queryByText(/Keycloak of this plane/)).toBeNull();
  });

  it("a plane that the claim does not control is not touched, nor are the clusters that it serves", async () => {
    const initial = planeWith(
      { identityProvider: { keycloak: {} } },
      undefined,
      {},
    );
    page(initial);
    await readAll(initial);
    await screen.findByRole("button", { name: "Delete the management plane" });
    expect(
      screen.getByText(
        "This delete does not touch the plane or the clusters that it serves.",
      ),
    ).toBeTruthy();
    expect(screen.queryByText("The plane serves no cluster now.")).toBeNull();
    expect(screen.queryByText(/still serves/)).toBeNull();
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
    expect(screen.queryByText(/still serves/)).toBeNull();
    expect(
      screen.getByText("Always deleted").parentElement?.textContent,
    ).not.toContain("CamundaManagementCluster plane-a");
    expect(
      screen.getByText("Not known", { selector: "span" }).parentElement
        ?.textContent,
    ).toContain("CamundaManagementCluster plane-a: not known");
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
