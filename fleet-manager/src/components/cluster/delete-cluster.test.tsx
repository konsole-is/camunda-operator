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
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyClaimChange,
  applyClaimDelete,
  previewClaimDelete,
} from "@/app/fleet/clusters/[namespace]/[name]/actions";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { Resource } from "@/lib/k8s/kinds";
import { type WatchSpec, watchId } from "@/lib/k8s/shared-watch";
import { REVISION_ANNOTATION } from "@/lib/labels";
import type { WriteMode } from "@/lib/write/types";
import { FakeEventSource } from "../../../test/fakes/event-source";
import { fakeWriter } from "../../../test/fakes/writer";
import { DeleteCluster } from "./delete-cluster";
import type { DeleteClusterData } from "./delete-cluster-model";

vi.mock("@/lib/write/writer", () => import("../../../test/fakes/writer"));

const KEY = "CamundaClusterClaim/team-a/orders";
const ref = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  namespace: "team-a",
  name: "orders",
};

type ClaimOver = {
  resourceVersion?: string;
  uid?: string;
  deletionTimestamp?: string;
  annotations?: Record<string, string>;
  deletionPolicy?: "Delete" | "Orphan";
};
const claim = (over: ClaimOver = {}) => {
  const { deletionPolicy, ...metadata } = over;
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: {
      namespace: "team-a",
      name: "orders",
      resourceVersion: "41",
      uid: "uid-1",
      ...metadata,
    },
    spec: {
      releaseRef: "camunda-8-9-1",
      plugins: [{ section: "backup-storage", registration: "aws-s3" }],
      ...(deletionPolicy && { deletionPolicy }),
    },
    status: {
      resolvedPolicy: { deletionPolicy: "Delete" },
      requests: [
        {
          section: "backup-storage",
          registration: "aws-s3",
          name: "orders-backup-storage",
        },
      ],
    },
  } as unknown as Resource<"CamundaClusterClaim">;
};

const data = (over: Partial<DeleteClusterData> = {}): DeleteClusterData =>
  ({
    claim: claim(),
    cluster: {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaCluster",
      metadata: { namespace: "team-a", name: "orders" },
    },
    sections: [
      {
        metadata: { name: "backup-storage" },
        spec: {
          label: "Backup storage",
          selection: "One",
          appliesTo: ["CamundaClusterClaim"],
          slot: "backupStorage",
        },
      },
    ],
    registrations: [
      {
        metadata: { name: "aws-s3" },
        spec: {
          label: "Amazon S3",
          sections: ["backup-storage"],
          deletionPolicies: ["Delete", "Orphan"],
        },
      },
    ],
    requests: [
      {
        metadata: { namespace: "team-a", name: "orders-backup-storage" },
        status: {
          resources: [
            { kind: "S3Bucket", name: "orders-backups", onOrphan: "Kept" },
          ],
        },
      },
    ],
    optimizes: [],
    schedules: [],
    backups: { elasticsearch: [], rdbms: [] },
    presets: [],
    releases: [],
    errors: [],
    ...over,
  }) as DeleteClusterData;

const actions = {
  preview: previewClaimDelete,
  remove: applyClaimDelete,
  applyPolicy: applyClaimChange,
};

// page renders the delete page of initial in writeMode, direct when it is
// left out. An explicit undefined is a page that could not read the mode.
function page(
  initial: DeleteClusterData = data(),
  ...mode: [WriteMode | undefined] | []
) {
  const writeMode = mode.length === 0 ? "direct" : mode[0];
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        <DeleteCluster
          initial={initial}
          writeMode={writeMode}
          actions={actions}
        />
      </GitPushesProvider>
    </QueryClientProvider>,
  );
}

// confirm types the name of the cluster and clicks label once the preview
// has run.
async function confirm(label: string) {
  fireEvent.change(await screen.findByLabelText("Type orders to confirm"), {
    target: { value: "orders" },
  });
  const button = screen.getByRole<HTMLButtonElement>("button", {
    name: label,
  });
  await waitFor(() => expect(button.disabled).toBe(false));
  fireEvent.click(button);
}

// watched waits until the page watches spec and returns the connection.
async function watched(spec: WatchSpec) {
  const watch = watchId(spec);
  await waitFor(() =>
    expect(FakeEventSource.instances.at(-1)?.ids()).toContain(watch),
  );
  return { source: FakeEventSource.instances.at(-1)!, watch };
}

