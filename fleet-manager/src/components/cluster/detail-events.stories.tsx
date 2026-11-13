import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { EventList } from "@/components/fleet/event-list";
import type { Resource } from "@/lib/k8s/kinds";
import { now } from "./fixtures";

const event = (
  reason: string,
  note: string,
  regarding: { kind: string; name: string },
  minutesAgo: number,
  type = "Normal",
) =>
  ({
    metadata: {
      name: `${regarding.name}.${reason}`,
      uid: `${regarding.name}/${reason}`,
    },
    type,
    reason,
    note,
    eventTime: new Date(Date.parse(now) - minutesAgo * 60_000).toISOString(),
    regarding: { ...regarding, namespace: "team-a" },
  }) as Resource<"Event">;

// DetailEvents reads its events live, so the story shows the EventList
// that it renders.
const meta = {
  title: "cluster/DetailEvents",
  component: EventList,
  args: { now, showObject: true },
} satisfies Meta<typeof EventList>;
export default meta;
type Story = StoryObj<typeof meta>;

export const EventsOfTheBrokers: Story = {
  name: "Events of the brokers",
  args: {
    events: [
      event(
        "BackOff",
        "Back-off restarting failed container zeebe",
        { kind: "Pod", name: "orders-zeebe-2" },
        3,
        "Warning",
      ),
      event(
        "SuccessfulCreate",
        "create Pod orders-zeebe-2 in StatefulSet orders-zeebe successful",
        { kind: "StatefulSet", name: "orders-zeebe" },
        14,
      ),
      event(
        "CreatedStatefulSet",
        "Created StatefulSet 'orders-zeebe'",
        { kind: "CamundaCluster", name: "orders" },
        62,
      ),
    ],
  },
};
