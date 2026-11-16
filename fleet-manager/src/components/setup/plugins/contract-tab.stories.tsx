import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { contractOf } from "./contract";
import { ContractTab } from "./contract-tab";
import { pluginRegistration, registrationSources } from "./fixtures";

const sources = registrationSources();
const es = sources.registrations.find(
  (r) => r.metadata.name === "elasticsearch",
)!;

const meta = {
  title: "setup/plugins/ContractTab",
  component: ContractTab,
  args: { contract: contractOf(es, sources.sections, []) },
} satisfies Meta<typeof ContractTab>;
export default meta;
type Story = StoryObj<typeof meta>;

export const BuiltInElasticsearch: Story = { name: "Built-in elasticsearch" };

export const RequiredInput: Story = {
  name: "Required input with formats",
  args: {
    contract: contractOf(
      pluginRegistration("aws-s3", {
        sections: ["backup-storage"],
        inputs: [
          {
            name: "kmsKeyArn",
            from: "encryption-key/keyArn",
            formats: ["aws-kms-arn"],
            required: true,
          },
          { name: "principal", from: "cloud-identity/roleArn" },
        ],
        outputs: [
          {
            key: "objectStorageConfig",
            description: "The name of the ObjectStorageConfig of the bucket.",
          },
          { key: "bucketArn", format: "aws-s3-arn" },
        ],
      }),
      sources.sections,
      [],
    ),
  },
};

export const DeleteOnly: Story = {
  name: "Delete only",
  args: {
    contract: contractOf(
      pluginRegistration("rustfs", {
        sections: ["backup-storage", "removed-section"],
        deletionPolicies: ["Delete"],
      }),
      sources.sections,
      [],
    ),
  },
};