// snapshot sends objects as the list of the watch of spec.
async function snapshot(spec: WatchSpec, objects: object[]) {
  const { source, watch } = await watched(spec);
  source.emit("snapshot", { watch, kind: spec.kind, objects });
  await act(async () => {});
}

// gone sends the delete of object on the watch of spec.
async function gone(spec: WatchSpec, object: object) {
  const { source, watch } = await watched(spec);
  source.emit("delta", { watch, type: "DELETED", kind: spec.kind, object });
  await act(async () => {});
}

const claimWatch: WatchSpec = {
  kind: "CamundaClusterClaim",
  namespace: "team-a",
  name: "orders",
};

// liveClaim sends c as the live claim and waits until the page shows it.
async function liveClaim(c: Resource<"CamundaClusterClaim">) {
  await snapshot(claimWatch, [c]);
}

const clusterWatch: WatchSpec = {
  kind: "CamundaCluster",
  namespace: "team-a",
  name: "orders",
};
const requestsWatch: WatchSpec = {
  kind: "PluginRequest",
  namespace: "team-a",
  labelSelector: "fleet.camunda.io/claim=orders",
};

// readAll sends a snapshot of the objects of initial on each watch of the
// page, so that every list is current and the review opens.
async function readAll(initial: DeleteClusterData = data()) {
  await liveClaim(initial.claim);
  await snapshot(clusterWatch, initial.cluster ? [initial.cluster] : []);
  await snapshot(requestsWatch, initial.requests);
  await snapshot(
    { kind: "CamundaOptimize", namespace: "team-a" },
    initial.optimizes,
  );
  await snapshot(
    { kind: "BackupSchedule", namespace: "team-a" },
    initial.schedules,
  );
}

