import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fleetEnvironment, pluginRegistration } from "./fixtures";
import { parameterRows } from "./parameters";
import { ParametersTab } from "./parameters-tab";

const kms = pluginRegistration("aws-kms", {
  parameters: {
    type: "object",
    properties: {
      deletionWindowDays: {
        type: "integer",
        minimum: 7,
        maximum: 30,
        description: "The days between the delete of a key and its removal.",
      },
      multiRegion: {
        type: "boolean",
        description: "Makes a key that other regions can use.",
      },
      keySpec: { type: "string", enum: ["SYMMETRIC_DEFAULT", "RSA_2048"] },
    },
  },
  parameterValues: { deletionWindowDays: 30, multiRegion: false },
  claimEditable: ["multiRegion"],
});
const environments = [
  fleetEnvironment("production", {
    displayName: "Production",
    parameters: {
      "aws-kms": {
        deletionWindowDays: { value: 14 },
        keySpec: { claimEditable: true },
      },
    },
  }),
  fleetEnvironment("staging", { displayName: "Staging" }),
];

const meta = {
  title: "setup/plugins/ParametersTab",
  component: ParametersTab,
  args: { rows: parameterRows(kms, environments, []), unread: [] },
} satisfies Meta<typeof ParametersTab>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ThreeParameters: Story = {
  name: "Three parameters and two environments",
};

export const NoParameters: Story = {
  name: "No parameters",
  args: { rows: parameterRows(pluginRegistration("rustfs", {}), [], []) },
};

export const BadValues: Story = {
  name: "Parameter values that are not an object",
  args: {
    rows: parameterRows(
      pluginRegistration("aws-kms", { parameterValues: [30] }),
      [],
      [],
    ),
  },
};
