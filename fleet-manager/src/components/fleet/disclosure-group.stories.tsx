import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { type ComponentProps, useState } from "react";
import { expect, within } from "storybook/test";
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

function rowAndBody(canvasElement: HTMLElement) {
  const row = within(canvasElement).getByRole("button", { name: /^Brokers/ });
  return {
    row,
    body: document.getElementById(row.getAttribute("aria-controls")!)!,
  };
}

export const Closed: Story = {
  name: "Closed",
  args: {
    children: <input aria-label="replicas" defaultValue="3" />,
  },
  play: async ({ canvasElement }) => {
    const { row, body: closed } = rowAndBody(canvasElement);
    await expect(row).toHaveAttribute("aria-expanded", "false");
    await expect(closed.getBoundingClientRect().height).toBe(0);
    const field = closed.querySelector("input")!;
    await expect(field.checkVisibility()).toBe(false);
    field.focus();
    await expect(document.activeElement).not.toBe(field);
  },
};

export const Open: Story = {
  name: "Open",
  args: { startsOpen: true },
  play: async ({ canvasElement }) => {
    await expect(rowAndBody(canvasElement).row).toHaveAttribute(
      "aria-expanded",
      "true",
    );
    await expect(within(canvasElement).getByText(/^3 settings:/)).toBeVisible();
  },
};

export const ErrorCount: Story = {
  name: "Error count",
  args: {
    heading: true,
    counts: [
      { text: "1", label: "1 changed in the claim" },
      { text: "2 errors", bad: true },
    ],
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(
      canvas.getByRole("heading", { level: 3, name: "Brokers" }),
    ).toBeVisible();
    await expect(
      canvas.getByRole("button", {
        name: "Brokers 1 changed in the claim 2 errors",
      }),
    ).toBeVisible();
    await expect(canvas.getByText("2 errors")).toBeVisible();
  },
};
