import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { presetDraftOf } from "./draft";
import { presetFixtures } from "./fixtures";
import { PresetEditor } from "./preset-editor";

const sources = presetFixtures();
const medium = sources.clusterPresets[0];
const es = sources.elasticsearchPresets[0];
const pg = sources.databasePresets[1];

const meta = {
  title: "setup/presets/PresetEditor",
  component: PresetEditor,
  args: {
    kind: "CamundaClusterPreset",
    draft: presetDraftOf(medium),
    live: medium.spec.cluster,
    secrets: {},
    onDraft: fn(),
    onErrors: fn(),
  },
} satisfies Meta<typeof PresetEditor>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ClusterPreset: Story = { name: "Cluster preset" };

export const ElasticsearchPresetAsYAML: Story = {
  name: "Elasticsearch preset as YAML",
  args: {
    kind: "ElasticsearchClusterPreset",
    draft: presetDraftOf(es),
    live: es.spec.cluster,
  },
};

export const PostgreSQLPresetAsYAML: Story = {
  name: "PostgreSQL preset as YAML",
  args: {
    kind: "DatabaseServerPreset",
    draft: presetDraftOf(pg),
    live: pg.spec.server,
  },
};

export const AMissingSecretKey: Story = {
  name: "A missing Secret key",
  args: {
    kind: "ElasticsearchClusterPreset",
    draft: {
      description: "",
      body: {
        ...es.spec.cluster,
        extraEnv: [
          {
            name: "S3_SECRET",
            valueFrom: {
              secretKeyRef: { name: "snapshot-credentials", key: "secret" },
            },
          },
        ],
      },
    },
    secrets: {
      "team-a": [{ name: "snapshot-credentials", keys: ["secret"] }],
      "team-b": [{ name: "snapshot-credentials", keys: ["access"] }],
    },
  },
};
