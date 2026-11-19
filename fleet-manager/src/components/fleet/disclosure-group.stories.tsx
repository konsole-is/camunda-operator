import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { type ComponentProps, useState } from "react";
import { expect, userEvent, within } from "storybook/test";
import { DisclosureGroup } from "./disclosure-group";

// Harness holds the open state that DisclosureGroup leaves to its caller.
function Harness({
  startsOpen = false,
  ...props
}: Omit<ComponentProps<typeof DisclosureGroup>, "open" | "onToggle"> & {
  startsOpen?: boolean;
}) {
  const [open, setOpen] = useState(startsOpen);
  return <DisclosureGroup {...props} open={open} onToggle={setOpen} />;
}

const meta = {
  title: "fleet/DisclosureGroup",
  component: Harness,
  args: {
    label: "Brokers",
    counts: [{ text: "0" }],
    children: (
      <p className="text-sm text-muted">
        3 settings: 2 from the preset, 1 default value
      </p>
    ),
  },
} satisfies Meta<typeof Harness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Closed: Story = {
  name: "Closed",
  play: async ({ canvasElement }) => {
    const row = within(canvasElement).getByRole("button", {
      name: "Brokers 0",
    });
    await expect(row).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(row);
    await expect(row).toHaveAttribute("aria-expanded", "true");
    await expect(within(canvasElement).getByText(/^3 settings:/)).toBeVisible();
  },
};

export const Open: Story = {
  name: "Open",
  args: { startsOpen: true },
};

export const ErrorCount: Story = {
  name: "Error count",
  args: {
    heading: true,
    counts: [
      { text: "1", label: "1 changed in the claim" },
      { text: "2 errors", label: "2 settings have an error", bad: true },
    ],
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(
      canvas.getByRole("heading", { level: 3, name: "Brokers" }),
    ).toBeVisible();
    await expect(
      canvas.getByRole("button", {
        name: "Brokers 1 changed in the claim 2 settings have an error",
      }),
    ).toBeVisible();
    await expect(canvas.getByText("2 errors")).toBeVisible();
  },
};
