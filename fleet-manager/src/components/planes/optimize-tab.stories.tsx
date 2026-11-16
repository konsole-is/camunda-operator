import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { OptimizeTab } from "./optimize-tab";
import type { OptimizeRow } from "./plane-optimize";

const row = (namespace: string, name: string, host: string): OptimizeRow => ({
  namespace,
  name,
  externalUrl: `https://${host}/optimize`,
  callbackUrl: `https://${host}/optimize/api/authentication/callback`,
});

const callbacks = (status: "True" | "False", reason: string, message = "") => ({
  type: "OptimizeCallbacksReady",
  status,
  reason,
  message,
});

const meta = {
  title: "planes/OptimizeTab",
  component: OptimizeTab,
} satisfies Meta<typeof OptimizeTab>;
export default meta;
type Story = StoryObj<typeof meta>;

export const KeycloakModeRegistered: Story = {
  name: "Keycloak mode, registered",
  args: {
    rows: [
      row("team-orders", "orders-optimize", "orders.example.com"),
      row("team-billing", "billing-optimize", "billing.example.com"),
    ],
    callbacks: callbacks("True", "Healthy"),
    oidc: false,
  },
};

export const KeycloakModeFailed: Story = {
  name: "Keycloak mode, failed",
  args: {
    rows: [row("team-orders", "orders-optimize", "orders.example.com")],
    callbacks: callbacks(
      "False",
      "ConnectionFailed",
      "The operator could not reach Keycloak at http://production-keycloak.platform:80/auth to register the login callbacks of Optimize",
    ),
    oidc: false,
  },
};

export const OidcMode: Story = {
  name: "OIDC mode",
  args: {
    rows: [row("team-orders", "orders-optimize", "orders.example.com")],
    callbacks: callbacks(
      "True",
      "Disabled",
      "The identity provider of the platform config holds the callback URLs of Optimize",
    ),
    oidc: true,
  },
};

export const NoOptimize: Story = {
  name: "No Optimize",
  args: {
    rows: [],
    callbacks: callbacks("True", "NoCallbacks"),
    oidc: false,
  },
};