// revisionOfFirstPush is the revision that GitOps writes for the first
// push of fakeWriter.
function revisionOfFirstPush() {
  return createHash("sha1")
    .update(JSON.stringify(fakeWriter.applied[0].apply[0]))
    .digest("hex");
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

const remove = (resourceVersion = "41") => ({
  intent: "Delete the cluster orders",
  apply: [],
  delete: [ref],
  uids: { [KEY]: "uid-1" },
  preconditions: { [KEY]: resourceVersion },
});

describe("DeleteCluster", () => {
  it("direct mode: Keep writes the deletion policy, waits until the claim has it, then deletes the claim by its UID and resourceVersion", async () => {
    page();
    await readAll();
    fireEvent.click(screen.getByRole("radio", { name: /Keep the resources/ }));
    await confirm("Delete the cluster");
    expect(
      await screen.findByText(
        /Waiting for the operator to read the deletion policy\./,
      ),
    ).toBeTruthy();
    expect(fakeWriter.applied).toHaveLength(1);
    const [policy] = fakeWriter.applied;
    expect(
      (policy.apply[0].spec as { deletionPolicy?: string }).deletionPolicy,
    ).toBe("Orphan");
    expect(policy.preconditions).toEqual({ [KEY]: "41" });
    await liveClaim(claim({ deletionPolicy: "Orphan", resourceVersion: "42" }));
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
    expect(fakeWriter.applied[1]).toEqual(remove("42"));
  });

  it("direct mode: the default sends only the delete", async () => {
    page();
    await readAll();
    await confirm("Delete the cluster");
    await waitFor(() => expect(fakeWriter.applied).toEqual([remove()]));
  });

  it("direct mode: after the delete the page follows the cluster and its requests until the claim is gone", async () => {
    page();
    await readAll();
    await confirm("Delete the cluster");
    const list = await screen.findByRole("region", { name: "Delete progress" });
    await liveClaim(claim({ deletionTimestamp: "2026-11-15T10:00:00Z" }));
    await snapshot(clusterWatch, []);
    const row = within(list).getByText("CamundaCluster orders").closest("li")!;
    await waitFor(() => expect(row.textContent).toContain("Deleted"));
    expect(
      within(list)
        .getByText("Backup storage request orders-backup-storage")
        .closest("li")!.textContent,
    ).toContain("Waiting");
    await gone(claimWatch, claim());
    expect(
      await screen.findByText("The cluster orders is deleted."),
    ).toBeTruthy();
  });

  it("a refused delete after the policy write keeps the refusal and a note on the policy while the claim changes, and the next confirm sends only the delete", async () => {
    fakeWriter.results = [
      { ok: true, mode: "direct" },
      {
        ok: false,
        errors: [{ code: "Forbidden", message: "The app may not delete it." }],
      },
    ];
    page();
    await readAll();
    fireEvent.click(screen.getByRole("radio", { name: /Keep the resources/ }));
    await confirm("Delete the cluster");
    await liveClaim(claim({ deletionPolicy: "Orphan", resourceVersion: "42" }));
    const note =
      "The deletion policy of orders is now Keep, and the cluster is not deleted.";
    expect(await screen.findByText(note)).toBeTruthy();
    expect(screen.getByText("The app may not delete it.")).toBeTruthy();
    await liveClaim(claim({ deletionPolicy: "Orphan", resourceVersion: "43" }));
    expect(screen.getByText(note)).toBeTruthy();
    expect(screen.getByText("The app may not delete it.")).toBeTruthy();
    await confirm("Delete the cluster");
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(3));
    expect(fakeWriter.applied[2]).toEqual(remove("43"));
  });

  it("Git mode: the policy and the delete are two commits, the second after GitOps applied the first", async () => {
    fakeWriter.reset("git");
    page(data(), "git");
    await readAll();
    fireEvent.click(screen.getByRole("radio", { name: /Keep the resources/ }));
    await confirm("Push the delete to Git");
    expect(
      await screen.findByText(
        /Waiting for GitOps to apply the deletion policy\./,
      ),
    ).toBeTruthy();
    expect(fakeWriter.applied).toHaveLength(1);
    await liveClaim(
      claim({
        resourceVersion: "42",
        deletionPolicy: "Orphan",
        annotations: { [REVISION_ANNOTATION]: revisionOfFirstPush() },
      }),
    );
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
    expect(fakeWriter.applied[1]).toEqual(remove("42"));
    expect(
      await screen.findByText("Waiting for GitOps to delete orders"),
    ).toBeTruthy();
  });

  it("Git mode: stop waiting sends no delete and says that the policy commit is in the repository", async () => {
    fakeWriter.reset("git");
    page(data(), "git");
    await readAll();
    fireEvent.click(screen.getByRole("radio", { name: /Keep the resources/ }));
    await confirm("Push the delete to Git");
    fireEvent.click(
      await screen.findByRole("button", { name: "Stop waiting" }),
    );
    expect(
      screen.getByRole("button", { name: "Push the delete to Git" }),
    ).toBeTruthy();
    expect(
      screen.getByText(
        /The commit of the deletion policy is in the repository\./,
      ),
    ).toBeTruthy();
    // The new claim version makes the review preview again, so the page
    // has seen it once the preview count grows.
    const previews = fakeWriter.previewed.length;
    await liveClaim(
      claim({
        resourceVersion: "42",
        deletionPolicy: "Orphan",
        annotations: { [REVISION_ANNOTATION]: revisionOfFirstPush() },
      }),
    );
    await waitFor(() =>
      expect(fakeWriter.previewed.length).toBeGreaterThan(previews),
    );
    expect(fakeWriter.applied).toHaveLength(1);
  });

  it.each([
    ["the default choice", false],
    ["Keep, which writes the policy first", true],
  ])(
    "Git mode: a claim with no file in the repository shows the refusal and writes nothing, with %s",
    async (_, keep) => {
      const message =
        "fleet/team-a/camundaclusterclaims/orders.yaml is not in the repository. GitOps does not manage CamundaClusterClaim orders.";
      fakeWriter.reset("git");
      fakeWriter.previews = Array.from({ length: 6 }, () => ({
        objects: [],
        errors: [{ code: "Invalid" as const, message }],
      }));
      page(data(), "git");
      await readAll();
      if (keep)
        fireEvent.click(
          screen.getByRole("radio", { name: /Keep the resources/ }),
        );
      fireEvent.change(await screen.findByLabelText("Type orders to confirm"), {
        target: { value: "orders" },
      });
      await waitFor(() =>
        expect(screen.getByRole("alert").textContent).toContain(message),
      );
      const button = screen.getByRole<HTMLButtonElement>("button", {
        name: "Push the delete to Git",
      });
      expect(button.disabled).toBe(true);
      expect(fakeWriter.applied).toEqual([]);
    },
  );

  it("opens no review until every list of the page has a current snapshot", async () => {
    page();
    expect(
      screen.getByText(/Delete is possible when the fleet manager can read/)
        .textContent,
    ).toContain("CamundaClusterClaim");
    expect(
      screen.queryByRole("button", { name: "Delete the cluster" }),
    ).toBeNull();
    await readAll();
    expect(
      await screen.findByRole("button", { name: "Delete the cluster" }),
    ).toBeTruthy();
  });

  it("a failed watch of the requests closes the review, names the kind, and leaves the fates not known", async () => {
    page();
    await readAll();
    await screen.findByRole("button", { name: "Delete the cluster" });
    const { source, watch } = await watched(requestsWatch);
    act(() =>
      source.emit("failure", { watch, kind: "PluginRequest", message: "lost" }),
    );
    expect(
      await screen.findByText("Could not read PluginRequest: lost"),
    ).toBeTruthy();
    expect(
      screen.getByText(
        "Delete is possible when the fleet manager can read PluginRequest.",
      ),
    ).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "Delete the cluster" }),
    ).toBeNull();
    expect(screen.getByText("Not known")).toBeTruthy();
    expect(screen.queryByText(/the backup bucket goes too/)).toBeNull();
  });

  it.each([
    ["PluginRegistration", /Not known/],
    ["CamundaClusterPreset", /Volumes not known/],
  ])(
    "a failed server read of %s names the kind and opens no review",
    async (kind, shown) => {
      page(data({ errors: [{ kind, message: "forbidden" }] }));
      await readAll();
      expect(
        screen.getByText(`Could not read ${kind}: forbidden`),
      ).toBeTruthy();
      expect(screen.getAllByText(shown).length).toBeGreaterThan(0);
      expect(
        screen.queryByRole("button", { name: "Delete the cluster" }),
      ).toBeNull();
    },
  );

  it("a current snapshot of a watch replaces a list that the server could not read", async () => {
    page(data({ errors: [{ kind: "PluginRequest", message: "forbidden" }] }));
    expect(
      screen.getByText("Could not read PluginRequest: forbidden"),
    ).toBeTruthy();
    await readAll();
    expect(
      await screen.findByRole("button", { name: "Delete the cluster" }),
    ).toBeTruthy();
    expect(
      screen.queryByText("Could not read PluginRequest: forbidden"),
    ).toBeNull();
  });

  it("the volumes follow the live cluster", async () => {
    const retaining = {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaCluster",
      metadata: { namespace: "team-a", name: "orders" },
      spec: {
        zeebe: {
          persistentVolumeClaimRetentionPolicy: { whenDeleted: "Retain" },
        },
      },
    } as unknown as Resource<"CamundaCluster">;
    page(data({ cluster: retaining }));
    expect(screen.getByText(/Volumes not known/)).toBeTruthy();
    await readAll(data({ cluster: retaining }));
    expect(
      await screen.findByText(/Volumes kept for a cluster of the same name/),
    ).toBeTruthy();
    await snapshot(clusterWatch, [data().cluster!]);
    expect(await screen.findByText(/Volumes deleted/)).toBeTruthy();
  });

  it("a delete in progress shows a target of a kind that it could not read as not known, with the error", () => {
    page(
      data({
        claim: claim({ deletionTimestamp: "2026-11-15T10:00:00Z" }),
        cluster: undefined,
        errors: [{ kind: "CamundaCluster", message: "forbidden" }],
      }),
    );
    expect(
      screen.getByText("Could not read CamundaCluster: forbidden"),
    ).toBeTruthy();
    const list = screen.getByRole("region", { name: "Delete progress" });
    expect(
      within(list).getByText("CamundaCluster orders").closest("li")!
        .textContent,
    ).toContain("Not known");
  });

  it("changes are off without a write mode", () => {
    page(data(), undefined);
    expect(screen.getByRole("alert").textContent).toBe(
      "Changes are off: the fleet manager could not read its settings.",
    );
    expect(
      screen.queryByRole("button", { name: "Delete the cluster" }),
    ).toBeNull();
  });

  it("a claim that is being deleted opens on the progress", () => {
    page(data({ claim: claim({ deletionTimestamp: "2026-11-15T10:00:00Z" }) }));
    expect(screen.queryByRole("region", { name: "Review" })).toBeNull();
    expect(
      screen.getByRole("region", { name: "Delete progress" }),
    ).toBeTruthy();
  });

  it("a claim made again under its name stops the page and sends nothing", async () => {
    page();
    await liveClaim(claim({ uid: "uid-2" }));
    expect(
      await screen.findByText(
        /The claim orders was made again after this page opened\./,
      ),
    ).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "Delete the cluster" }),
    ).toBeNull();
    expect(fakeWriter.applied).toEqual([]);
  });
});
