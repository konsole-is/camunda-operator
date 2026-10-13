import type { Story } from "@/gallery/registry";
import { StatusChip } from "./status-chip";
import { StatusDot } from "./status-dot";

export const statusStories: Story[] = [
  {
    component: "StatusChip",
    name: "Ready",
    render: () => (
      <StatusChip condition={{ status: "True", reason: "Healthy" }} />
    ),
  },
  {
    component: "StatusChip",
    name: "Not ready",
    render: () => (
      <StatusChip
        condition={{
          status: "False",
          reason: "ZeebeNotReady",
          message: "1 of 3 Zeebe brokers are ready",
        }}
      />
    ),
  },
  {
    component: "StatusChip",
    name: "Not ready without reason",
    render: () => <StatusChip condition={{ status: "False" }} />,
  },
  {
    component: "StatusChip",
    name: "Unknown",
    render: () => (
      <StatusChip condition={{ status: "Unknown", reason: "Reconciling" }} />
    ),
  },
  { component: "StatusChip", name: "No status", render: () => <StatusChip /> },
  {
    component: "StatusChip",
    name: "Loading",
    render: () => <StatusChip loading />,
  },
  {
    component: "StatusDot",
    name: "All states",
    render: () => (
      <div className="flex items-center gap-3">
        <StatusDot label="Zeebe" condition={{ status: "True" }} />
        <StatusDot
          label="Operate"
          condition={{ status: "False", reason: "PodsNotReady" }}
        />
        <StatusDot label="Tasklist" condition={{ status: "Unknown" }} />
        <StatusDot label="Optimize" />
        <StatusDot label="Backups" off />
      </div>
    ),
  },
];
