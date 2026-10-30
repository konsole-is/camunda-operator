// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { ClusterField } from "@/lib/fields";
import { child } from "@/lib/merge";
import { FieldState, FieldValue } from "./field-state";

afterEach(cleanup);

function field(over: Partial<ClusterField>): ClusterField {
  return {
    key: "CamundaCluster:spec.zeebe.storageSize",
    target: { kind: "CamundaCluster" },
    group: "zeebe",
    path: child(child("spec", "zeebe"), "storageSize"),
    label: "storageSize",
    value: "64Gi",
    state: { kind: "default" },
    ...over,
  };
}

describe("FieldValue", () => {
  it("says Not set for a field without a value", () => {
    render(<FieldValue value={undefined} />);
    expect(screen.getByText("Not set")).toBeTruthy();
  });

  it.each([
    ["64Gi", "64Gi"],
    [3, "3"],
    [false, "false"],
  ])("shows %j as the text %s", (value, text) => {
    render(<FieldValue value={value} />);
    expect(document.body.textContent).toBe(text);
  });

  it("shows an object as JSON", () => {
    render(<FieldValue value={{ cpu: "2" }} />);
    expect(screen.getByText('{"cpu":"2"}')).toBeTruthy();
  });
});

describe("FieldState", () => {
  it("shows the state, its note, and the lock", () => {
    render(
      <FieldState
        field={field({
          state: { kind: "setBy", manager: "pvc-autoresizer", takeOver: true },
          lock: { reason: "release" },
        })}
      />,
    );
    expect(screen.getByText("Set by pvc-autoresizer")).toBeTruthy();
    expect(
      screen.getByText("Setting it in the claim takes it over."),
    ).toBeTruthy();
    expect(screen.getByText("Set by the release")).toBeTruthy();
  });

  it("names the section of a filled field in the state and the lock", () => {
    render(
      <FieldState
        field={field({
          state: {
            kind: "filled",
            section: "volume-encryption",
            sectionLabel: "Volume encryption",
          },
          lock: {
            reason: "filled",
            section: "volume-encryption",
            sectionLabel: "Volume encryption",
          },
        })}
      />,
    );
    expect(screen.getAllByText("Filled by Volume encryption")).toHaveLength(2);
  });

  it("links the field in the Claim tab when given a link", () => {
    render(
      <FieldState
        field={field({ state: { kind: "claim" } })}
        claimHref="/fleet/clusters/team-a/orders?tab=claim&field=x"
      />,
    );
    expect(
      screen
        .getByRole("link", { name: "Open in the Claim tab" })
        .getAttribute("href"),
    ).toBe("/fleet/clusters/team-a/orders?tab=claim&field=x");
  });

  it("has no link without one", () => {
    render(<FieldState field={field({ state: { kind: "claim" } })} />);
    expect(screen.queryByRole("link")).toBeNull();
  });
});
