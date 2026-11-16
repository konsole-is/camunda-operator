// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import {
  coreSections,
  fixtureClaim,
  fixtureComponents,
  fixtureCondition,
  fixtureContract,
  fixtureGrant,
  fixtureRegistrations,
  fixtureRequest,
  fixtureRequests,
  fixtureSelections,
  requestNow,
} from "./request-fixtures";
import { RequestOverview } from "./request-detail";
import { requestDetail } from "./requests";

afterEach(cleanup);

type Over = {
  deletionPolicy?: "Delete" | "Orphan";
  request?: Resource<"PluginRequest">;
  grants?: Resource<"Grant">[];
  contracts?: Parameters<typeof requestDetail>[0]["contracts"];
  unread?: string[];
};

// show renders the Overview of the request of section of the fixture
// claim, read through requestDetail.
function show(section: string, over: Over = {}) {
  const claim = fixtureClaim(fixtureSelections, over.deletionPolicy);
  const requests = fixtureRequests.map((r) =>
    over.request && r.spec.section === over.request.spec.section
      ? over.request
      : r,
  );
  const components = fixtureComponents(claim, requests);
  const detail = requestDetail({
    component: components.find((c) => c.section === section)!,
    claim,
    cluster: "orders",
    sections: coreSections,
    registrations: fixtureRegistrations,
    components,
    grants: over.grants ?? [],
    contracts: over.contracts ?? {},
  })!;
  return render(
    <RequestOverview
      detail={detail}
      unread={over.unread ?? []}
      now={requestNow}
    />,
  );
}

const region = (name: string) => screen.getByRole("region", { name });
const cells = (row: HTMLElement) =>
  within(row)
    .getAllByRole("cell")
    .map((c) => c.textContent);

const rejectedRequest = fixtureRequest("backup-storage", {
  summary: [{ label: "Bucket", value: "orders-backups" }],
  conditions: [
    fixtureCondition("OutputsReady", "True", "Healthy"),
    fixtureCondition(
      "Ready",
      "False",
      "GrantRejected",
      "Grant orders-read has Ready=False",
    ),
  ],
});

