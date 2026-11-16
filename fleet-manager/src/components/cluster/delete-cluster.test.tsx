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
  // notMade is a claim that does not control the cluster of its name.
  notMade?: boolean;
};
const claim = (over: ClaimOver = {}) => {
  const { deletionPolicy, notMade, ...metadata } = over;
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
      ...(!notMade && { cluster: { name: "orders" } }),
    },
  } as unknown as Resource<"CamundaClusterClaim">;
};

// controlled is the owner reference of an object that the claim made.
const controlled = {
  ownerReferences: [{ controller: true, uid: "uid-1" }],
};
// applied is the status of a cluster whose generation 1 the operator
// applied.
const applied = {
  observedGeneration: 1,
  conditions: [
    { type: "Ready", status: "True", reason: "Healthy", observedGeneration: 1 },
    {
      type: "ZeebeReady",
      status: "True",
      reason: "Healthy",
      observedGeneration: 1,
    },
  ],
};

const data = (over: Partial<DeleteClusterData> = {}): DeleteClusterData =>
  ({
    claim: claim(),
    cluster: {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaCluster",
      metadata: {
        namespace: "team-a",
        name: "orders",
        generation: 1,
        ...controlled,
      },
      status: applied,
    },
    environments: [],
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
        metadata: {
          namespace: "team-a",
          name: "orders-backup-storage",
          ownerReferences: [{ controller: true, uid: "uid-1" }],
        },
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
};

const optimizesWatch: WatchSpec = {
  kind: "CamundaOptimize",
  namespace: "team-a",
};

// readAll sends a snapshot of the objects of initial on each watch of the
// page, so that every list is current and the review opens. It leaves out
// the watches of the kinds of except.
async function readAll(
  initial: DeleteClusterData = data(),
  except: string[] = [],
) {
  const lists: [WatchSpec, object[]][] = [
    [claimWatch, [initial.claim]],
    [clusterWatch, initial.cluster ? [initial.cluster] : []],
    [
      { kind: "Namespace", name: "team-a" },
      initial.namespace ? [initial.namespace] : [],
    ],
    [requestsWatch, initial.requests],
    [optimizesWatch, initial.optimizes],
    [{ kind: "BackupSchedule", namespace: "team-a" }, initial.schedules],
    [{ kind: "Environment" }, initial.environments],
    [{ kind: "PluginSection" }, initial.sections],
    [{ kind: "PluginRegistration" }, initial.registrations],
    [{ kind: "CamundaClusterPreset" }, initial.presets],
    [{ kind: "CamundaRelease" }, initial.releases],
    [
      { kind: "LogicalBackupElasticsearch", namespace: "team-a" },
      initial.backups.elasticsearch,
    ],
    [
      { kind: "LogicalBackupRDBMS", namespace: "team-a" },
      initial.backups.rdbms,
    ],
  ];
  for (const [spec, objects] of lists)
    if (!except.includes(spec.kind)) await snapshot(spec, objects);
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

  it("disables the choice while the delete is sent, so that the page cannot show another policy than the write", async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <GitPushesProvider>
          <DeleteCluster
            initial={data()}
            writeMode="direct"
            actions={{ ...actions, remove: () => new Promise(() => {}) }}
          />
        </GitPushesProvider>
      </QueryClientProvider>,
    );
    await readAll();
    const keep = screen.getByRole<HTMLInputElement>("radio", {
      name: /Keep the resources/,
    });
    expect(keep.disabled).toBe(false);
    await confirm("Delete the cluster");
    await waitFor(() => expect(keep.disabled).toBe(true));
    expect(
      screen.getByRole<HTMLInputElement>("radio", {
        name: /Delete the resources/,
      }).disabled,
    ).toBe(true);
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
      "The cluster is not deleted. The claim orders has the deletion policy Keep.";
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

  it("a failed watch during the wait for the policy shows the read error", async () => {
    fakeWriter.reset("git");
    page(data(), "git");
    await readAll();
    fireEvent.click(screen.getByRole("radio", { name: /Keep the resources/ }));
    await confirm("Push the delete to Git");
    await screen.findByText(
      /Waiting for GitOps to apply the deletion policy\./,
    );
    const { source, watch } = await watched(optimizesWatch);
    act(() =>
      source.emit("failure", {
        watch,
        kind: "CamundaOptimize",
        message: "lost",
      }),
    );
    expect(
      await screen.findByText("Could not read CamundaOptimize: lost"),
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
        "The cluster is not deleted. The repository holds the commit that sets the deletion policy of orders to Keep, and GitOps has not applied it yet. The claim orders has the deletion policy Delete.",
      ),
    ).toBeTruthy();
    await liveClaim(
      claim({
        resourceVersion: "42",
        deletionPolicy: "Orphan",
        annotations: { [REVISION_ANNOTATION]: revisionOfFirstPush() },
      }),
    );
    expect(
      await screen.findByText(
        "The cluster is not deleted. The claim orders has the deletion policy Keep.",
      ),
    ).toBeTruthy();
    expect(fakeWriter.applied).toHaveLength(1);
  });

  it("direct mode: a change by someone else during the wait shows the policy that the claim has, not the one that the page wrote", async () => {
    page();
    await readAll();
    fireEvent.click(screen.getByRole("radio", { name: /Keep the resources/ }));
    await confirm("Delete the cluster");
    await screen.findByText(
      /Waiting for the operator to read the deletion policy\./,
    );
    await liveClaim(claim({ deletionPolicy: "Delete", resourceVersion: "42" }));
    expect(
      await screen.findByText(
        "The cluster is not deleted. The claim orders has the deletion policy Delete.",
      ),
    ).toBeTruthy();
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
    ["PluginSection", /Not known/],
    ["CamundaClusterPreset", /Volumes not known/],
    ["Environment", /Delete is possible/],
    ["Namespace", /Delete is possible/],
    ["LogicalBackupElasticsearch", /Delete is possible/],
  ])(
    "a failed server read of %s names the kind and opens no review until its watch has a snapshot",
    async (kind, shown) => {
      page(data({ errors: [{ kind, message: "forbidden" }] }));
      await readAll(data(), [kind]);
      expect(
        screen.getByText(`Could not read ${kind}: forbidden`),
      ).toBeTruthy();
      expect(screen.getAllByText(shown).length).toBeGreaterThan(0);
      expect(
        screen.queryByRole("button", { name: "Delete the cluster" }),
      ).toBeNull();
      await readAll();
      expect(
        await screen.findByRole("button", { name: "Delete the cluster" }),
      ).toBeTruthy();
      expect(
        screen.queryByText(`Could not read ${kind}: forbidden`),
      ).toBeNull();
    },
  );

  it("a claim on Keep opens no review once a registration that cannot keep comes in, until Delete is picked", async () => {
    const initial = data({
      claim: claim({ deletionPolicy: "Orphan" }),
      registrations: [],
      errors: [{ kind: "PluginRegistration", message: "forbidden" }],
    });
    page(initial);
    await readAll(initial, ["PluginRegistration"]);
    await snapshot({ kind: "PluginRegistration" }, [
      {
        metadata: { name: "aws-s3" },
        spec: {
          label: "Amazon S3",
          sections: ["backup-storage"],
          deletionPolicies: ["Delete"],
        },
      },
    ]);
    expect(
      await screen.findByText(
        'PluginRegistration "aws-s3" of section "backup-storage" cannot keep the resources. Choose Delete the resources.',
      ),
    ).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "Delete the cluster" }),
    ).toBeNull();
    fireEvent.click(
      screen.getByRole("radio", { name: /Delete the resources/ }),
    );
    expect(
      await screen.findByRole("button", { name: "Delete the cluster" }),
    ).toBeTruthy();
  });

  it("a failed watch of CamundaOptimize shows its objects as not known, also one that the claim did not make", async () => {
    const handMade = {
      metadata: { namespace: "team-a", name: "orders-optimize" },
      spec: { clusterRef: { name: "orders" } },
    } as unknown as Resource<"CamundaOptimize">;
    const initial = data({ optimizes: [handMade] });
    page(initial);
    await readAll(initial);
    await screen.findByRole("button", { name: "Delete the cluster" });
    expect(
      screen.getByText("Kept: the claim did not make it", { exact: false }),
    ).toBeTruthy();
    const { source, watch } = await watched(optimizesWatch);
    act(() =>
      source.emit("failure", {
        watch,
        kind: "CamundaOptimize",
        message: "lost",
      }),
    );
    await screen.findByText("Could not read CamundaOptimize: lost");
    expect(
      screen.queryByText("Kept: the claim did not make it", { exact: false }),
    ).toBeNull();
    const row = screen
      .getByText("CamundaOptimize orders-optimize")
      .closest("li")!;
    expect(row.textContent).toBe("CamundaOptimize orders-optimize: not known");
  });

  it("a failed watch of the CamundaCluster shows its fate as not known", async () => {
    page();
    await readAll();
    await screen.findByRole("button", { name: "Delete the cluster" });
    const { source, watch } = await watched(clusterWatch);
    act(() =>
      source.emit("failure", {
        watch,
        kind: "CamundaCluster",
        message: "lost",
      }),
    );
    await screen.findByText("Could not read CamundaCluster: lost");
    const row = screen.getByText(/^CamundaCluster orders/).closest("li")!;
    expect(row.textContent).toBe("CamundaCluster orders: not known");
  });

  it("a cluster that the claim did not make shows as kept, and the end text says that it stays", async () => {
    const initial = data({
      claim: claim({ notMade: true }),
      cluster: {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaCluster",
        metadata: { namespace: "team-a", name: "orders" },
      } as unknown as Resource<"CamundaCluster">,
    });
    page(initial);
    await readAll(initial);
    await screen.findByRole("button", { name: "Delete the cluster" });
    expect(
      screen.getByText("CamundaCluster orders").closest("li")!.textContent,
    ).toBe("CamundaCluster orders: Kept: the claim did not make it");
    await gone(claimWatch, initial.claim);
    expect(
      await screen.findByText(
        "The claim orders is deleted. The cluster orders stays: the claim did not make it.",
      ),
    ).toBeTruthy();
  });

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

  it("the volumes follow the live cluster once the operator applied its spec", async () => {
    const retaining = (generation: number) =>
      ({
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaCluster",
        metadata: {
          namespace: "team-a",
          name: "orders",
          generation,
          ...controlled,
        },
        spec: {
          zeebe: {
            persistentVolumeClaimRetentionPolicy: { whenDeleted: "Retain" },
          },
        },
        status: applied,
      }) as unknown as Resource<"CamundaCluster">;
    page(data({ cluster: retaining(1) }));
    expect(
      screen.getByText(/^CamundaCluster orders/).closest("li")!.textContent,
    ).toBe("CamundaCluster orders: not known");
    await readAll(data({ cluster: retaining(1) }));
    expect(
      await screen.findByText(/Volumes kept for a cluster of the same name/),
    ).toBeTruthy();
    await snapshot(clusterWatch, [retaining(2)]);
    expect(
      await screen.findByText(
        /Volumes not known: the operator has not applied the current spec of the cluster yet/,
      ),
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

  it("a delete in progress adds a target that a later snapshot finds", async () => {
    const initial = data();
    page(
      data({
        claim: claim({ deletionTimestamp: "2026-11-15T10:00:00Z" }),
        requests: [],
        errors: [{ kind: "PluginRequest", message: "forbidden" }],
      }),
    );
    const list = screen.getByRole("region", { name: "Delete progress" });
    expect(
      within(list).queryByText("Backup storage request orders-backup-storage"),
    ).toBeNull();
    await snapshot(requestsWatch, initial.requests);
    expect(
      await within(list).findByText(
        "Backup storage request orders-backup-storage",
      ),
    ).toBeTruthy();
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
