// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { ComponentStats } from "./component-stats";

afterEach(cleanup);

const GiB = 2 ** 30;

function pod(name: string, ready: boolean, restarts: number[]) {
  return {
    metadata: { name },
    status: {
      phase: "Running",
      conditions: [{ type: "Ready", status: ready ? "True" : "False" }],
      containerStatuses: restarts.map((restartCount) => ({
        ready,
        restartCount,
      })),
    },
  };
}

const pods = [
  pod("orders-zeebe-0", true, [1, 2]),
  pod("orders-zeebe-1", true, [0]),
  pod("orders-zeebe-2", false, [4]),
];
const usage = [
  {
    pod: "orders-zeebe-0",
    cpu: { usedCores: 0.5, limitCores: 2 },
    memory: { usedBytes: 3 * GiB, limitBytes: 4 * GiB },
  },
  {
    pod: "orders-zeebe-1",
    cpu: { usedCores: 1.8, limitCores: 2 },
    memory: { usedBytes: 1 * GiB, limitBytes: 4 * GiB },
  },
];

// tile returns the text of the tile whose label is label.
const tile = (label: string) =>
  screen.getByText(label, { selector: "dt" }).parentElement!.textContent;

describe("ComponentStats", () => {
  it("counts the ready pods and says when not all are ready", () => {
    render(<ComponentStats pods={pods} usage={usage} />);
    expect(tile("Pods ready")).toBe("Pods ready2/3Not all pods are ready");
  });

  it("adds no note when every pod is ready", () => {
    render(<ComponentStats pods={pods.slice(0, 2)} usage={usage} />);
    expect(tile("Pods ready")).toBe("Pods ready2/2");
  });

  it("names the pod with the highest CPU and the pod with the highest memory against their limits", () => {
    render(<ComponentStats pods={pods} usage={usage} />);
    expect(tile("Highest CPU")).toBe(
      "Highest CPU90 %orders-zeebe-1, of its limit",
    );
    expect(tile("Highest memory")).toBe(
      "Highest memory75 %orders-zeebe-0, of its limit",
    );
  });

  it("says that the use is loading before the first answer", () => {
    render(<ComponentStats pods={pods} usage={undefined} />);
    expect(tile("Highest CPU")).toBe("Highest CPULoading");
  });

  it("says that the use is not available without a metrics server", () => {
    render(<ComponentStats pods={pods} usage="unavailable" />);
    expect(tile("Highest memory")).toBe("Highest memoryNot available");
  });

  it("counts the restarts of every container of every pod", () => {
    render(<ComponentStats pods={pods} usage={usage} />);
    expect(tile("Restarts")).toBe("Restarts7Since each pod started");
  });

  it("counts the restarts of an init container too", () => {
    const sidecar = {
      metadata: { name: "orders-zeebe-0" },
      status: {
        initContainerStatuses: [{ ready: true, restartCount: 5 }],
        containerStatuses: [{ ready: true, restartCount: 0 }],
      },
    };
    render(<ComponentStats pods={[sidecar]} usage={usage} />);
    expect(tile("Restarts")).toBe("Restarts5Since each pod started");
  });
});
