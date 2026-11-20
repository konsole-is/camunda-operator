import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { planeFields } from "@/lib/fields";
import type { PlaneClaim } from "@/lib/session";
import { PlaneClaimTab } from "./plane-claim-tab";

const claim = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaManagementClusterClaim",
  metadata: { namespace: "platform", name: "mgmt", resourceVersion: "3" },
  spec: {
    environments: ["production", "staging"],
    management: {
      identityProvider: {
        externalKeycloak: {
          url: "https://keycloak.example.com/auth",
          realm: "camunda",
          adminCredentialsSecretRef: { name: "keycloak-admin" },
        },
      },
      identity: {
        admin: { username: "admin" },
        externalUrl: "https://mgmt.example.com/identity",
      },
      console: { externalUrl: "https://mgmt.example.com/console" },
    },
    versions: { identity: "8.9.14", console: "8.9.133" },
  },
} as unknown as PlaneClaim;

const meta = {
  title: "planes/claim/Claim tab",
  component: PlaneClaimTab,
  args: {
    fields: planeFields({ claim, registrations: [] }),
    stored: claim,
    urls: {},
  },
  decorators: [
    (Story) => (
      <div className="max-w-5xl">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof PlaneClaimTab>;
export default meta;
type Story = StoryObj<typeof meta>;

export const NoAccessFromOutside: Story = {
  name: "No access from outside, with typed URLs",
};
