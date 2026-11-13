import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { parameterFields } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import type { ResolvedPolicy } from "@/lib/policy";
import type { OpenAPISchema } from "@/lib/schema";
import { secrets } from "./fixtures";
import { SelectionSettingsEditor } from "./selection-settings-editor";

const ELASTICSEARCH: OpenAPISchema = {
  type: "object",
  properties: {
    preset: { type: "string", title: "Preset" },
    releaseRef: { type: "string", title: "Release" },
    overrides: {
      type: "object",
      properties: {
        replicas: { type: "integer", format: "int32", minimum: 1 },
        resources: {
          type: "object",
          "x-kubernetes-preserve-unknown-fields": true,
        },
        storageSize: { type: "string" },
        scheduling: {
          type: "object",
          "x-kubernetes-preserve-unknown-fields": true,
        },
        podLabels: {
          type: "object",
          additionalProperties: { type: "string" },
        },
      },
    },
  },
};

const fields = parameterFields({
  cluster: {
    kind: "CamundaCluster",
    metadata: { name: "orders", labels: { [CLAIM_LABEL]: "orders" } },
    spec: {},
  } as unknown as Resource<"CamundaCluster">,
  merged: mergeCluster(undefined, undefined, {}),
  claim: {
    kind: "CamundaClusterClaim",
    metadata: { name: "orders" },
    spec: {
      presetRef: "medium",
      plugins: [
        {
          section: "secondary-storage",
          registration: "elasticsearch",
          parameters: { overrides: { replicas: 3, storageSize: "50Gi" } },
        },
      ],
    },
  } as unknown as Resource<"CamundaClusterClaim">,
  policy: {
    parameters: {
      elasticsearch: {
        preset: { value: "medium" },
        releaseRef: { value: "camunda-8-9-23" },
        overrides: {
          claimEditable: true,
          value: { replicas: 1, storageSize: "50Gi" },
        },
      },
    },
  } as unknown as ResolvedPolicy,
  sections: [],
  registrations: [
    {
      kind: "PluginRegistration",
      metadata: { name: "elasticsearch" },
      spec: { parameters: ELASTICSEARCH },
    } as unknown as Resource<"PluginRegistration">,
  ],
});

const meta = {
  title: "cluster/SelectionSettingsEditor",
  component: SelectionSettingsEditor,
  args: { secrets, set: fn(), reset: fn() },
} satisfies Meta<typeof SelectionSettingsEditor>;
export default meta;
type Story = StoryObj<typeof meta>;

export const BuiltInElasticsearchOverrides: Story = {
  name: "Built-in Elasticsearch overrides",
  args: { fields, schema: ELASTICSEARCH },
};
