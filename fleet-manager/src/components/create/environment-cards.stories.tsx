import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { fn } from "storybook/test";
import {
  EnvironmentCards,
  type EnvironmentCardsProps,
} from "./environment-cards";
import { development, legacy, production } from "./fixtures";

// The create flow holds the picked environments, so the story holds them too.
function EnvironmentCardsHarness(props: EnvironmentCardsProps) {
  const [picked, setPicked] = useState(() => [props.value ?? []].flat());
  if (props.multiple) {
    const { onChange } = props;
    return (
      <EnvironmentCards
        {...props}
        value={picked}
        onChange={(names: string[]) => {
          onChange?.(names);
          setPicked(names);
        }}
      />
    );
  }
  const { onChange } = props;
  return (
    <EnvironmentCards
      {...props}
      value={picked[0]}
      onChange={(name: string) => {
        onChange?.(name);
        setPicked([name]);
      }}
    />
  );
}

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
  render: (args) => (
    <EnvironmentCardsHarness key={JSON.stringify(args.value)} {...args} />
  ),
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
