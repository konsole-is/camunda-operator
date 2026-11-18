// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { ReadError } from "@/lib/k8s/unread";
import type { ContractRef } from "@/lib/usage";
import type { KubeObject, Preview, WriteMode } from "@/lib/write/types";
import { type ContractActions, ConnectionDetail } from "./connection-detail";
import { deleteChange, draftOf, updateChange } from "./draft";
import type { ContractObject } from "./facts";
import { connectionFixtures } from "./fixtures";
import type { ConnectionSources } from "./sources";

const router = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
  useResources: () => {
    throw new Error("the test reads Secrets through the static reader");
  },
  useWatchesFresh: () => [false],
}));

afterEach(cleanup);

// The detail shows the toast of a write, which reads the Git pushes.
const renderIn = (ui: ReactElement) =>
  render(ui, { wrapper: GitPushesProvider });

function show(target: ContractRef) {
  renderIn(
    <ConnectionDetail
      target={target}
      sources={{ ...connectionFixtures(), errors: [] }}
    />,
  );
}

const inTeamA = (kind: ContractRef["kind"], name: string): ContractRef => ({
  kind,
  namespace: "team-a",
  name,
});

const panel = (name: string) => screen.getByRole("region", { name });

describe("ConnectionDetail", () => {
  it("shows the Ready reason and message of the contract", () => {
    show(inTeamA("DatabaseServerConfig", "postgres"));
    const status = panel("Status");
    expect(within(status).getByText("ConnectionFailed")).toBeTruthy();
    expect(
      within(status).getByText(
        "Connecting to pg.team-a:5432: connection refused",
      ),
    ).toBeTruthy();
  });

  it("says No status yet for a contract without a Ready", () => {
    const sources = connectionFixtures();
    sources.objectStorageConfigs[0] = {
      ...sources.objectStorageConfigs[0],
      status: undefined,
    };
    renderIn(
      <ConnectionDetail
        target={inTeamA("ObjectStorageConfig", "backups")}
        sources={{ ...sources, errors: [] }}
      />,
    );
    expect(within(panel("Status")).getByText("No status yet")).toBeTruthy();
  });

  it("never claims a check of the bucket", () => {
    show(inTeamA("ObjectStorageConfig", "backups"));
    expect(screen.queryByText(/reachab|bucket access|tested/i)).toBeNull();
  });

  it.each<[ContractRef, string]>([
    [
      inTeamA("SecondaryStorageConfig", "orders-es"),
      "Managed by ElasticsearchCluster orders-es. Change it there.",
    ],
    [
      inTeamA("ObjectStorageConfig", "orders-backup"),
      "Managed by Registration aws-s3. Change it there.",
    ],
    [
      { kind: "ManagementAuthConfig", name: "plane-a-optimize" },
      "Managed by Management plane platform/plane-a. Change it there.",
    ],
  ])(
    "shows Managed by and Change it there for a contract not created by hand: %j",
    (target, sentence) => {
      show(target);
      expect(within(panel("Writer")).getByText(sentence)).toBeTruthy();
    },
  );

  it.each<[ContractRef, string]>([
    [inTeamA("ObjectStorageConfig", "backups"), "Created by hand"],
    [inTeamA("ObjectStorageConfig", "old-backup"), "Kept from aws-s3"],
  ])(
    "shows no Managed by line for a contract created by hand or kept from a registration: %j",
    (target, writer) => {
      show(target);
      expect(within(panel("Writer")).getByText(writer)).toBeTruthy();
      expect(screen.queryByText(/Managed by/)).toBeNull();
    },
  );

  it("shows the CA certificate as a name and a key", () => {
    show(inTeamA("SecondaryStorageConfig", "shared-es"));
    const settings = panel("Settings");
    const ca = within(settings).getByText("CA certificate").nextElementSibling!;
    expect(within(ca as HTMLElement).getByText("shared-es-ca")).toBeTruthy();
    expect(within(ca as HTMLElement).getByText("ca.crt")).toBeTruthy();
  });

  it("shows the admin credentials as a name, a username key, and a password key", () => {
    show(inTeamA("DatabaseServerConfig", "postgres"));
    const admin = within(panel("Settings")).getByText("Admin credentials")
      .nextElementSibling as HTMLElement;
    expect(within(admin).getByText("postgres-admin")).toBeTruthy();
    expect(within(admin).getByText("username")).toBeTruthy();
    expect(within(admin).getByText("password")).toBeTruthy();
  });

  it("shows the replica hint next to the node count", () => {
    show(inTeamA("SecondaryStorageConfig", "shared-es"));
    expect(
      within(panel("Settings")).getByText(
        "Sets the index replicas when the cluster sets no replica count.",
      ),
    ).toBeTruthy();
  });

  it("lists every user of the contract", () => {
    show(inTeamA("SecondaryStorageConfig", "shared-es"));
    const users = panel("Used by");
    expect(within(users).getByText("team-a/orders")).toBeTruthy();
    expect(within(users).getByText("team-a/billing")).toBeTruthy();
    expect(
      within(users).getByText("Not ready · StorageAlreadyAttached"),
    ).toBeTruthy();
  });

  it("names the contract and its namespace in the heading", () => {
    show(inTeamA("ObjectStorageConfig", "backups"));
    expect(
      screen.getByRole("heading", { level: 1, name: /backups/ }),
    ).toBeTruthy();
    expect(screen.getByText("team-a")).toBeTruthy();
  });

  it("says that a missing contract does not exist", () => {
    show(inTeamA("ObjectStorageConfig", "gone"));
    expect(screen.getByRole("alert").textContent).toBe(
      "The contract gone does not exist.",
    );
  });

  it("says that the users are not known while a kind that can use the contract could not be read", () => {
    renderIn(
      <ConnectionDetail
        target={inTeamA("ObjectStorageConfig", "backups")}
        sources={{
          ...connectionFixtures(),
          errors: [{ kind: "DatabaseServer", message: "forbidden" }],
        }}
      />,
    );
    expect(
      within(panel("Used by")).getByText(
        "The users are not known while the fleet manager cannot read DatabaseServer.",
      ),
    ).toBeTruthy();
  });

  it("does not say that a contract is missing while its kind cannot be read", () => {
    renderIn(
      <ConnectionDetail
        target={inTeamA("ObjectStorageConfig", "backups")}
        sources={{
          ...connectionFixtures(),
          objectStorageConfigs: [],
          errors: [{ kind: "ObjectStorageConfig", message: "forbidden" }],
        }}
      />,
    );
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read ObjectStorageConfig: forbidden",
    );
  });

  it("shows the read errors", () => {
    renderIn(
      <ConnectionDetail
        target={inTeamA("ObjectStorageConfig", "backups")}
        sources={{
          ...connectionFixtures(),
          errors: [{ kind: "CamundaCluster", message: "forbidden" }],
        }}
      />,
    );
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read CamundaCluster: forbidden",
    );
  });
});

