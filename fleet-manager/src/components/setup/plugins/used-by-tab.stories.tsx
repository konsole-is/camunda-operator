import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import {
  condition,
  pluginGrant,
  pluginRequest,
  registrationSources,
} from "./fixtures";
import { usedByRows } from "./used-by";
import { UsedByTab } from "./used-by-tab";

const healthy = [
  condition("OutputsReady", "True", "Healthy"),
  condition("Ready", "True", "Healthy"),
] as never;

const esRows = usedByRows(
  "elasticsearch",
  registrationSources({
    requests: [
      pluginRequest(
        "team-orders",
        "orders-secondary-storage",
        {
          registration: "elasticsearch",
          section: "secondary-storage",
          claimRef: { kind: "CamundaClusterClaim", name: "orders" },
          target: { kind: "CamundaCluster", name: "orders" },
          deletionPolicy: "Orphan",
        },
        {
          conditions: healthy,
          summary: [
            { label: "Health", value: "green", severity: "Info" },
            { label: "Nodes", value: "3" },
          ],
          resources: [
            {
              kind: "ElasticsearchCluster",
              name: "orders-es",
              role: "Search and secondary storage",
              onOrphan: "Kept",
            },
            {
              kind: "SecondaryStorageConfig",
              name: "orders-es",
              onOrphan: "Kept",
            },
          ],
        },
      ),
    ],
  }),
  [],
);

const s3Rows = usedByRows(
  "aws-s3",
  registrationSources({
    requests: [
      pluginRequest(
        "team-orders",
        "orders-backup-storage",
        {
          registration: "aws-s3",
          claimRef: { kind: "CamundaClusterClaim", name: "orders" },
          target: { kind: "CamundaCluster", name: "orders" },
        },
        {
          conditions: [
            condition("OutputsReady", "True", "Healthy"),
            condition(
              "Ready",
              "False",
              "GrantRejected",
              "Grant orders-bucket-access is refused: OverCeiling",
            ),
          ] as never,
          summary: [
            { label: "Bucket", value: "cpo-orders-backups", severity: "Info" },
          ],
          resources: [
            {
              kind: "S3Bucket",
              name: "cpo-orders-backups",
              location: "eu-west-1",
              onOrphan: "Kept",
            },
          ],
        },
      ),
    ],
  }),
  [
    pluginGrant(
      "team-orders",
      "orders-bucket-access",
      { template: "bucket-access" },
      {
        "plugins.camunda.io/registration": "aws-s3",
        "plugins.camunda.io/request": "orders-backup-storage",
      },
      [condition("Ready", "False", "OverCeiling")],
    ),
  ],
);

const planeRows = usedByRows(
  "management-database-server",
  registrationSources({
    requests: [
      pluginRequest(
        "team-orders",
        "ops-management-database",
        {
          registration: "management-database-server",
          claimRef: { kind: "CamundaManagementClusterClaim", name: "ops" },
          target: { kind: "CamundaManagementCluster", name: "ops" },
        },
        {
          conditions: healthy,
          summary: [{ label: "Databases", value: "3 of 3 ready" }],
        },
      ),
    ],
  }),
  [],
);

const meta = {
  title: "setup/plugins/UsedByTab",
  component: UsedByTab,
  args: { rows: esRows, unread: [] },
} satisfies Meta<typeof UsedByTab>;
export default meta;
type Story = StoryObj<typeof meta>;

export const BuiltInElasticsearch: Story = {
  name: "Built-in elasticsearch with its resources",
};

export const RefusedGrant: Story = {
  name: "A request with a refused Grant",
  args: { rows: s3Rows },
};

export const ManagementPlane: Story = {
  name: "A management plane request",
  args: { rows: planeRows },
};

export const NoRequest: Story = { name: "No request", args: { rows: [] } };
