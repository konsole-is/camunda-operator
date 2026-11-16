import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Button } from "@/components/ui/button";
import { PlaneHeader } from "./plane-header";

const meta = {
  title: "planes/PlaneHeader",
  component: PlaneHeader,
} satisfies Meta<typeof PlaneHeader>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OidcReady: Story = {
  name: "OIDC ready",
  args: {
    name: "plane-a",
    namespace: "platform",
    ready: { status: "True", reason: "Healthy" },
    mode: "OIDC",
    platformConfig: "oidc-platform",
    serves: { environments: ["platform", "staging"] },
  },
};

export const KeycloakNotReady: Story = {
  name: "Keycloak not ready",
  args: {
    name: "plane-b",
    namespace: "platform",
    ready: {
      status: "False",
      reason: "KeycloakNotReady",
      message: "0 of 1 Keycloak pods are ready",
    },
    mode: "Keycloak",
    platformConfig: "default-platform",
    serves: { environments: ["production"] },
    actions: (
      <Button variant="outline" size="sm">
        Suspend
      </Button>
    ),
  },
};

export const Suspended: Story = {
  name: "Suspended",
  args: {
    name: "plane-c",
    namespace: "platform",
    ready: { status: "True", reason: "Suspended" },
    mode: "External Keycloak",
    platformConfig: "default-platform",
    serves: { environments: ["platform"] },
  },
};

export const UnmanagedWithSelector: Story = {
  name: "Unmanaged with a selector",
  args: {
    name: "legacy-plane",
    namespace: "camunda-system",
    ready: { status: "True", reason: "Healthy" },
    mode: "Keycloak",
    serves: { selector: "team in (orders,payments), !legacy" },
  },
};
