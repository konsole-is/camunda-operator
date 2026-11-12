// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import type { Warning } from "@/lib/warnings";
import { MissingSources, WarningChips } from "./warning-chips";

afterEach(cleanup);

// jsdom has no ResizeObserver, and the Radix tooltip measures its trigger.
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

const NOW = "2026-11-10T12:00:00Z";

const restarts: Warning = {
  kind: "restarts",
  severity: "Warning",
  label: "Restarts",
  detail: "Container zeebe of pod orders-zeebe-1 restarted 5 times",
  since: "2026-11-10T11:55:00Z",
};
const oom: Warning = {
  kind: "oom",
  severity: "Error",
  label: "Out of memory",
  detail: "Container zeebe of pod orders-zeebe-0 ran out of memory",
  since: "2026-11-10T11:48:00Z",
};
const backup: Warning = {
  kind: "backup",
  severity: "Warning",
  label: "Backup failed",
  detail: "The newest backup orders-nightly failed",
};

describe("WarningChips", () => {
  it("shows an Error before a Warning", () => {
    render(<WarningChips warnings={[restarts, oom, backup]} now={NOW} />);
    expect(screen.getAllByRole("button").map((b) => b.textContent)).toEqual([
      "Error: Out of memory",
      "Warning: Restarts",
      "Warning: Backup failed",
    ]);
  });

  it("names the severity in the accessible name of a chip", () => {
    render(
      <WarningChips
        warnings={[
          { ...oom, kind: "memory", label: "Near memory limit" },
          {
            ...backup,
            kind: "memory",
            label: "Near memory limit",
            detail: "Pod orders-zeebe-1 uses 80 % of its memory limit",
          },
        ]}
        now={NOW}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Error: Near memory limit" }),
    ).toBeTruthy();
    expect(
      screen.getByRole("button", { name: "Warning: Near memory limit" }),
    ).toBeTruthy();
  });

  it("shows the detail and the age in the tooltip", async () => {
    render(<WarningChips warnings={[oom]} now={NOW} />);
    await userEvent.tab();
    expect(document.activeElement?.textContent).toBe("Error: Out of memory");
    expect((await screen.findByRole("tooltip")).textContent).toBe(
      "Container zeebe of pod orders-zeebe-0 ran out of memory, 12m ago.",
    );
  });

  it("shows the detail alone for a warning without a time", async () => {
    render(<WarningChips warnings={[backup]} now={NOW} />);
    await userEvent.tab();
    expect((await screen.findByRole("tooltip")).textContent).toBe(
      "The newest backup orders-nightly failed.",
    );
  });

  it("shows a dash without warnings", () => {
    const { container } = render(<WarningChips warnings={[]} now={NOW} />);
    expect(container.textContent).toBe("-");
  });
});

describe("MissingSources", () => {
  it("says once that metrics-server is missing", () => {
    render(
      <MissingSources pods={{ clusters: {}, missing: ["metrics-server"] }} />,
    );
    expect(
      screen.getAllByText(
        "metrics-server is not found, so the near memory and CPU limit warnings do not show.",
        { exact: false },
      ),
    ).toHaveLength(1);
    expect(screen.getByRole("link").getAttribute("href")).toBe(
      "/setup/settings",
    );
  });

  it("says that Prometheus is missing", () => {
    render(<MissingSources pods={{ clusters: {}, missing: ["prometheus"] }} />);
    expect(
      screen.getByText(
        "Prometheus is not set or does not answer, so the backpressure warning does not show.",
        { exact: false },
      ),
    ).toBeTruthy();
    expect(screen.getByRole("link").getAttribute("href")).toBe(
      "/setup/settings",
    );
  });

  it("says that the warnings of the pods do not show while the route is unavailable", () => {
    const { container } = render(<MissingSources pods="unavailable" />);
    expect(container.textContent).toBe(
      "The warnings of the pods do not show, because the fleet manager could not read them.",
    );
  });

  it("says nothing when no source is missing", () => {
    const { container } = render(
      <MissingSources pods={{ clusters: {}, missing: [] }} />,
    );
    expect(container.textContent).toBe("");
  });

  it("says nothing before the first answer", () => {
    const { container } = render(<MissingSources pods={undefined} />);
    expect(container.textContent).toBe("");
  });
});