const SECRETS = staticSecretRefs({
  "team-a": [
    { name: "postgres-admin", keys: ["username", "password"] },
    { name: "shared-es-user", keys: ["user", "pass"] },
    { name: "shared-es-ca", keys: ["ca.crt"] },
  ],
});

// A preview with one changed line, so the confirm of the review opens.
function previewOf(contract: ContractObject): Preview {
  return {
    objects: [
      {
        ref: {
          apiVersion: "core.camunda.io/v1",
          kind: contract.kind,
          namespace: contract.metadata.namespace,
          name: contract.metadata.name ?? "",
        },
        before: contract as unknown as KubeObject,
        after: contract as unknown as KubeObject,
        diff: [{ op: "added", text: "changed: true" }],
      },
    ],
    errors: [],
  };
}

function edit(
  target: ContractRef,
  opts: {
    sources?: ConnectionSources;
    errors?: ReadError[];
    stale?: string[];
    mode?: WriteMode;
  } = {},
) {
  const sources = opts.sources ?? connectionFixtures();
  const mode = "mode" in opts ? opts.mode : "direct";
  const live = [
    ...sources.objectStorageConfigs,
    ...sources.secondaryStorageConfigs,
    ...sources.databaseServerConfigs,
    ...sources.databaseConfigs,
    ...sources.managementAuthConfigs,
  ].find((c) => c.metadata.name === target.name)!;
  const actions = {
    previewChange: vi.fn<ContractActions["previewChange"]>(async () =>
      live ? previewOf(live) : { objects: [], errors: [] },
    ),
    applyChange: vi.fn<ContractActions["applyChange"]>(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
  };
  const view = (s: ConnectionSources, e: ReadError[], st: string[]) => (
    <GitPushesProvider>
      <SecretRefsReaderContext.Provider value={SECRETS}>
        <ConnectionDetail
          target={target}
          sources={{ ...s, errors: e, stale: st }}
          mode={mode}
          actions={actions}
        />
      </SecretRefsReaderContext.Provider>
    </GitPushesProvider>
  );
  const { rerender } = render(
    view(sources, opts.errors ?? [], opts.stale ?? []),
  );
  return {
    actions,
    live,
    rerender: (s: ConnectionSources, e: ReadError[] = [], st: string[] = []) =>
      rerender(view(s, e, st)),
  };
}

async function openDelete(name: string) {
  fireEvent.change(screen.getByLabelText(`Type ${name} to confirm`), {
    target: { value: name },
  });
  fireEvent.click(screen.getByRole("button", { name: "Review the delete" }));
  return screen.findByRole("region", { name: "Review the delete" });
}

describe("ConnectionDetail with changes", () => {
  it("saves a contract created by hand through the review", async () => {
    const target = inTeamA("DatabaseServerConfig", "postgres");
    const { actions, live } = edit(target);
    fireEvent.change(screen.getByLabelText("Host"), {
      target: { value: "pg-2.team-a" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    const review = await screen.findByRole("region", {
      name: "Review the contract",
    });
    const draft = draftOf(live);
    (draft.spec as { host: string }).host = "pg-2.team-a";
    await waitFor(() =>
      expect(actions.previewChange).toHaveBeenCalledWith(
        updateChange(live, draft),
      ),
    );
    const save = within(review).getByRole("button", { name: "Save" });
    await waitFor(() => expect(save).toHaveProperty("disabled", false));
    fireEvent.click(save);
    await waitFor(() =>
      expect(actions.applyChange).toHaveBeenCalledWith(
        updateChange(live, draft),
      ),
    );
  });

  it("a save that changes a field of another tool shows the field and asks", async () => {
    const target = inTeamA("DatabaseServerConfig", "postgres");
    const { actions, live } = edit(target);
    const ref = {
      apiVersion: "core.camunda.io/v1",
      kind: "DatabaseServerConfig",
      namespace: "team-a",
      name: "postgres",
    };
    const ownership = [
      {
        ref,
        changed: [
          {
            path: "spec.host",
            steps: [{ field: "spec" }, { field: "host" }],
            managers: [
              {
                name: "kubectl-client-side-apply",
                operation: "Update",
                kind: "other" as const,
                known: true,
              },
            ],
          },
        ],
        cleared: [],
      },
    ];
    actions.previewChange.mockImplementation(async (change) =>
      change.takeOver
        ? { ...previewOf(live), ownership }
        : {
            objects: [],
            errors: [
              {
                code: "Conflict",
                message: 'conflict with "kubectl-client-side-apply"',
                path: "spec.host",
                ref,
              },
            ],
            ownership,
          },
    );
    fireEvent.change(screen.getByLabelText("Host"), {
      target: { value: "pg-2.team-a" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    const review = await screen.findByRole("region", {
      name: "Review the contract",
    });
    const choice = await within(review).findByRole("checkbox", {
      name: "Take over these fields",
    });
    expect(
      within(review).getByRole("group", {
        name: "Fields that another tool set",
      }).textContent,
    ).toContain(
      "spec.host: kubectl-client-side-apply set it. The save changes it.",
    );
    const save = within(review).getByRole("button", { name: "Save" });
    expect(save).toHaveProperty("disabled", true);

    fireEvent.click(choice);
    await waitFor(() => expect(save).toHaveProperty("disabled", false));
    fireEvent.click(save);
    await waitFor(() => expect(actions.applyChange).toHaveBeenCalledTimes(1));
    expect(actions.applyChange.mock.calls[0][0]).toMatchObject({
      takeOver: { "DatabaseServerConfig/team-a/postgres": ["spec.host"] },
    });
    expect(actions.applyChange.mock.calls[0][0].force).toBeUndefined();
  });

  it("names each user and what it does on the change in the review", async () => {
    edit(inTeamA("SecondaryStorageConfig", "shared-es"));
    fireEvent.change(screen.getByLabelText("Endpoint"), {
      target: { value: "https://search-2.example.com:9243" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    const review = await screen.findByRole("region", {
      name: "Review the contract",
    });
    expect(
      within(review).getByText(
        /^CamundaCluster team-a\/orders restarts all its pods/,
      ),
    ).toBeTruthy();
    expect(
      within(review).getByText(
        /^CamundaCluster team-a\/billing restarts all its pods/,
      ),
    ).toBeTruthy();
  });

  it("saves a contract kept from a registration", async () => {
    const { actions } = edit(inTeamA("ObjectStorageConfig", "old-backup"));
    fireEvent.change(screen.getByLabelText("Region"), {
      target: { value: "eu-central-1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    await waitFor(() => expect(actions.previewChange).toHaveBeenCalled());
    expect(actions.previewChange.mock.lastCall![0].intent).toBe(
      "Change the object storage contract old-backup",
    );
  });

  it.each<[string, ContractRef]>([
    ["owner", inTeamA("SecondaryStorageConfig", "orders-es")],
    ["registration", inTeamA("ObjectStorageConfig", "orders-backup")],
    ["plane", { kind: "ManagementAuthConfig", name: "plane-a-optimize" }],
  ])(
    "shows no form, Save, or Delete for a contract not created by hand: %s",
    (_, target) => {
      edit(target);
      expect(screen.queryByRole("textbox")).toBeNull();
      expect(screen.queryByRole("button", { name: /Review|Save/ })).toBeNull();
      expect(screen.queryByText("Delete the contract")).toBeNull();
      expect(
        screen.getByText(/^Managed by .+\. Change it there\.$/),
      ).toBeTruthy();
    },
  );

  it("blocks the save of a CA certificate whose key is not picked", () => {
    edit(inTeamA("SecondaryStorageConfig", "shared-es"));
    fireEvent.change(
      within(screen.getByRole("group", { name: "CA certificate" })).getByRole(
        "combobox",
        { name: "Secret" },
      ),
      { target: { value: "shared-es-user" } },
    );
    expect(screen.getByRole("button", { name: "Review" })).toHaveProperty(
      "disabled",
      true,
    );
    expect(
      screen.getAllByText("Pick the Secret and the key of the CA certificate.")
        .length,
    ).toBeGreaterThan(0);
  });

  it.each<[string, { errors?: ReadError[]; stale?: string[] }]>([
    [
      "cannot be read",
      { errors: [{ kind: "LogicalBackupRDBMS", message: "forbidden" }] },
    ],
    ["has not answered", { stale: ["LogicalBackupRDBMS"] }],
  ])("blocks a save while a user kind %s", (_, reads) => {
    edit(inTeamA("ObjectStorageConfig", "backups"), reads);
    const line =
      "A change is possible when the fleet manager can read LogicalBackupRDBMS.";
    expect(screen.getByText(line)).toBeTruthy();
    const region = screen.getByLabelText("Region") as HTMLInputElement;
    expect(region.value).not.toBe("");
    fireEvent.change(region, { target: { value: "eu-central-1" } });
    expect(screen.getByRole("button", { name: "Review" })).toHaveProperty(
      "disabled",
      true,
    );
  });

  it("closes the save review when a user kind stops being read", async () => {
    const sources = connectionFixtures();
    const { rerender } = edit(inTeamA("ObjectStorageConfig", "backups"), {
      sources,
    });
    fireEvent.change(screen.getByLabelText("Region"), {
      target: { value: "eu-central-1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    await screen.findByRole("region", { name: "Review the contract" });
    rerender(sources, [], ["LogicalBackupRDBMS"]);
    expect(
      screen.queryByRole("region", { name: "Review the contract" }),
    ).toBeNull();
  });

  it("keeps a contract kept from a registration read only while the requests are not known", () => {
    edit(inTeamA("ObjectStorageConfig", "old-backup"), {
      stale: ["PluginRequest"],
    });
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(
      screen.getByText("Managed by Registration aws-s3. Change it there."),
    ).toBeTruthy();
  });

  it("offers Delete only for an unused contract created by hand", () => {
    edit(inTeamA("ObjectStorageConfig", "backups"));
    expect(
      screen.getByText(
        "Delete is possible when no resource uses this contract.",
      ),
    ).toBeTruthy();
    expect(screen.queryByLabelText("Type backups to confirm")).toBeNull();
    cleanup();

    const sources = connectionFixtures();
    sources.databaseConfigs = [];
    edit(inTeamA("DatabaseServerConfig", "postgres"), { sources });
    expect(screen.getByLabelText("Type postgres to confirm")).toBeTruthy();
  });

  it("refuses Delete while a user kind cannot be read", () => {
    const sources = connectionFixtures();
    sources.databaseConfigs = [];
    edit(inTeamA("DatabaseServerConfig", "postgres"), {
      sources,
      errors: [{ kind: "PointInTimeRestore", message: "forbidden" }],
    });
    expect(
      screen.getByText(
        "Delete is possible when the fleet manager can read PointInTimeRestore.",
      ),
    ).toBeTruthy();
    expect(screen.queryByLabelText("Type postgres to confirm")).toBeNull();
    expect(
      within(panel("Used by")).getByText(
        "The users are not known while the fleet manager cannot read PointInTimeRestore.",
      ),
    ).toBeTruthy();
  });

  it("refuses Delete while the watch of a user kind has not answered", () => {
    edit(inTeamA("DatabaseConfig", "orders-db"), {
      stale: ["CamundaManagementCluster"],
    });
    expect(
      screen.getByText(
        "Delete is possible when the fleet manager can read CamundaManagementCluster.",
      ),
    ).toBeTruthy();
  });

  it("counts the request of a custom registration whose output names the contract, and offers no Delete", () => {
    const sources = connectionFixtures();
    sources.sections = [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginSection",
        metadata: { name: "management-database" },
        spec: { selection: "One", slot: "managementDatabases" },
      } as ConnectionSources["sections"][number],
    ];
    sources.requests = [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginRequest",
        metadata: { namespace: "team-a", name: "plane-a-databases" },
        spec: {
          section: "management-database",
          registration: "acme-postgres",
          parameters: {},
        },
        status: {
          outputs: { identityDatabaseConfig: "orders-db" },
          conditions: [
            { type: "OutputsReady", status: "True", observedGeneration: 0 },
          ],
        },
      } as unknown as ConnectionSources["requests"][number],
    ];
    edit(inTeamA("DatabaseConfig", "orders-db"), { sources });

    expect(
      within(panel("Used by")).getByText("team-a/plane-a-databases"),
    ).toBeTruthy();
    expect(
      screen.getByText(
        "Delete is possible when no resource uses this contract.",
      ),
    ).toBeTruthy();
    expect(screen.queryByLabelText("Type orders-db to confirm")).toBeNull();
  });

  it("refuses Delete and names the request while a custom request has no output yet", () => {
    const sources = connectionFixtures();
    sources.sections = [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginSection",
        metadata: { name: "management-database" },
        spec: { selection: "One", slot: "managementDatabases" },
      } as ConnectionSources["sections"][number],
    ];
    sources.requests = [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginRequest",
        metadata: { namespace: "team-a", name: "plane-a-databases" },
        spec: {
          section: "management-database",
          registration: "acme-postgres",
          parameters: { identity: "orders-db" },
        },
      } as unknown as ConnectionSources["requests"][number],
    ];
    edit(inTeamA("DatabaseConfig", "orders-db"), { sources });

    expect(
      screen.getByText(
        "Delete is possible when the fleet manager knows the contracts of PluginRequest team-a/plane-a-databases.",
      ),
    ).toBeTruthy();
    expect(
      within(panel("Used by")).getByText(
        "The users are not known while the fleet manager does not know the contracts of PluginRequest team-a/plane-a-databases.",
      ),
    ).toBeTruthy();
    expect(screen.queryByLabelText("Type orders-db to confirm")).toBeNull();
  });

  it("counts a claim that selects the contract as a user", () => {
    const sources = connectionFixtures();
    sources.planeClaims = [
      {
        kind: "CamundaManagementClusterClaim",
        metadata: { namespace: "team-a", name: "new-orders" },
        spec: {
          plugins: [
            {
              section: "management-database",
              registration: "existing-database-configs",
              parameters: { identity: "orders-db" },
            },
          ],
        },
      } as unknown as CamundaManagementClusterClaim,
    ];
    edit(inTeamA("DatabaseConfig", "orders-db"), { sources });
    expect(
      within(panel("Used by")).getByText("team-a/new-orders"),
    ).toBeTruthy();
    expect(
      screen.getByText(
        "Delete is possible when no resource uses this contract.",
      ),
    ).toBeTruthy();
  });

  it("deletes with the precondition and the UID", async () => {
    const sources = connectionFixtures();
    const live = sources.databaseConfigs[0];
    live.metadata = { ...live.metadata, resourceVersion: "9", uid: "u-9" };
    const { actions } = edit(inTeamA("DatabaseConfig", "orders-db"), {
      sources,
    });
    const review = await openDelete("orders-db");
    const button = within(review).getByRole("button", {
      name: "Delete the contract",
    });
    await waitFor(() => expect(button).toHaveProperty("disabled", false));
    fireEvent.click(button);
    await waitFor(() =>
      expect(actions.applyChange).toHaveBeenCalledWith(deleteChange(live)),
    );
    expect(deleteChange(live).uids).toEqual({
      "DatabaseConfig/team-a/orders-db": "u-9",
    });
  });

  it("closes the delete review when a user appears", async () => {
    const sources = connectionFixtures();
    const { rerender } = edit(inTeamA("DatabaseConfig", "orders-db"), {
      sources,
    });
    await openDelete("orders-db");
    rerender({
      ...sources,
      secondaryStorageConfigs: [
        ...sources.secondaryStorageConfigs,
        {
          kind: "SecondaryStorageConfig",
          apiVersion: "core.camunda.io/v1",
          metadata: { namespace: "team-a", name: "orders-rdbms" },
          spec: { type: "rdbms", rdbms: { databaseConfigRef: "orders-db" } },
        },
      ],
    });
    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
  });

  it("closes the delete review when a user kind stops being read", async () => {
    const sources = connectionFixtures();
    const { rerender } = edit(inTeamA("DatabaseConfig", "orders-db"), {
      sources,
    });
    await openDelete("orders-db");
    rerender(sources, [
      { kind: "SecondaryStorageConfig", message: "forbidden" },
    ]);
    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
  });

  it("reads Push to Git in Git mode", async () => {
    edit(inTeamA("DatabaseConfig", "orders-db"), { mode: "git" });
    const review = await openDelete("orders-db");
    expect(
      within(review).getByRole("button", { name: "Push the delete to Git" }),
    ).toBeTruthy();
    expect(
      within(review).getByText(
        "In Git mode the delete is a commit that removes the file of the contract. GitOps must prune the DatabaseConfig to delete it from the cluster.",
      ),
    ).toBeTruthy();
  });

  it("shows one fresh form after a discard", () => {
    const errors = vi.spyOn(console, "error").mockImplementation(() => {});
    edit(inTeamA("DatabaseConfig", "orders-db"));
    fireEvent.change(screen.getByLabelText("Database name"), {
      target: { value: "orders-2" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Discard" }));
    const names = screen.getAllByLabelText(
      "Database name",
    ) as HTMLInputElement[];
    expect(names.map((n) => n.value)).toEqual(["orders"]);
    expect(
      errors.mock.calls.filter((call) => String(call[0]).includes("same key")),
    ).toEqual([]);
    errors.mockRestore();
  });

  it("disables the form without a write mode", () => {
    edit(inTeamA("ObjectStorageConfig", "backups"), { mode: undefined });
    expect(screen.getByLabelText("Region").closest("fieldset")!.disabled).toBe(
      true,
    );
  });
});
