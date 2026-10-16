// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { ProgressRows } from "./progress-rows";

afterEach(cleanup);

describe("ProgressRows", () => {
  it("shows the Ready message on top and the state of each check of each row", () => {
    render(
      <ProgressRows
        ready={{
          status: "False",
          reason: "WaitingForOutputs",
          message: "Waiting for Backup storage",
        }}
        rows={[
          {
            id: "backup",
            label: "Backup storage",
            detail: "New S3 bucket",
            checks: [
              {
                label: "Outputs",
                condition: { status: "False", reason: "Provisioning" },
              },
              {
                label: "Ready",
                condition: { status: "False", reason: "Provisioning" },
              },
            ],
          },
          {
            id: "cluster",
            label: "Cluster",
            checks: [{ label: "ClusterReady" }],
          },
        ]}
      />,
    );
    expect(screen.getByText("Waiting for Backup storage")).toBeTruthy();
    expect(
      screen.getByRole("img", { name: "Outputs: Not ready (Provisioning)" }),
    ).toBeTruthy();
    expect(
      screen.getByRole("img", { name: "ClusterReady: No status" }),
    ).toBeTruthy();
  });
});
