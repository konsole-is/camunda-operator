import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Resource } from "@/lib/k8s/kinds";
import { RequestOverview } from "./request-detail";
import {
  coreSections,
  fixtureClaim,
  fixtureComponents,
  fixtureCondition,
  fixtureContract,
  fixtureGrant,
  fixtureRegistrations,
  fixtureRequest,
  fixtureRequests,
  fixtureSelections,
  requestNow,
} from "./request-fixtures";
import { type RequestDetail, requestDetail } from "./requests";

// detailOf reads the request of section of the fixture claim, as the
// cluster page does.
function detailOf(
  section: string,
  over: {
    request?: Resource<"PluginRequest">;
    grants?: Resource<"Grant">[];
    deletionPolicy?: "Delete" | "Orphan";
  } = {},
): RequestDetail {
  const claim = fixtureClaim(fixtureSelections, over.deletionPolicy);
  const requests = fixtureRequests.map((r) =>
    over.request && r.spec.section === over.request.spec.section
      ? over.request
      : r,
  );
  const components = fixtureComponents(claim, requests);
  return requestDetail({
    component: components.find((c) => c.section === section)!,
    claim,
    cluster: "orders",
    sections: coreSections,
    registrations: fixtureRegistrations,
    components,
    grants: over.grants ?? [],
    contracts: { SecondaryStorageConfig: [fixtureContract("orders-storage")] },
  })!;
}

const ready = [
  fixtureCondition("Applied", "True", "Healthy"),
  fixtureCondition("MatchesTemplate", "True", "Healthy"),
  fixtureCondition("Ready", "True", "Healthy"),
];

const meta = {
  title: "cluster/RequestOverview",
  component: RequestOverview,
  args: { unread: [], now: requestNow },
} satisfies Meta<typeof RequestOverview>;
export default meta;
type Story = StoryObj<typeof meta>;

export const CloudIdentityWithGrants: Story = {
  name: "Cloud identity with the grants of the cluster",
  args: {
    detail: detailOf("cloud-identity", {
      grants: [
        fixtureGrant("orders-read", "backup-storage", ready),
        fixtureGrant("orders-key", "encryption-key"),
      ],
    }),
  },
};

export const BackupStorageWithARejectedGrant: Story = {
  name: "Backup storage with a rejected grant",
  args: {
    detail: detailOf("backup-storage", {
      request: fixtureRequest("backup-storage", {
        summary: [{ label: "Bucket", value: "orders-backups" }],
        conditions: [
          fixtureCondition("OutputsReady", "True", "Healthy"),
          fixtureCondition(
            "Ready",
            "False",
            "GrantRejected",
            "Grant orders-read has Ready=False",
          ),
        ],
      }),
      grants: [
        fixtureGrant("orders-read", "backup-storage", [
          fixtureCondition(
            "Applied",
            "False",
            "OverCeiling",
            "s3:DeleteObject is not in the ceiling of the identity",
          ),
          fixtureCondition("MatchesTemplate", "True", "Healthy"),
          fixtureCondition(
            "Ready",
            "False",
            "OverCeiling",
            "s3:DeleteObject is not in the ceiling of the identity",
          ),
        ]),
      ],
    }),
  },
};

export const GrantWithAnOldTemplate: Story = {
  name: "A grant applied with the old template",
  args: {
    detail: detailOf("backup-storage", {
      grants: [
        fixtureGrant("orders-read", "backup-storage", [
          fixtureCondition("Applied", "True", "Healthy"),
          fixtureCondition(
            "MatchesTemplate",
            "False",
            "TemplateChanged",
            "The fragment no longer matches its template",
          ),
          fixtureCondition(
            "Ready",
            "False",
            "TemplateChanged",
            "The fragment no longer matches its template",
          ),
        ]),
      ],
    }),
  },
};

export const EncryptionKeyUsedByTwoSections: Story = {
  name: "Encryption key used by two sections",
  args: { detail: detailOf("encryption-key") },
};

export const ConnectorsPermissions: Story = {
  name: "Connectors permissions",
  args: { detail: detailOf("connectors-permissions") },
};

export const AccessWaitingForEndpoints: Story = {
  name: "Access waiting for endpoints",
  args: {
    detail: detailOf("access", {
      request: fixtureRequest("access", {
        summary: [
          {
            label: "Host",
            value: "Waiting for the Gateway",
            severity: "Warning",
          },
        ],
        conditions: [
          fixtureCondition("OutputsReady", "False", "WaitingForEndpoints"),
          fixtureCondition("Ready", "False", "WaitingForEndpoints"),
        ],
      }),
    }),
  },
};

export const ExistingSecondaryStorageConnected: Story = {
  name: "Existing secondary storage, connected",
  args: { detail: detailOf("secondary-storage") },
};

export const ResourcesKeptAndDeleted: Story = {
  name: "Resources kept and deleted",
  args: { detail: detailOf("backup-storage", { deletionPolicy: "Orphan" }) },
};

export const NotRead: Story = {
  name: "Grants and the claim not read",
  args: {
    detail: detailOf("backup-storage", {
      grants: [fixtureGrant("orders-read", "backup-storage", ready)],
    }),
    unread: ["Grant", "CamundaClusterClaim"],
  },
};
