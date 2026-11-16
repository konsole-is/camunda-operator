import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Resource } from "@/lib/k8s/kinds";
import { BuiltinStorage } from "./builtin-storage";
import { fixtureCondition, requestNow } from "./request-fixtures";

const meta = {
  title: "cluster/BuiltinStorage",
  component: BuiltinStorage,
  args: { unread: [], now: requestNow },
} satisfies Meta<typeof BuiltinStorage>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ElasticsearchCluster: Story = {
  name: "ElasticsearchCluster",
  args: {
    storage: {
      kind: "ElasticsearchCluster",
      name: "orders-es",
      object: {
        apiVersion: "core.camunda.io/v1",
        kind: "ElasticsearchCluster",
        metadata: { namespace: "team-a", name: "orders-es" },
        status: {
          version: "8.19.2",
          snapshotRepository: "camunda",
          volumes: [
            { name: "data-orders-es-0", capacity: "32Gi" },
            { name: "data-orders-es-1", capacity: "32Gi" },
          ],
          conditions: [fixtureCondition("Ready", "True", "Healthy")],
        },
      } as unknown as Resource<"ElasticsearchCluster">,
    },
  },
};

export const DatabaseServerWithArchive: Story = {
  name: "DatabaseServer with an archive",
  args: {
    storage: {
      kind: "DatabaseServer",
      name: "orders-db",
      object: {
        apiVersion: "core.camunda.io/v1",
        kind: "DatabaseServer",
        metadata: { namespace: "team-a", name: "orders-db" },
        status: {
          version: "17.5",
          archive: { reachableFrom: "2026-11-10T09:58:00Z" },
          recovery: {
            requestID: "3f2b1c4d-5e6a-4b7c-8d9e-0f1a2b3c4d5e",
            targetTime: "2026-11-15T07:30:00Z",
            startedAt: "2026-11-15T07:31:00Z",
            contract: "orders-db",
            requestedBy: "orders",
          },
          volumes: [{ name: "orders-db-1", capacity: "10Gi" }],
          conditions: [
            fixtureCondition(
              "Ready",
              "False",
              "ArchiveFailing",
              "The plugin could not upload the write-ahead log for 6 minutes",
            ),
          ],
        },
      } as unknown as Resource<"DatabaseServer">,
    },
  },
};

export const NotReadYet: Story = {
  name: "ElasticsearchCluster not read",
  args: {
    storage: { kind: "ElasticsearchCluster", name: "orders-es" },
    unread: ["ElasticsearchCluster"],
  },
};