describe("RequestOverview", () => {
  it("shows the summary items with their severity", () => {
    const request = fixtureRequest("encryption-key", {
      summary: [
        { label: "Key", value: "alias/orders", severity: "Info" },
        { label: "Rotation", value: "Off", severity: "Warning" },
        { label: "State", value: "Pending deletion", severity: "Error" },
      ],
    });
    show("encryption-key", { request });
    const summary = region("Summary");
    expect(within(summary).getByText("alias/orders").className).not.toMatch(
      /text-(warn|bad)/,
    );
    const warning = within(summary).getByText("Off");
    expect(warning.className).toContain("text-warn");
    expect(warning.textContent).toBe("Off");
    expect(warning.parentElement?.textContent).toContain("Warning:");
    const error = within(summary).getByText("Pending deletion");
    expect(error.className).toContain("text-bad");
    expect(error.parentElement?.textContent).toContain("Error:");
  });

  it("says when the plugin updated the summary", () => {
    show("encryption-key");
    expect(within(region("Summary")).getByText("Updated 20m ago")).toBeTruthy();
  });

  it("shows a summary item of the wrong type as the error of its row and keeps the others", () => {
    const request = fixtureRequest("encryption-key", {
      summary: [{ label: "Key", value: "alias/orders" }, "Rotation"] as never,
    });
    show("encryption-key", { request });
    const summary = region("Summary");
    expect(within(summary).getByText("alias/orders")).toBeTruthy();
    expect(
      within(summary).getByText("Summary item 2 is not a valid summary item."),
    ).toBeTruthy();
  });

  it("shows each resource with its fate and the policy", () => {
    show("backup-storage", { deletionPolicy: "Orphan" });
    const resources = region("Resources");
    expect(within(resources).getByText(/Keep the resources/)).toBeTruthy();
    expect(
      cells(within(resources).getByRole("row", { name: /^Bucket/ })),
    ).toEqual(["Bucket", "orders-backups", "-", "eu-west-1", "-", "Kept"]);
    expect(
      cells(
        within(resources).getByRole("row", { name: /^ObjectStorageConfig/ }),
      ).at(-1),
    ).toBe("Deleted");
  });

  it("shows no fate for the resources of a request that the claim does not control", () => {
    const request = fixtureRequest("backup-storage", {
      resources: [{ kind: "Bucket", name: "orders-backups", onOrphan: "Kept" }],
    });
    request.metadata!.ownerReferences = [];
    show("backup-storage", { request });
    const resources = region("Resources");
    expect(
      cells(within(resources).getByRole("row", { name: /^Bucket/ })).at(-1),
    ).toBe("Not controlled by this claim");
    expect(within(resources).queryByText("Kept")).toBeNull();
  });

  it("shows no fate while the claim is not read", () => {
    show("backup-storage", { unread: ["CamundaClusterClaim"] });
    const resources = region("Resources");
    expect(
      cells(within(resources).getByRole("row", { name: /^Bucket/ })).at(-1),
    ).toBe("Not known");
    expect(within(resources).queryByText("Kept")).toBeNull();
    expect(within(resources).queryByText("Deleted")).toBeNull();
    expect(within(resources).getByText(/CamundaClusterClaim/)).toBeTruthy();
  });

  it("shows the inputs with their section and the outputs with their field", () => {
    show("volume-encryption");
    expect(
      cells(
        within(region("Outputs")).getByRole("row", {
          name: /storageClassName/,
        }),
      ),
    ).toEqual([
      "storageClassName",
      "orders-encrypted",
      "CamundaCluster spec.zeebe.storageClassName",
    ]);
    expect(
      cells(within(region("Inputs")).getByRole("row", { name: /keyRef/ })),
    ).toEqual([
      "keyRef",
      "Encryption key / keyRef",
      "arn:aws:kms:eu-west-1:1:key/1234",
    ]);
  });

  it("says that an input is waiting until the claim controller resolves it", () => {
    show("volume-encryption", { request: fixtureRequest("volume-encryption") });
    expect(
      cells(within(region("Inputs")).getByRole("row", { name: /keyRef/ })).at(
        -1,
      ),
    ).toBe("Waiting");
  });

  it("shows a rejected grant with its reason", () => {
    const grant = fixtureGrant("orders-read", "backup-storage", [
      fixtureCondition("Applied", "False", "Rejected", "The policy is refused"),
      fixtureCondition("MatchesTemplate", "True", "Healthy"),
      fixtureCondition("Ready", "False", "Rejected", "The policy is refused"),
    ]);
    show("backup-storage", { request: rejectedRequest, grants: [grant] });
    const grants = region("Grants");
    expect(within(grants).getByText("orders-read")).toBeTruthy();
    expect(within(grants).getAllByText("Rejected").length).toBeGreaterThan(0);
    expect(within(grants).getByText(/^Refused/)).toBeTruthy();
    expect(
      within(grants).getByLabelText("Statement of orders-read").textContent,
    ).toContain('"s3:GetObject"');
  });

  it("shows Ready=False with the reason of the first failing condition of a grant, and that it is applied with the old template", () => {
    const grant = fixtureGrant("orders-read", "backup-storage", [
      fixtureCondition("Applied", "True", "Healthy"),
      fixtureCondition("MatchesTemplate", "False", "TemplateChanged"),
      fixtureCondition("Ready", "False", "TemplateChanged"),
    ]);
    show("backup-storage", { grants: [grant] });
    const grants = region("Grants");
    const list = within(grants).getByRole("list", {
      name: "Conditions of orders-read",
    });
    const ready = within(list).getAllByRole("listitem").at(-1)!;
    expect(ready.textContent).toContain("Ready");
    expect(ready.textContent).toContain("TemplateChanged");
    expect(
      within(grants).getByText(/^Applied with the old template/),
    ).toBeTruthy();
    expect(within(grants).queryByText(/Refused/)).toBeNull();
  });

  it("shows a grant without a status with its statement and no conditions", () => {
    show("backup-storage", {
      grants: [fixtureGrant("orders-read", "backup-storage")],
    });
    const grants = region("Grants");
    expect(
      within(grants).getByLabelText("Statement of orders-read"),
    ).toBeTruthy();
    expect(within(grants).getByText("No conditions")).toBeTruthy();
  });

  it("marks the conditions of a grant Not known while the grants are not read", () => {
    show("backup-storage", {
      grants: [
        fixtureGrant("orders-read", "backup-storage", [
          fixtureCondition("Ready", "True", "Healthy"),
        ]),
      ],
      unread: ["Grant"],
    });
    const grants = region("Grants");
    expect(within(grants).getByText("Ready: Not known")).toBeTruthy();
    expect(within(grants).queryByText("Healthy")).toBeNull();
  });

  it("names the Grant kind, never that there are no grants, while the grants of the cloud identity are not read", () => {
    show("cloud-identity", { unread: ["Grant"] });
    expect(
      within(region("Grants")).getByText(
        "Not known: the page could not read Grant.",
      ),
    ).toBeTruthy();
  });

  it("shows an existing contract as Connected without resources", () => {
    show("secondary-storage", {
      contracts: {
        SecondaryStorageConfig: [fixtureContract("orders-storage")],
      },
    });
    const row = within(region("Connected")).getByRole("row", {
      name: /SecondaryStorageConfig orders-storage/,
    });
    expect(cells(row)).toEqual([
      "SecondaryStorageConfig",
      "orders-storage",
      "Ready",
    ]);
    expect(screen.queryByRole("region", { name: "Resources" })).toBeNull();
  });

  it("says that a connected contract is not in the namespace", () => {
    show("secondary-storage");
    expect(
      cells(
        within(region("Connected")).getByRole("row", {
          name: /SecondaryStorageConfig/,
        }),
      ).at(-1),
    ).toBe("Not in the namespace");
  });

  it("marks the Ready of a connected contract Not known while its kind is not read", () => {
    show("secondary-storage", {
      contracts: {
        SecondaryStorageConfig: [fixtureContract("orders-storage")],
      },
      unread: ["SecondaryStorageConfig"],
    });
    const row = within(region("Connected")).getByRole("row", {
      name: /SecondaryStorageConfig/,
    });
    expect(cells(row).at(-1)).toBe("Ready: Not known");
  });

  it("shows the principal and the ServiceAccount of the Connectors permissions", () => {
    show("connectors-permissions");
    const principal = region("Principal");
    expect(
      within(principal).getByText("arn:aws:iam::1:role/orders-connectors"),
    ).toBeTruthy();
    expect(within(principal).getByText("orders-connectors")).toBeTruthy();
  });

  it("shows the sections that use the key", () => {
    show("encryption-key");
    expect(
      within(region("Used by"))
        .getAllByRole("listitem")
        .map((i) => i.textContent),
    ).toEqual(["Volume encryption", "Backup storage"]);
  });

  it("names the PluginRegistration kind in the inputs and users while it is not read", () => {
    show("encryption-key", { unread: ["PluginRegistration"] });
    expect(
      within(region("Used by")).getByText(
        "Not known: the page could not read PluginRegistration.",
      ),
    ).toBeTruthy();
  });

  it("links each link of the request with a hit area", () => {
    show("access");
    const link = within(region("Links")).getByRole("link", { name: "Operate" });
    expect(link.getAttribute("href")).toBe(
      "https://orders.example.com/operate",
    );
    expect(link.className).toContain("hit-area");
  });

  it.each([
    ["cloud-identity", "orders-camunda", "Grants"],
    ["encryption-key", "alias/orders", "Used by"],
    ["volume-encryption", "orders-encrypted", "Inputs"],
    ["backup-storage", "orders-backups", "Resources"],
    ["secondary-storage", "orders-storage, Ready", "Connected"],
    ["access", "orders.example.com", "Links"],
    ["connectors-permissions", "orders-connectors", "Principal"],
  ])(
    "shows the summary headline and the block of %s",
    (section, headline, block) => {
      show(section, {
        grants: [fixtureGrant("orders-read", "backup-storage")],
      });
      expect(
        within(region("Summary")).getAllByText(headline).length,
      ).toBeGreaterThan(0);
      expect(region(block)).toBeTruthy();
    },
  );
});
