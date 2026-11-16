import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "./namespaced-secret-ref-picker";
import {
  CREDENTIALS_SLOTS,
  S3_CREDENTIALS_SLOTS,
  SecretKeysPicker,
} from "./secret-keys-picker";

// A story must not read live Secrets, so each story reads fixed lists.
const SECRETS = staticSecretRefs({
  "team-a": [
    { name: "shared-es-user", keys: ["user", "pass"] },
    { name: "s3-keys", keys: ["access-key-id", "secret-access-key"] },
  ],
  empty: [],
});

const meta = {
  title: "fleet/SecretKeysPicker",
  component: SecretKeysPicker,
  args: {
    label: "Credentials",
    namespace: "team-a",
    slots: CREDENTIALS_SLOTS,
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
    <SecretKeysPicker key={JSON.stringify(args.value)} {...args} />
  ),
} satisfies Meta<typeof SecretKeysPicker>;
export default meta;
type Story = StoryObj<typeof meta>;

export const CredentialsWithDefaultKeys: Story = {
  name: "Credentials with default keys",
  args: { value: { name: "shared-es-user", passwordKey: "pass" } },
};

export const S3KeyPair: Story = {
  name: "S3 key pair",
  args: {
    label: "Access key",
    slots: S3_CREDENTIALS_SLOTS,
    value: {
      name: "s3-keys",
      accessKeyIdKey: "access-key-id",
      secretAccessKeyKey: "secret-access-key",
    },
  },
};

export const ASecretThatIsGone: Story = {
  name: "A Secret that is gone",
  args: { value: { name: "old-es-user", usernameKey: "user" } },
};

export const NoSecret: Story = {
  name: "No Secret",
  args: { namespace: "empty" },
};
