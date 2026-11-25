// @vitest-environment jsdom
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
  applyContractChange,
  previewContractChange,
} from "@/app/setup/connections/actions";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { WriteMode } from "@/lib/write/types";
import { fakeWriter } from "../../../../test/fakes/writer";
import { ContractCreate } from "./contract-create";
import { createChange } from "./draft";
import { connectionFixtures } from "./fixtures";
import { CREATE_EFFECT } from "./impact";
import { CONNECTION_SOURCE_KINDS } from "./source-kinds";
import type { ConnectionSources } from "./sources";
import type { ConnectionTab } from "./tabs";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));
const cache = vi.hoisted(() => ({ lists: {} as Record<string, unknown[]> }));
vi.mock("@/lib/k8s/read", () => ({
  list: async (kind: string) => cache.lists[kind] ?? [],
}));
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

beforeEach(() => {
  fakeWriter.reset();
  router.push.mockClear();
});
afterEach(cleanup);

const SECRETS = staticSecretRefs({
  "team-a": [
    { name: "gcs", keys: ["key.json"] },
    { name: "shared-es-user", keys: ["username", "password"] },
    { name: "postgres-admin", keys: ["username", "password"] },
    { name: "orders-db", keys: ["user", "pass"] },
  ],
  "team-b": [],
  platform: [{ name: "plane-a-optimize-client", keys: ["client-secret"] }],
});

// withTeamB returns the fixtures with the namespaces team-b and platform.
function withTeamB(): ConnectionSources {
  const sources = connectionFixtures();
  sources.namespaces = [
    ...sources.namespaces,
    { metadata: { name: "team-b" } },
    { metadata: { name: "platform" } },
  ];
  return sources;
}

// setup renders the page on page, and fills the cache of the server that
// the actions read from server, both withTeamB by default.
function setup(
  tab: ConnectionTab,
  opts: {
    mode?: WriteMode;
    page?: ConnectionSources;
    server?: ConnectionSources;
  } = {},
) {
  const mode = "mode" in opts ? opts.mode : fakeWriter.mode;
  const server = opts.server ?? withTeamB();
  cache.lists = Object.fromEntries(
    Object.entries(CONNECTION_SOURCE_KINDS).map(([key, kind]) => [
      kind,
      server[key as keyof ConnectionSources] as unknown[],
    ]),
  );
  const view = (sources: ConnectionSources) => (
    <GitPushesProvider>
      <SecretRefsReaderContext.Provider value={SECRETS}>
        <ContractCreate
          tab={tab}
          sources={{ ...sources, errors: [], stale: [] }}
          mode={mode}
          actions={{
            previewChange: previewContractChange,
            applyChange: applyContractChange,
          }}
        />
      </SecretRefsReaderContext.Provider>
    </GitPushesProvider>
  );
  const { rerender } = render(view(opts.page ?? withTeamB()));
  return {
    rerender: (sources: ConnectionSources) => rerender(view(sources)),
  };
}

const user = () => userEvent.setup();
const field = (label: string) =>
  screen.getByLabelText(label, { exact: true }) as HTMLInputElement;
const select = (label: string, value: string) =>
  fireEvent.change(field(label), { target: { value } });
// pick selects value in the select label of the picker group.
const pick = (group: string, label: string, value: string) =>
  fireEvent.change(
    within(screen.getByRole("group", { name: group })).getByLabelText(label),
    { target: { value } },
  );

// create opens the review and confirms it. The dry run of the fake writer
// answers with one object, so the confirm turns on.
async function create(u: ReturnType<typeof user>) {
  fakeWriter.previews = [
    {
      objects: [
        {
          ref: { kind: "ObjectStorageConfig", name: field("Name").value },
          diff: [{ op: "added", text: `name: ${field("Name").value}` }],
        } as never,
      ],
      errors: [],
    },
  ];
  await u.click(screen.getByRole("button", { name: "Review" }));
  const review = screen.getByRole("region", { name: "Review the contract" });
  const confirm = within(review).getByRole("button", { name: "Create" });
  await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
  await u.click(confirm);
  return review;
}

const applied = () => fakeWriter.applied[0].apply[0];

