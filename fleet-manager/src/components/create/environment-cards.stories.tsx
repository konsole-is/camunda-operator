import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { EnvironmentCards } from "./environment-cards";
import { development, legacy, production } from "./fixtures";

const meta = {
  title: "create/EnvironmentCards",
  component: EnvironmentCards,
  args: {
    environments: [production, legacy],
    counts: {
      prod: { namespaces: 2, clusters: 3 },
      dev: { namespaces: 1, clusters: 0 },
      legacy: { namespaces: 0, clusters: 0 },
    },
    onChange: fn(),
  },
} satisfies Meta<typeof EnvironmentCards>;
export default meta;
type Story = StoryObj<typeof meta>;

export const TwoEnvironmentsOnePicked: Story = {
  name: "Two environments, one picked",
  args: { environments: [production, development], value: "prod" },
};

export const NotReady: Story = { name: "Not ready" };

export const SeveralPicked: Story = {
  name: "Several picked",
  args: {
    multiple: true,
    environments: [production, development, legacy],
    value: ["dev", "prod"],
  },
};
