import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { Resource } from "@/lib/k8s/kinds";
import { endpointRows } from "./endpoints";
import { EndpointsTab } from "./endpoints-tab";

const route = (
  name: string,
  pathPrefix: string,
  service = "orders-gateway",
) => ({
  name,
  pathPrefix,
  port: 8080,
  protocol: "HTTP" as const,
  service,
});

const cluster = {
  kind: "CamundaCluster",
  metadata: { namespace: "team-a", name: "orders" },
  spec: { presetRef: "medium", externalUrl: "https://orders.example.com" },
  status: {
    endpoints: [
      route("admin", "/admin"),
      route("connectors", "/inbound", "orders-connectors"),
      route("default", "/"),
      {
        name: "grpc",
        port: 26500,
        protocol: "GRPC",
        service: "orders-gateway",
      },
      route("operate", "/operate"),
      route("rest", "/v2"),
      route("tasklist", "/tasklist"),
    ],
  },
} as Resource<"CamundaCluster">;

const optimize = {
  kind: "CamundaOptimize",
  metadata: { namespace: "team-a", name: "orders-optimize" },
  spec: {
    clusterRef: { name: "orders" },
    externalUrl: "https://orders.example.com/optimize",
  },
  status: {
    endpoints: [route("optimize", "/optimize", "orders-optimize-webapp")],
  },
} as unknown as Resource<"CamundaOptimize">;

const meta = {
  title: "cluster/EndpointsTab",
  component: EndpointsTab,
} satisfies Meta<typeof EndpointsTab>;
export default meta;
type Story = StoryObj<typeof meta>;

export const EmbeddedAppsAndAnOptimize: Story = {
  name: "Embedded apps and an Optimize",
  args: {
    rows: endpointRows({
      cluster,
      spec: { connectors: { enabled: true } } as CamundaClusterSpec,
      optimizes: [optimize],
      accessOutputs: { grpcHost: "grpc.orders.example.com:443" },
    }),
  },
};

export const NoEndpointsYet: Story = {
  name: "No endpoints yet",
  args: { rows: [] },
};
