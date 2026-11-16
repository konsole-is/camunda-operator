import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { PlaneField } from "@/lib/fields";
import { PlaneSettings } from "./plane-settings";

const field = (
  key: string,
  label: string,
  value: unknown,
  extra: Partial<PlaneField> = {},
): PlaneField => ({
  key,
  group: "identity",
  label,
  value,
  claimPath: key,
  claimSteps: [],
  ...extra,
});

const meta = {
  title: "planes/PlaneSettings",
  component: PlaneSettings,
  args: { fields: [] },
} satisfies Meta<typeof PlaneSettings>;
export default meta;
type Story = StoryObj<typeof meta>;

export const IdentityOidc: Story = {
  name: "Identity in OIDC mode",
  args: {
    fields: [
      field("identityProvider", "Identity provider", "oidc"),
      field("admin.claimName", "Administrator claim name", "email", {
        lock: { reason: "immutableAfterStart" },
      }),
      field("admin.claimValue", "Administrator claim value", "admin@example.com", {
        lock: { reason: "immutableAfterStart" },
      }),
      field("versions.identity", "Management Identity version", "8.9.14"),
      field(
        "url.identity",
        "Management Identity URL",
        "https://management.example.com/identity",
      ),
    ],
  },
};

export const IdentityKeycloak: Story = {
  name: "Identity in Keycloak mode",
  args: {
    fields: [
      field("identityProvider", "Identity provider", "keycloak"),
      field("admin.username", "Administrator username", "admin", {
        note: "A new username creates a second user.",
      }),
      field("admin.email", "Administrator email", "admin@example.com"),
      field("versions.identity", "Management Identity version", "8.9.14"),
    ],
  },
};

export const WebModeler: Story = {
  name: "Web Modeler with mail",
  args: {
    fields: [
      field("webModeler", "Web Modeler", true),
      field("webModeler.mail", "Web Modeler mail", {
        fromAddress: "modeler@example.com",
        smtpHost: "smtp.example.com",
      }),
      field("versions.webModeler", "Web Modeler version", "8.9.15"),
      field(
        "url.webModeler",
        "Web Modeler URL",
        "https://management.example.com/modeler",
      ),
      field(
        "url.webModelerWebsockets",
        "Web Modeler websockets URL",
        "https://management.example.com/modeler-ws",
        { lock: { reason: "section" } },
      ),
    ],
  },
};
