// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import { clusterFields } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import { ComponentSettings } from "./component-settings";
import { componentSettings } from "./settings";

afterEach(cleanup);

const CLAIM = "CamundaClusterClaim/cluster/0b5c7a2e";

function fieldsOf(spec: CamundaClusterSpec) {
  return clusterFields({
    cluster: {
      kind: "CamundaCluster",
      metadata: {
        name: "orders",
        namespace: "team-a",
        labels: { [CLAIM_LABEL]: "orders" },
        managedFields: [
          {
            manager: CLAIM,
            time: "2026-10-01T00:00:00Z",
            fieldsV1: { "f:spec": { "f:zeebe": { "f:storageClassName": {} } } },
          },
          {
            manager: "pvc-autoresizer",
            time: "2026-10-02T00:00:00Z",
            fieldsV1: { "f:spec": { "f:zeebe": { "f:storageSize": {} } } },
          },
        ],
      },
      spec,
    } as unknown as Resource<"CamundaCluster">,
    merged: mergeCluster(undefined, undefined, spec),
    claim: {
      kind: "CamundaClusterClaim",
      metadata: { name: "orders", namespace: "team-a" },
      spec: {
        presetRef: "medium",
        plugins: [
          { section: "volume-encryption", registration: "aws-ebs-kms" },
        ],
      },
    } as unknown as Resource<"CamundaClusterClaim">,
    sections: [
      {
        kind: "PluginSection",
        metadata: { name: "volume-encryption" },
        spec: {
          label: "Volume encryption",
          slot: "volumeEncryption",
          appliesTo: ["CamundaClusterClaim"],
          selection: "One",
        },
      } as unknown as Resource<"PluginSection">,
    ],
    registrations: [],
  });
}

const brokers: CamundaClusterSpec = {
  zeebe: { storageClassName: "encrypted", storageSize: "64Gi" },
};

function renderSettings(spec: CamundaClusterSpec, docs = {}) {
  render(
    <ComponentSettings
      groups={componentSettings(fieldsOf(spec), "zeebe", spec)}
      docs={docs}
      operatorVersion="0.2.0"
      cluster={{ namespace: "team-a", name: "orders" }}
    />,
  );
}

function row(label: string): HTMLElement {
  const header = screen
    .getAllByRole("rowheader")
    .find((h) => h.querySelector("code")?.textContent === label);
  expect(header, `a row ${label}`).toBeDefined();
  return header!.closest("tr")!;
}

describe("ComponentSettings", () => {
  it("the Brokers settings show a storage class Filled by Volume encryption and a volume size Set by its resizer", () => {
    renderSettings(brokers);
    const storageClass = within(row("storageClassName"));
    expect(storageClass.getByText("encrypted")).toBeDefined();
    expect(
      storageClass.getAllByText("Filled by Volume encryption"),
    ).toHaveLength(2);
    const size = within(row("storageSize"));
    expect(size.getByText("64Gi")).toBeDefined();
    expect(size.getByText("Set by pvc-autoresizer")).toBeDefined();
    expect(
      size.getByText("Setting it in the claim takes it over."),
    ).toBeDefined();
  });

  it("a field with a claim path links the Claim tab", () => {
    renderSettings(brokers);
    expect(
      within(row("storageSize"))
        .getByRole("link", { name: "Open in the Claim tab" })
        .getAttribute("href"),
    ).toBe(
      "/fleet/clusters/team-a/orders?tab=claim&field=spec.cluster.zeebe.storageSize",
    );
  });

  it("shows the fields of an embedded app under its own heading", () => {
    renderSettings({
      ...brokers,
      gateway: { mode: "Embedded" },
      operate: { replicas: 2 },
    });
    expect(
      screen.getByRole("heading", { name: "Operate (embedded)" }),
    ).toBeDefined();
    const operate = screen.getByRole("table", { name: "Operate settings" });
    expect(within(operate).getByText("replicas")).toBeDefined();
  });

  it("a field without a value says Not set", () => {
    renderSettings({ zeebe: { storageSize: "64Gi" } });
    expect(within(row("storageClassName")).getByText("Not set")).toBeDefined();
  });

  it("renders no input", () => {
    renderSettings(brokers);
    for (const role of ["textbox", "combobox", "checkbox"])
      expect(screen.queryAllByRole(role)).toEqual([]);
  });

  it("shows the help of a field that has a doc", () => {
    renderSettings(brokers, {
      "CamundaCluster:spec.zeebe.storageSize": {
        description: "The size of the volume of each broker.",
        anchor: "#zeebespec",
      },
    });
    expect(
      screen.getByRole("button", { name: "About storageSize" }),
    ).toBeDefined();
  });

  it("says No settings when no group has a field", () => {
    render(
      <ComponentSettings
        groups={[{ label: "Connectors", embedded: false, fields: [] }]}
        docs={{}}
        operatorVersion="0.2.0"
        cluster={{ namespace: "team-a", name: "orders" }}
      />,
    );
    expect(screen.getByText("No settings.")).toBeDefined();
  });
});
