import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { EventList } from "./event-list";

const meta = {
  title: "fleet/EventList",
  component: EventList,
  args: { now: "2026-10-15T12:00:00Z", events: [] },
} satisfies Meta<typeof EventList>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Events: Story = {
  name: "Events",
  args: {
    events: [
      {
        type: "Normal",
        reason: "Created",
        note: "Created the StatefulSet orders-zeebe",
        eventTime: "2026-10-15T09:00:00Z",
      },
      {
        type: "Warning",
        reason: "BackOff",
        note: "Back-off restarting the failed container",
        eventTime: "2026-10-15T11:50:00Z",
        series: { count: 3, lastObservedTime: "2026-10-15T11:59:00Z" },
      },
    ],
  },
};

export const Empty: Story = { name: "Empty" };
