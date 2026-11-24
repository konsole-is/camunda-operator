// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import type { ObjectStorageConfig } from "@/generated/crds/objectstorageconfig";
import type { ValidationError } from "@/lib/schema";
import { ContractForm } from "./contract-form";
import { type ContractDraft, draftOf, newSpec } from "./draft";
import type { ContractObject } from "./facts";
import { connectionFixtures } from "./fixtures";

vi.mock("@/lib/k8s/hooks", () => ({
  useResources: () => {
    throw new Error("the test reads Secrets through the static reader");
  },
  useWatchesFresh: () => [false],
}));

afterEach(cleanup);

const SECRETS = staticSecretRefs({
  "team-a": [
    { name: "shared-es-user", keys: ["user", "pass", "username"] },
    { name: "shared-es-ca", keys: ["ca.crt", "tls.crt"] },
    { name: "s3", keys: ["id", "secret"] },
    { name: "gcs", keys: ["key.json"] },
    { name: "postgres-admin", keys: ["username", "password"] },
    { name: "orders-db", keys: ["user", "pass"] },
  ],
  platform: [{ name: "plane-a-optimize-client", keys: ["client-secret"] }],
  "camunda-system": [{ name: "keycloak-client", keys: ["secret"] }],
});

const sources = connectionFixtures();
const find = <T extends ContractObject>(list: readonly T[], name: string) =>
  list.find((c) => c.metadata.name === name)!;

function setup(
  contract: ContractObject,
  errors: ValidationError[] = [],
  {
    choose = false,
    draft = draftOf(contract),
  }: { choose?: boolean; draft?: ContractDraft } = {},
) {
  const onChange = vi.fn<(draft: ContractDraft) => void>();
  const form = (draft: ContractDraft) => (
    <SecretRefsReaderContext.Provider value={SECRETS}>
      <ContractForm
        kind={contract.kind}
        namespace={contract.metadata.namespace}
        draft={draft}
        errors={errors}
        namespaces={["camunda-system", "platform", "team-a"]}
        databaseConfigs={["orders-db", "billing-db"]}
        databaseServerConfigs={["postgres"]}
        choose={choose}
        onChange={onChange}
      />
    </SecretRefsReaderContext.Provider>
  );
  const { rerender } = render(form(draft));
  const last = () =>
    onChange.mock.lastCall![0].spec as unknown as Record<string, never>;
  return {
    onChange,
    last,
    rerender: (next: ContractDraft) => rerender(form(next)),
  };
}

const type = (label: string, value: string) =>
  fireEvent.change(screen.getByLabelText(label), { target: { value } });
const pick = (group: string, label: string, value: string) =>
  fireEvent.change(
    within(screen.getByRole("group", { name: group })).getByRole("combobox", {
      name: label,
    }),
    { target: { value } },
  );

const s3Keys: ObjectStorageConfig = {
  ...find(sources.objectStorageConfigs, "backups"),
  spec: {
    type: "S3",
    s3: {
      bucketName: "acme",
      region: "eu-west-1",
      auth: {
        type: "credentials",
        credentials: {
          secretRef: {
            name: "s3",
            accessKeyIdKey: "id",
            secretAccessKeyKey: "secret",
          },
        },
      },
    },
  },
};
const gcsKey: ObjectStorageConfig = {
  ...s3Keys,
  spec: {
    type: "GCS",
    gcs: {
      bucketName: "acme",
      auth: {
        type: "credentials",
        credentials: { secretRef: { name: "gcs", key: "key.json" } },
      },
    },
  },
};
const azure: ObjectStorageConfig = {
  ...s3Keys,
  spec: {
    type: "AzureBlob",
    azureBlob: {
      accountName: "acme",
      container: "backups",
      auth: { type: "workloadIdentity", workloadIdentity: { clientId: "c-1" } },
    },
  },
};