describe("ContractCreate", () => {
  it("creates an object storage contract in the picked namespace", async () => {
    const u = user();
    const { rerender } = setup("object-storage");

    expect(
      screen.getByRole("heading", {
        level: 1,
        name: "New object storage contract",
      }),
    ).toBeDefined();
    await u.type(field("Name"), "archive");
    select("Namespace", "team-a");
    await u.type(field("Bucket name"), "acme-archive");
    await u.type(field("Region"), "eu-west-1");
    await u.type(field("Role ARN"), "arn:aws:iam::123456789012:role/archive");
    const review = await create(u);

    expect(within(review).getByText(CREATE_EFFECT)).toBeDefined();
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    expect(fakeWriter.applied).toEqual([
      createChange("ObjectStorageConfig", "team-a", "archive", {
        spec: {
          type: "S3",
          s3: {
            bucketName: "acme-archive",
            region: "eu-west-1",
            auth: {
              type: "workloadIdentity",
              workloadIdentity: {
                roleArn: "arn:aws:iam::123456789012:role/archive",
              },
            },
          },
        },
      }),
    ]);
    expect(
      await screen.findByText("The contract archive is created."),
    ).toBeDefined();
    expect(router.push).not.toHaveBeenCalled();

    const shown = withTeamB();
    shown.objectStorageConfigs.push({
      ...shown.objectStorageConfigs[0],
      metadata: { namespace: "team-a", name: "archive" },
    });
    rerender(shown);
    await waitFor(() =>
      expect(router.push).toHaveBeenCalledWith(
        "/setup/connections/object-storage/team-a/archive",
      ),
    );
  });

  it("creates an Elasticsearch secondary storage contract with its credentials", async () => {
    const u = user();
    setup("secondary-storage");

    await u.type(field("Name"), "search-two");
    select("Namespace", "team-a");
    await u.type(field("Endpoint"), "https://search.team-a:9200");
    pick("Credentials", "Secret", "shared-es-user");
    await create(u);

    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    expect(applied().metadata).toEqual({
      name: "search-two",
      namespace: "team-a",
    });
    expect(applied().spec).toEqual({
      type: "elasticsearch",
      elasticsearch: {
        endpoint: "https://search.team-a:9200",
        credentialsSecretRef: { name: "shared-es-user" },
      },
    });
  });

  it("creates a database server contract", async () => {
    const u = user();
    setup("database-servers");

    await u.type(field("Name"), "pg-two");
    select("Namespace", "team-a");
    await u.type(field("Host"), "pg-two.team-a");
    await u.type(field("Port"), "5432");
    pick("Admin credentials", "Secret", "postgres-admin");
    await create(u);

    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    expect(applied().spec).toEqual({
      engine: "postgres",
      host: "pg-two.team-a",
      port: 5432,
      adminCredentialsSecretRef: { name: "postgres-admin" },
    });
  });

  it("creates a database contract on a database server", async () => {
    const u = user();
    setup("databases");

    await u.type(field("Name"), "orders-two");
    select("Namespace", "team-a");
    await u.type(field("Database name"), "orders2");
    select("Database server", "postgres");
    pick("Credentials", "Secret", "orders-db");
    pick("Credentials", "Username key", "user");
    pick("Credentials", "Password key", "pass");
    await create(u);

    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    expect(applied().spec).toEqual({
      databaseName: "orders2",
      serverRef: "postgres",
      credentialsSecretRef: {
        name: "orders-db",
        usernameKey: "user",
        passwordKey: "pass",
      },
    });
  });

  it("creates a Management Identity contract without a namespace", async () => {
    const u = user();
    setup("management-identity");

    // The only Namespace select is the one of the client secret.
    expect(
      screen
        .getAllByLabelText("Namespace")
        .every((s) => s.closest('[role="group"]') !== null),
    ).toBe(true);
    await u.type(field("Name"), "optimize-two");
    for (const label of [
      "Issuer URL",
      "Auth URL",
      "Token URL",
      "JWKS URL",
      "Base URL",
    ])
      await u.type(field(label), "https://login.example.com/realms/camunda");
    await u.type(field("Client ID"), "optimize");
    await u.type(field("Audience"), "optimize");
    pick("Client secret", "Namespace", "platform");
    pick("Client secret", "Secret", "plane-a-optimize-client");
    pick("Client secret", "Key", "client-secret");
    await create(u);

    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    expect(fakeWriter.applied[0].create).toEqual([
      "ManagementAuthConfig//optimize-two",
    ]);
    expect(applied().metadata).toEqual({ name: "optimize-two" });
    expect(applied().spec).toMatchObject({
      clientId: "optimize",
      clientSecretRef: {
        namespace: "platform",
        name: "plane-a-optimize-client",
        key: "client-secret",
      },
    });
  });

  it("offers the database servers of the picked namespace only", () => {
    const sources = withTeamB();
    sources.databaseServerConfigs.push({
      ...sources.databaseServerConfigs[0],
      metadata: { namespace: "team-b", name: "pg-b" },
    });
    setup("databases", { page: sources, server: sources });
    const offered = () =>
      [...(field("Database server") as unknown as HTMLSelectElement).options]
        .map((o) => o.value)
        .filter((v) => v !== "");

    select("Namespace", "team-a");
    expect(offered()).toEqual(["postgres"]);
    select("Namespace", "team-b");
    expect(offered()).toEqual(["pg-b"]);
  });

  it("writes only the block of the picked storage type", async () => {
    const u = user();
    setup("object-storage");

    await u.type(field("Name"), "docs");
    select("Namespace", "team-a");
    await u.type(field("Bucket name"), "acme");
    await u.type(field("Region"), "eu-west-1");
    await u.click(screen.getByRole("radio", { name: "GCS" }));
    await u.click(screen.getByRole("radio", { name: "Credentials" }));
    await u.type(field("Bucket name"), "acme-docs");
    pick("Credentials", "Secret", "gcs");
    pick("Credentials", "Key", "key.json");
    await create(u);

    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    expect(applied().spec).toEqual({
      type: "GCS",
      gcs: {
        bucketName: "acme-docs",
        auth: {
          type: "credentials",
          credentials: { secretRef: { name: "gcs", key: "key.json" } },
        },
      },
    });
  });

  it("refuses a name of the namespace and accepts it in another", async () => {
    const u = user();
    setup("object-storage");

    await u.type(field("Name"), "backups");
    select("Namespace", "team-a");
    expect(
      screen.getAllByText("A contract with this name exists in this namespace.")
        .length,
    ).toBeGreaterThan(0);
    expect(
      screen.getByRole("button", { name: "Review" }).hasAttribute("disabled"),
    ).toBe(true);

    select("Namespace", "team-b");
    expect(
      screen.queryByText("A contract with this name exists in this namespace."),
    ).toBeNull();
  });

  it("refuses a Management Identity name that a plane claim writes", async () => {
    const u = user();
    const sources = withTeamB();
    sources.planeClaims = [
      {
        metadata: { namespace: "platform", name: "ops" },
        spec: { managementAuthConfigName: "ops-auth" },
      } as unknown as CamundaManagementClusterClaim,
    ];
    setup("management-identity", { page: sources, server: sources });

    await u.type(field("Name"), "ops-auth");
    expect(
      screen.getAllByText(
        "The management plane platform/ops writes a contract with this name.",
      ).length,
    ).toBeGreaterThan(0);
  });

  it("stays on the page with the name when the create fails", async () => {
    const u = user();
    // The page has not seen old-backup yet. The server has.
    const page = withTeamB();
    page.objectStorageConfigs = page.objectStorageConfigs.filter(
      (c) => c.metadata.name !== "old-backup",
    );
    setup("object-storage", { page });

    await u.type(field("Name"), "old-backup");
    select("Namespace", "team-a");
    await u.type(field("Bucket name"), "acme-old");
    await u.type(field("Region"), "eu-west-1");
    // The action refuses the dry run, so the review offers no Create.
    await u.click(screen.getByRole("button", { name: "Review" }));
    const review = screen.getByRole("region", { name: "Review the contract" });

    expect(
      await within(review).findByText(
        "The ObjectStorageConfig old-backup exists already. Pick another name.",
        { exact: false },
      ),
    ).toBeDefined();
    expect(fakeWriter.previewed).toEqual([]);
    expect(fakeWriter.applied).toEqual([]);
    await u.click(within(review).getByRole("button", { name: "Back" }));
    expect(router.push).not.toHaveBeenCalled();
    expect(field("Name").closest("fieldset:disabled")).toBeNull();
    expect(field("Name").value).toBe("old-backup");
  });

  it("keeps the typed name and takes the namespace of the copy source", async () => {
    const u = user();
    setup("object-storage");

    await u.type(field("Name"), "next-bucket");
    select("Copy from", "team-a/backups");
    expect(field("Name").value).toBe("next-bucket");
    expect(field("Namespace").value).toBe("team-a");
    expect(field("Bucket name").value).toBe("acme-backups");

    select("Copy from", "");
    expect(field("Bucket name").value).toBe("");
    expect(field("Name").value).toBe("next-bucket");
    expect(field("Namespace").value).toBe("team-a");
  });

  it("Discard after a copy goes back to the copy and keeps Copy from", async () => {
    const u = user();
    setup("object-storage");

    select("Copy from", "team-a/backups");
    await u.type(field("Name"), "next-bucket");
    select("Namespace", "team-b");
    await u.type(field("Bucket name"), "-two");
    await u.click(screen.getByRole("button", { name: "Discard" }));

    expect(field("Copy from").value).toBe("team-a/backups");
    expect(field("Name").value).toBe("");
    expect(field("Namespace").value).toBe("team-a");
    expect(field("Bucket name").value).toBe("acme-backups");
  });

  it("reads no Secret before a namespace is picked", async () => {
    const read = vi.fn(SECRETS);
    render(
      <GitPushesProvider>
        <SecretRefsReaderContext.Provider value={read}>
          <ContractCreate
            tab="databases"
            sources={{ ...withTeamB(), errors: [], stale: [] }}
            mode="direct"
            actions={{
              previewChange: previewContractChange,
              applyChange: applyContractChange,
            }}
          />
        </SecretRefsReaderContext.Provider>
      </GitPushesProvider>,
    );

    expect(
      screen.getByText("Pick the namespace of the contract first."),
    ).toBeDefined();
    expect(screen.queryByRole("group", { name: "Credentials" })).toBeNull();
    select("Namespace", "team-a");
    expect(screen.getByRole("group", { name: "Credentials" })).toBeDefined();
    expect(read).toHaveBeenCalledWith("team-a");
    expect(read).not.toHaveBeenCalledWith("");
  });

  it("disables the form while changes are off", () => {
    setup("databases", { mode: undefined });

    expect(screen.getByText(CHANGES_OFF)).toBeDefined();
    expect(field("Name").closest("fieldset:disabled")).not.toBeNull();
    expect(field("Copy from").closest("fieldset:disabled")).not.toBeNull();
    expect(screen.queryByRole("button", { name: "Review" })).toBeNull();
  });
});
