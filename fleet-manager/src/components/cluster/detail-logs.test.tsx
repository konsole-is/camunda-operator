// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { parseLogLine } from "@/lib/observe/logs";
import { DetailLogs, LOG_TAIL_LINES, MAX_LOG_STREAMS } from "./detail-logs";

const live = vi.hoisted(() => ({
  sources: [] as unknown[][],
  streams: { lines: [] as unknown[], errors: [] as unknown[], ended: false },
  controller: undefined as unknown,
  registrations: [] as unknown[],
}));
vi.mock("@/lib/observe/use-log-stream", () => ({
  useLogStreams: (sources: unknown[]) => {
    live.sources.push(sources);
    return live.streams;
  },
}));
vi.mock("@/lib/observe/use-observe", () => ({
  useControllerPods: (registration: unknown) => {
    live.registrations.push(registration);
    return live.controller;
  },
}));

beforeEach(() => {
  live.sources = [];
  live.streams = { lines: [], errors: [], ended: false };
  live.controller = undefined;
  live.registrations = [];
});
afterEach(cleanup);

const pod = (name: string) => ({ metadata: { name } });
const brokers = ["orders-zeebe-0", "orders-zeebe-1", "orders-zeebe-2"].map(pod);
const lastSources = () => live.sources.at(-1)!;
const controller = {
  registration: "rustfs-bucket",
  request: { namespace: "team-a", name: "orders-backup-storage" },
};

describe("DetailLogs", () => {
  it("follows every pod of the component in All pods", () => {
    render(<DetailLogs namespace="team-a" pods={brokers} />);

    expect(lastSources()).toEqual(
      brokers.map((p) => ({
        namespace: "team-a",
        pod: p.metadata.name,
        tailLines: LOG_TAIL_LINES,
      })),
    );
    expect(
      screen.getByRole("option", { name: "All pods (3)" }),
    ).toBeTruthy();
    expect(live.registrations.every((r) => r === undefined)).toBe(true);
  });

  it("follows one pod after the user picks it", async () => {
    render(<DetailLogs namespace="team-a" pods={brokers} />);

    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Pod" }),
      "orders-zeebe-1",
    );

    expect(lastSources()).toEqual([
      { namespace: "team-a", pod: "orders-zeebe-1", tailLines: LOG_TAIL_LINES },
    ]);
  });

  it("follows at most 10 pods and says how many it leaves out", () => {
    const many = Array.from({ length: 14 }, (_, i) => pod(`orders-zeebe-${i}`));

    render(<DetailLogs namespace="team-a" pods={many} />);

    expect(lastSources()).toHaveLength(MAX_LOG_STREAMS);
    expect(
      screen.getByText(
        "Showing the logs of 10 of 14 pods. Pick a pod to see the others.",
      ),
    ).toBeTruthy();
  });

  it("follows the controller pods with the lines of the request only", () => {
    live.controller = { namespace: "plugins", pods: ["rustfs-controller-0"] };

    render(
      <DetailLogs
        namespace="team-a"
        pods={[pod("rustfs-0")]}
        controller={controller}
      />,
    );

    expect(live.registrations).toContain("rustfs-bucket");
    expect(lastSources()).toEqual([
      { namespace: "team-a", pod: "rustfs-0", tailLines: LOG_TAIL_LINES },
      {
        namespace: "plugins",
        pod: "rustfs-controller-0",
        tailLines: LOG_TAIL_LINES,
        request: "team-a/orders-backup-storage",
      },
    ]);
    const group = screen.getByRole("group", { name: "Controller" });
    expect(
      within(group).getByRole("option", { name: "rustfs-controller-0" }),
    ).toBeTruthy();
  });

  it("follows one controller pod after the user picks it", async () => {
    live.controller = { namespace: "plugins", pods: ["rustfs-controller-0"] };
    render(
      <DetailLogs
        namespace="team-a"
        pods={[pod("rustfs-0")]}
        controller={controller}
      />,
    );

    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Pod" }),
      within(screen.getByRole("group", { name: "Controller" })).getByRole(
        "option",
        { name: "rustfs-controller-0" },
      ),
    );

    expect(lastSources()).toEqual([
      {
        namespace: "plugins",
        pod: "rustfs-controller-0",
        tailLines: LOG_TAIL_LINES,
        request: "team-a/orders-backup-storage",
      },
    ]);
  });

  it("says so when it cannot read the controller pods", () => {
    live.controller = "unavailable";

    render(
      <DetailLogs
        namespace="team-a"
        pods={[pod("rustfs-0")]}
        controller={controller}
      />,
    );

    expect(screen.getByText("Could not read the controller pods.")).toBeTruthy();
    expect(lastSources()).toHaveLength(1);
  });

  it("names a pod whose stream broke", () => {
    live.streams = {
      lines: [],
      errors: [{ pod: "orders-zeebe-2", error: "The pod has no log yet" }],
      ended: false,
    };

    render(<DetailLogs namespace="team-a" pods={brokers} />);

    expect(
      screen.getByText("orders-zeebe-2: The pod has no log yet"),
    ).toBeTruthy();
  });

  it("shows the lines of the streams", () => {
    live.streams = {
      lines: [{ ...parseLogLine("Broker 0 is ready"), pod: "orders-zeebe-0" }],
      errors: [],
      ended: false,
    };

    render(<DetailLogs namespace="team-a" pods={brokers} />);

    expect(
      within(screen.getByRole("log")).getByText("Broker 0 is ready"),
    ).toBeTruthy();
  });

  it("says so when the component has no pods", () => {
    render(<DetailLogs namespace="team-a" pods={[]} />);

    expect(
      screen.getByText("This component has no pods, so it has no logs."),
    ).toBeTruthy();
    expect(screen.queryByRole("log")).toBeNull();
    expect(lastSources()).toEqual([]);
  });
});