describe("ContractForm", () => {
  it("changes the region of an S3 contract", () => {
    const { last } = setup(find(sources.objectStorageConfigs, "backups"));
    type("Region", "eu-central-1");
    expect(last()).toMatchObject({
      s3: { region: "eu-central-1", bucketName: "acme-backups" },
    });
  });

  it("turns on path-style access with the themed checkbox", () => {
    const { last } = setup(find(sources.objectStorageConfigs, "backups"));
    fireEvent.click(
      screen.getByRole("checkbox", { name: "Path-style access" }),
    );
    expect(last()).toMatchObject({ s3: { forcePathStyle: true } });
  });

  it("picks the S3 pair", () => {
    const { last } = setup(s3Keys);
    pick("Access key", "Secret access key key", "id");
    expect(last()).toMatchObject({
      s3: {
        auth: {
          credentials: {
            secretRef: {
              name: "s3",
              accessKeyIdKey: "id",
              secretAccessKeyKey: "id",
            },
          },
        },
      },
    });
  });

  it("changes the bucket of a GCS contract with a key", () => {
    const { last } = setup(gcsKey);
    type("Bucket name", "acme-2");
    expect(last()).toMatchObject({ gcs: { bucketName: "acme-2" } });
    expect(screen.getByRole("group", { name: "Credentials" })).toBeTruthy();
  });

  it("changes the client ID of an Azure contract with workload identity", () => {
    const { last } = setup(azure);
    type("Client ID", "c-2");
    expect(last()).toMatchObject({
      azureBlob: { auth: { workloadIdentity: { clientId: "c-2" } } },
    });
  });

  it("shows the storage type and the auth type without a control to change them", () => {
    setup(s3Keys);
    expect(
      screen.getByText("Storage type").nextElementSibling!.textContent,
    ).toBe("S3");
    expect(screen.getByText("Auth").nextElementSibling!.textContent).toBe(
      "Credentials",
    );
    expect(screen.queryByRole("radio")).toBeNull();
    expect(screen.queryByRole("combobox", { name: /type/i })).toBeNull();
  });

  it("offers the storage type and the auth type as choices on a new contract", () => {
    setup(s3Keys, [], {
      choose: true,
      draft: { spec: newSpec("ObjectStorageConfig") },
    });
    const storage = within(screen.getByRole("group", { name: "Storage type" }));
    expect(
      (storage.getByRole("radio", { name: "S3" }) as HTMLInputElement).checked,
    ).toBe(true);
    expect(storage.getByRole("radio", { name: "GCS" })).toBeTruthy();
    expect(storage.getByRole("radio", { name: "Azure Blob" })).toBeTruthy();
    const auth = within(screen.getByRole("group", { name: "Auth" }));
    expect(
      (auth.getByRole("radio", { name: "Workload identity" }) as HTMLInputElement)
        .checked,
    ).toBe(true);
    expect(auth.getByRole("radio", { name: "Credentials" })).toBeTruthy();
  });

  it("starts the GCS block empty on a pick of GCS", () => {
    const { last } = setup(s3Keys, [], {
      choose: true,
      draft: {
        spec: {
          type: "S3",
          s3: {
            bucketName: "acme",
            region: "eu-west-1",
            auth: { type: "workloadIdentity" },
          },
        },
      },
    });
    fireEvent.click(screen.getByRole("radio", { name: "GCS" }));
    expect(last()).toEqual({
      type: "GCS",
      gcs: { auth: { type: "workloadIdentity" } },
    });
  });

  it("keeps the bucket and shows the key picker on a pick of credentials", () => {
    const gcs = {
      spec: {
        type: "GCS",
        gcs: { bucketName: "acme-docs", auth: { type: "workloadIdentity" } },
      },
    } as ContractDraft;
    const { last, rerender } = setup(s3Keys, [], { choose: true, draft: gcs });
    fireEvent.click(screen.getByRole("radio", { name: "Credentials" }));
    expect(last()).toEqual({
      type: "GCS",
      gcs: { bucketName: "acme-docs", auth: { type: "credentials" } },
    });

    rerender({ spec: last() as unknown as ContractDraft["spec"] });
    expect(
      within(screen.getByRole("group", { name: "Credentials" })).getByRole(
        "combobox",
        { name: "Secret" },
      ),
    ).toBeTruthy();
  });

  it("offers Elasticsearch and RDBMS on a new secondary storage contract", () => {
    const { last } = setup(find(sources.secondaryStorageConfigs, "shared-es"), [], {
      choose: true,
      draft: { spec: newSpec("SecondaryStorageConfig") },
    });
    fireEvent.click(screen.getByRole("radio", { name: "RDBMS" }));
    expect(last()).toEqual({ type: "rdbms", rdbms: {} });
  });

  it("changes the endpoint of an Elasticsearch contract", () => {
    const { last } = setup(find(sources.secondaryStorageConfigs, "shared-es"));
    type("Endpoint", "https://search-2.example.com:9243");
    expect(last()).toMatchObject({
      elasticsearch: { endpoint: "https://search-2.example.com:9243" },
    });
  });

  it("shows the replica hint at the node count", () => {
    setup(find(sources.secondaryStorageConfigs, "shared-es"));
    expect(
      screen.getByText(
        "Sets the index replicas when the cluster sets no replica count.",
      ),
    ).toBeTruthy();
    expect(
      (screen.getByLabelText("Node count") as HTMLInputElement).value,
    ).toBe("3");
  });

  it("clears the node count", () => {
    const { last } = setup(find(sources.secondaryStorageConfigs, "shared-es"));
    type("Node count", "");
    expect(Object.hasOwn(last().elasticsearch, "nodeCount")).toBe(false);
  });

  it("picks the credentials as a name, a username key, and a password key", () => {
    const { last } = setup(find(sources.secondaryStorageConfigs, "shared-es"));
    pick("Credentials", "Username key", "");
    expect(last()).toMatchObject({
      elasticsearch: {
        credentialsSecretRef: { name: "shared-es-user", passwordKey: "pass" },
      },
    });
    expect(
      Object.hasOwn(
        (last().elasticsearch as { credentialsSecretRef: object })
          .credentialsSecretRef,
        "usernameKey",
      ),
    ).toBe(false);
  });

  it("removes the CA certificate when its Secret is set to none", () => {
    const { last } = setup(find(sources.secondaryStorageConfigs, "shared-es"));
    pick("CA certificate", "Secret", "");
    expect(Object.hasOwn(last().elasticsearch, "caSecretRef")).toBe(false);
  });

  it("keeps a new CA certificate Secret in the draft before its key is picked", () => {
    const { last } = setup(find(sources.secondaryStorageConfigs, "shared-es"));
    pick("CA certificate", "Secret", "shared-es-user");
    expect(
      (last().elasticsearch as { caSecretRef: object }).caSecretRef,
    ).toEqual({ name: "shared-es-user" });
  });

  it("picks the database config of an RDBMS contract", () => {
    const rdbms = {
      ...find(sources.secondaryStorageConfigs, "shared-es"),
      spec: {
        type: "rdbms" as const,
        rdbms: { databaseConfigRef: "orders-db" },
      },
    };
    const { last } = setup(rdbms);
    type("Database config", "billing-db");
    expect(last()).toEqual({
      type: "rdbms",
      rdbms: { databaseConfigRef: "billing-db" },
    });
  });

  it("changes the port of a database server", () => {
    const { last } = setup(sources.databaseServerConfigs[0]);
    type("Port", "6432");
    expect(last()).toMatchObject({ host: "pg.team-a", port: 6432 });
  });

  it("picks the server and the backup credentials of a database", () => {
    const { last } = setup(find(sources.databaseConfigs, "orders-db"));
    pick("Backup credentials", "Secret", "orders-db");
    expect(last()).toMatchObject({
      backupCredentialsSecretRef: { name: "orders-db" },
    });
    expect(
      screen.getByRole("combobox", { name: "Database server" }),
    ).toBeTruthy();
  });

  it("picks the client secret in another namespace", () => {
    const { last } = setup(sources.managementAuthConfigs[0]);
    pick("Client secret", "Namespace", "camunda-system");
    pick("Client secret", "Secret", "keycloak-client");
    pick("Client secret", "Key", "secret");
    expect(last()).toMatchObject({
      clientSecretRef: {
        namespace: "camunda-system",
        name: "keycloak-client",
        key: "secret",
      },
    });
  });

  it("changes the audience of a Management Identity contract", () => {
    const { last } = setup(sources.managementAuthConfigs[0]);
    type("Audience", "optimize-2");
    expect(last()).toMatchObject({
      audience: "optimize-2",
      clientId: "optimize",
    });
  });

  it("shows the message of each draft error at its field", () => {
    setup(find(sources.objectStorageConfigs, "backups"), [
      { path: "spec.s3.bucketName", message: "Enter the bucket name." },
      { path: "spec.s3.region", message: "Enter the region, or an endpoint." },
    ]);
    expect(
      screen.getByLabelText("Bucket name").getAttribute("aria-invalid"),
    ).toBe("true");
    expect(screen.getByText("Enter the bucket name.")).toBeTruthy();
    expect(screen.getByText("Enter the region, or an endpoint.")).toBeTruthy();
  });
});
