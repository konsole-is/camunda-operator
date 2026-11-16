import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { PlaneConditions } from "./plane-conditions";

const now = "2026-11-15T12:00:00Z";

const healthy = (type: string, lastTransitionTime = now) =>
  ({ type, status: "True", reason: "Healthy", lastTransitionTime }) as const;
const disabled = (type: string, message: string) =>
  ({
    type,
    status: "True",
    reason: "Disabled",
    message,
    lastTransitionTime: now,
  }) as const;

const claim = [
  healthy("Ready"),
  healthy("RequestsReady"),
  healthy("ManagementClusterReady"),
];

const meta = {
  title: "planes/PlaneConditions",
  component: PlaneConditions,
} satisfies Meta<typeof PlaneConditions>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OidcReady: Story = {
  name: "OIDC ready",
  args: {
    plane: [
      healthy("Ready"),
      disabled("SecretsReady", "Component is disabled."),
      healthy("MirroredSecretsReady", "2026-11-14T09:00:00Z"),
      healthy("ServiceAccountsReady"),
      healthy("ManagementAuthReady"),
      disabled(
        "OptimizeCallbacksReady",
        "The identity provider of the platform config holds the callback URLs of Optimize",
      ),
      healthy("IdentityReady"),
    ],
    claim,
    now,
  },
};

export const FailingManagementAuth: Story = {
  name: "Failing ManagementAuthReady",
  args: {
    plane: [
      healthy("SecretsReady"),
      healthy("MirroredSecretsReady"),
      healthy("ServiceAccountsReady"),
      {
        type: "ManagementAuthReady",
        status: "False",
        reason: "InvalidReference",
        message:
          'CamundaPlatformConfig "oidc-platform" sets no spec.auth.oidc.jwksUrl',
        lastTransitionTime: "2026-11-15T11:50:00Z",
      },
      healthy("OptimizeCallbacksReady"),
    ],
    claim: [
      healthy("RequestsReady"),
      {
        type: "ManagementClusterReady",
        status: "False",
        reason: "Creating",
        message: "InvalidReference: the platform config sets no JWKS URL",
        lastTransitionTime: "2026-11-15T11:50:00Z",
      },
    ],
    now,
  },
};

export const ClaimOnly: Story = {
  name: "Claim only",
  args: {
    claim: [
      {
        type: "RequestsReady",
        status: "False",
        reason: "Pending",
        message: "The management-database request waits for its plugin",
        lastTransitionTime: now,
      },
      {
        type: "ManagementClusterReady",
        status: "Unknown",
        reason: "Creating",
        message:
          "The claim controller has not created the management plane yet",
        lastTransitionTime: now,
      },
    ],
    now,
  },
};
