import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { parseLogLine } from "@/lib/observe/logs";
import type { LogLine } from "@/lib/observe/types";
import { DetailLogsView } from "./detail-logs";
import { brokers } from "./fixtures";

// logLine gives one JSON line of a broker, as Camunda writes it, from pod.
function logLine(
  pod: string,
  second: number,
  level: string,
  logger: string,
  message: string,
): LogLine {
  const timestamp = `2026-10-30T11:58:${String(second).padStart(2, "0")}.120Z`;
  return {
    ...parseLogLine(
      JSON.stringify({
        timestamp,
        severity: level,
        logger_name: logger,
        message,
      }),
    ),
    pod,
  };
}

const meta = {
  title: "cluster/DetailLogs",
  component: DetailLogsView,
  args: { picked: "" },
} satisfies Meta<typeof DetailLogsView>;
export default meta;
type Story = StoryObj<typeof meta>;

export const AllPodsOfTheBrokers: Story = {
  name: "All pods of the brokers",
  args: {
    pods: brokers.map((p) => p.metadata.name),
    lines: [
      logLine(
        "orders-zeebe-0",
        2,
        "INFO",
        "io.camunda.zeebe.broker.system",
        "Broker 0 is ready",
      ),
      logLine(
        "orders-zeebe-1",
        3,
        "INFO",
        "io.camunda.zeebe.broker.system",
        "Broker 1 is ready",
      ),
      logLine(
        "orders-zeebe-0",
        9,
        "INFO",
        "io.camunda.zeebe.raft",
        "Partition 1 transitioned to LEADER in term 4",
      ),
      logLine(
        "orders-zeebe-2",
        11,
        "WARN",
        "io.camunda.zeebe.broker.system",
        "Disk usage is above 80 percent of the data volume",
      ),
      logLine(
        "orders-zeebe-2",
        14,
        "ERROR",
        "io.camunda.zeebe.journal",
        "Could not append to the journal of partition 3, because the disk is full",
      ),
      logLine(
        "orders-zeebe-1",
        20,
        "INFO",
        "io.camunda.zeebe.raft",
        "Partition 2 transitioned to LEADER in term 7",
      ),
    ],
    errors: [],
  },
};

export const ARequestWithItsController: Story = {
  name: "A request with its controller",
  args: {
    pods: ["rustfs-0"],
    controllerPods: ["rustfs-controller-6f9c-2xk4p"],
    lines: [
      logLine(
        "rustfs-0",
        1,
        "INFO",
        "rustfs::server",
        "Bucket orders-backup-storage is ready",
      ),
      {
        ...parseLogLine(
          JSON.stringify({
            ts: "2026-10-30T11:58:04.000Z",
            level: "info",
            logger: "plugin",
            msg: "Reconciled the bucket",
            request: { namespace: "team-a", name: "orders-backup-storage" },
          }),
        ),
        pod: "rustfs-controller-6f9c-2xk4p",
      },
    ],
    errors: [
      { pod: "rustfs-0", error: "The connection to the log stream broke" },
    ],
  },
};
