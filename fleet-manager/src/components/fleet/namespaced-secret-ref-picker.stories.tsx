import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import {
  NamespacedSecretRefPicker,
  SecretRefsReaderContext,
  staticSecretRefs,
} from "./namespaced-secret-ref-picker";

// A story must not read live Secrets, so each story reads fixed lists.
const SECRETS = staticSecretRefs({
  "camunda-system": [
    { name: "oidc-credentials", keys: ["client-secret", "client-id"] },
    { name: "smtp", keys: ["token"] },
  ],
  "team-a": [{ name: "orders-oidc", keys: ["secret"] }],
});

const meta = {
  title: "fleet/NamespacedSecretRefPicker",
  component: NamespacedSecretRefPicker,
  args: {
    label: "Client secret",
    namespaces: ["camunda-system", "team-a"],
    onChange: fn(),
  },
  decorators: [
    (Story) => (
      <SecretRefsReaderContext.Provider value={SECRETS}>
        <div className="max-w-2xl">
          <Story />
        </div>
      </SecretRefsReaderContext.Provider>
    ),
  ],
  render: (args) => (
    <NamespacedSecretRefPicker key={JSON.stringify(args.value)} {...args} />
  ),
} satisfies Meta<typeof NamespacedSecretRefPicker>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Picked: Story = {
  name: "Picked",
  args: {
    value: {
      namespace: "camunda-system",
      name: "oidc-credentials",
      key: "client-secret",
    },
  },
};

export const MissingSecret: Story = {
  name: "Missing Secret",
  args: {
    label: "License key",
    value: {
      namespace: "camunda-system",
      name: "camunda-license",
      key: "license-key",
    },
  },
};

export const NothingPicked: Story = { name: "Nothing picked" };
