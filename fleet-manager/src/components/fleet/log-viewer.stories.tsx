import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { LogLine } from "@/lib/observe/types";
import { LogViewer } from "./log-viewer";

const lines: LogLine[] = [
  {
    raw: "",
    time: "2026-10-15T11:58:01Z",
    level: "INFO",
    message: "Partition 1 became leader",
    logger: "io.camunda.zeebe.broker",
    fields: { partitionId: 1, term: 4 },
  },
  {
    raw: "",
    time: "2026-10-15T11:58:05Z",
    level: "WARN",
    message: "Exporter is behind by 1200 records",
    fields: {},
  },
  {
    raw: "",
    time: "2026-10-15T11:58:09Z",
    level: "ERROR",
    message: "Failed to export a batch to Elasticsearch: connection refused",
    fields: { exporter: "camundaexporter" },
  },
].map((l) => ({ ...l, raw: `${l.level} ${l.message}` }));

const meta = {
  title: "fleet/LogViewer",
  component: LogViewer,
  args: { lines: [] },
} satisfies Meta<typeof LogViewer>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Following: Story = { name: "Following", args: { lines } };

export const NoLines: Story = { name: "No lines" };
