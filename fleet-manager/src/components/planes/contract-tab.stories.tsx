import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Resource } from "@/lib/k8s/kinds";
import { ContractTab } from "./contract-tab";

const realm = "https://login.example.com/realms/camunda";

const contractOf = (issuerBackendUrl?: string) =>
  ({
    apiVersion: "core.camunda.io/v1",
    kind: "ManagementAuthConfig",
    metadata: { name: "production" },
    spec: {
      baseUrl: "http://production-identity.platform:80",
      issuerUrl: realm,
      ...(issuerBackendUrl && { issuerBackendUrl }),
      authUrl: `${realm}/protocol/openid-connect/auth`,
      tokenUrl: `${realm}/protocol/openid-connect/token`,
      jwksUrl: `${realm}/protocol/openid-connect/certs`,
      clientId: "optimize",
      audience: "optimize-api",
      clientSecretRef: {
        namespace: "platform",
        name: "production-optimize-client",
        key: "client-secret",
      },
    },
  }) as unknown as Resource<"ManagementAuthConfig">;

const reader = (
  namespace: string,
  name: string,
  cluster: string,
  externalUrl?: string,
) =>
  ({
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaOptimize",
    metadata: { namespace, name },
    spec: {
      clusterRef: { name: cluster },
      managementAuthRef: "production",
      ...(externalUrl && { externalUrl }),
    },
  }) as unknown as Resource<"CamundaOptimize">;

const meta = {
  title: "planes/ContractTab",
  component: ContractTab,
} satisfies Meta<typeof ContractTab>;
export default meta;
type Story = StoryObj<typeof meta>;

export const WithIssuerInsideTheCluster: Story = {
  name: "With issuer inside the cluster",
  args: {
    name: "production",
    contract: contractOf("http://keycloak.platform:8080/auth/realms/camunda"),
    readers: [
      reader("team-lab", "lab-optimize", "lab"),
      reader(
        "team-orders",
        "orders-optimize",
        "orders",
        "https://orders.example.com/optimize",
      ),
    ],
  },
};

export const WithoutIssuerInsideTheCluster: Story = {
  name: "Without issuer inside the cluster",
  args: { name: "production", contract: contractOf(), readers: [] },
};

export const NoContractYet: Story = {
  name: "No contract yet",
  args: { name: undefined, contract: undefined, readers: [] },
};

export const ContractMissing: Story = {
  name: "Contract missing",
  args: { name: "production", contract: undefined, readers: [] },
};

export const ContractsNotRead: Story = {
  name: "Contracts not read",
  args: {
    name: "production",
    contract: undefined,
    readers: [],
    unread: ["ManagementAuthConfig", "CamundaOptimize"],
  },
};
